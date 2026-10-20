#!/usr/bin/env python3
"""SSH 恢复守护：每 90 秒探测一次生产机，SSH 一恢复立即：
1) 诊断（服务状态/二进制类型/磁盘/内存/OOM 日志）
2) 修复：youyuan.service 恢复 + 拆自动更新定时炸弹
3) 如 youyuan.new 是完整新二进制则顺手完成部署
全程日志到 maintenance/server-recovery.log
"""
import hashlib, json, sys, time, datetime

import paramiko

CFG = r"D:/YouYuan-APK/private/linux-access.json"
LOG = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/maintenance/server-recovery.log"
NEW_SHA = "6daf66ab00b2c4dff2a982230deaa36bea39b11b6225e4938cb6ae5d470c3455"

def log(msg):
    line = f"[{datetime.datetime.now().strftime('%m-%d %H:%M:%S')}] {msg}"
    print(line, flush=True)
    with open(LOG, "a", encoding="utf-8") as f:
        f.write(line + "\n")

def connect():
    cfg = json.load(open(CFG))
    pin = cfg["host_key_sha256"].replace(":", "").lower()
    t = paramiko.Transport((cfg["host"], 22))
    t.start_client(timeout=60)
    hk = t.get_remote_server_key()
    if hashlib.sha256(hk.asbytes()).hexdigest() != pin:
        raise RuntimeError("host key mismatch")
    t.auth_password(cfg["username"], cfg["password"])
    return t

def run(t, cmd, timeout=60):
    ch = t.open_session(timeout=20)
    ch.settimeout(timeout)
    ch.exec_command(cmd)
    buf = b""
    dl = time.time() + timeout
    while time.time() < dl:
        if ch.recv_ready():
            buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready():
            break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

DIAG = [
    ("uptime", "uptime"),
    ("mem", "free -m"),
    ("disk", "df -h / /opt 2>&1 | head -5"),
    ("svc", "systemctl is-active youyuan.service; systemctl show youyuan.service -p ActiveEnterTimestamp -p ExecMainStartTimestamp -p NRestarts 2>&1 | head -4"),
    ("bin", "file /opt/youyuan/youyuan 2>&1; ls -l --time-style=full-iso /opt/youyuan/ 2>&1"),
    ("newbin", "test -f /opt/youyuan/youyuan.new && sha256sum /opt/youyuan/youyuan.new 2>&1 || echo NO_NEW"),
    ("state", "cat /var/lib/youyuan-update/installed-code 2>&1"),
    ("port", "ss -ltn | grep -E '19429|22 ' || true"),
    ("oom", "dmesg -T 2>/dev/null | grep -iE 'oom|out of memory' | tail -5 || journalctl -k --since '-3 hours' 2>/dev/null | grep -iE 'oom|out of memory' | tail -5 || echo NO_OOM_LOG"),
    ("svclog", "journalctl -u youyuan.service --since '-3 hours' --no-pager 2>/dev/null | tail -25"),
    ("timer", "systemctl is-enabled youyuan-update.timer 2>&1; systemctl list-timers youyuan-update.timer --no-pager 2>&1 | head -3"),
    ("sshd", "systemctl is-active sshd ssh 2>&1 | head -2"),
]

REPAIR_STEPS = [
    # 拆定时炸弹：把 installed-code 顶到 1077，阻止 21:39/03:39 定时器用 APK 砖掉服务
    ("defuse-timer", "mkdir -p /var/lib/youyuan-update && echo 1077 > /var/lib/youyuan-update/installed-code && systemctl stop youyuan-update.timer 2>/dev/null; echo TIMER_DEFUSED"),
    # 若 /opt/youyuan/youyuan 不是 ELF（被 APK 覆盖），且 youyuan.new 是完整新二进制 → 换回
    ("check-and-swap", """
import hashlib,os,sys
target='/opt/youyuan/youyuan'; newf='/opt/youyuan/youyuan.new'
def sha(p):
    h=hashlib.sha256()
    with open(p,'rb') as f:
        for b in iter(lambda:f.read(1<<22),b''): h.update(b)
    return h.hexdigest()
def is_elf(p):
    try:
        with open(p,'rb') as f: return f.read(4)==b'\\x7fELF'
    except: return False
print('target_elf=',is_elf(target))
if not is_elf(target):
    if os.path.exists(newf) and sha(newf)=='6daf66ab00b2c4dff2a982230deaa36bea39b11b6225e4938cb6ae5d470c3455':
        os.rename(target,target+'.broken-'+str(int(__import__('time').time())))
        os.rename(newf,target); os.chmod(target,0o755)
        print('SWAPPED_FROM_NEW')
    else:
        print('NEW_INVALID_NEED_UPLOAD')
else:
    print('TARGET_OK')
"""),
]

PY_SWAP = REPAIR_STEPS[1][1]

def main():
    log("=== 恢复守护启动，每 90s 探测 ===")
    for attempt in range(1, 61):
        try:
            t = connect()
            log(f"SSH 恢复（第 {attempt} 次探测成功）")
            for name, cmd in DIAG:
                out = run(t, cmd, 45)
                log(f"--- {name} ---\n{out.strip()}")
            # 拆炸弹
            out = run(t, REPAIR_STEPS[0][1], 45)
            log(f"--- defuse-timer ---\n{out.strip()}")
            # 检查/换回二进制
            out = run(t, "python3 - <<'PYEOF'\n" + PY_SWAP + "\nPYEOF", 90)
            log(f"--- check-and-swap ---\n{out.strip()}")
            if "SWAPPED_FROM_NEW" in out or "TARGET_OK" in out:
                out = run(t, "systemctl restart youyuan.service && sleep 3 && systemctl is-active youyuan.service && ss -ltn | grep 19429 || echo START_FAIL", 60)
                log(f"--- restart ---\n{out.strip()}")
                out = run(t, "curl -s -m 5 http://127.0.0.1:19429/health || echo HEALTH_FAIL", 30)
                log(f"--- health ---\n{out.strip()}")
            t.close()
            log("=== 守护完成一轮，退出（后续按需人工）===")
            return 0
        except Exception as e:
            log(f"attempt {attempt}: {type(e).__name__}: {e}")
        time.sleep(90)
    log("=== 60 次探测失败，守护退出 ===")
    return 1

if __name__ == "__main__":
    sys.exit(main())
