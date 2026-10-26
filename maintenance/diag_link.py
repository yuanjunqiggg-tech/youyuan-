#!/usr/bin/env python3
"""诊断连接链路：cloudflared 隧道 / 中继配置 / 更新炸弹 / youyuan.new 完整性"""
import hashlib, json, sys, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
if hashlib.sha256(hk.asbytes()).hexdigest() != pin:
    print("PIN_MISMATCH"); sys.exit(3)
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=40):
    ch = t.open_session(timeout=20)
    ch.settimeout(to)
    ch.exec_command(cmd)
    dl = time.time() + to
    buf = b""
    while time.time() < dl:
        if ch.recv_ready():
            buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready():
            break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

CMDS = [
    ("cloudflared", "systemctl is-active cloudflared 2>&1; systemctl list-units --all | grep -i 'cloudflared\\|tunnel' | head -5; ps aux | grep -i cloudflared | grep -v grep | head -3"),
    ("timer拆弹", "mkdir -p /var/lib/youyuan-update && echo 1077 > /var/lib/youyuan-update/installed-code 2>/dev/null; systemctl stop youyuan-update.timer 2>/dev/null; systemctl disable youyuan-update.timer 2>&1; echo TIMER_DEFUSED"),
    ("new完整性", "test -f /opt/youyuan/youyuan.new && sha256sum /opt/youyuan/youyuan.new || echo NO_NEW"),
    ("监听与防火墙", "ss -ltnp | grep -E '19429|cloudflared' | head -5; iptables -L INPUT -n 2>/dev/null | head -8 || echo no-iptables"),
    ("服务配置", "systemctl cat youyuan.service 2>/dev/null | head -25"),
]
for name, cmd in CMDS:
    print(f"===== {name} =====")
    print(run(cmd))
t.close()
