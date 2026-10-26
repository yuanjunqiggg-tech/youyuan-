#!/usr/bin/env python3
"""冷启动实测：重启 youyuan.service，测端口可用时间与首次请求耗时，并确认机器人恢复。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22)); t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=180):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.02)
    try: ch.close()
    except Exception: pass
    return buf.decode("utf-8", errors="replace")

print("== 重启前 ==")
print(run("curl -s -m 3 http://127.0.0.1:19429/api/bot/status; echo; date +%s.%N", 15).strip())

print("\n== 重启 + 计时到端口可连 ==")
print(run("""
date +%s.%N > /tmp/t0
systemctl restart youyuan.service
for i in $(seq 400); do
  code=$(curl -s -o /dev/null -m 1 -w '%{http_code}' --connect-timeout 1 http://127.0.0.1:19429/health 2>/dev/null)
  if [ "$code" = "200" ]; then date +%s.%N > /tmp/t1; echo "port_ready_after=$(python3 -c \"print(round(float(open('/tmp/t1').read())-float(open('/tmp/t0').read()),2))\")s"; break; fi
  sleep 0.05
done
""", 180).strip())

print("\n== 冷启动后连续 5 次 plugin/list ==")
print(run("for i in 1 2 3 4 5; do curl -s -o /tmp/pl$i.json -w \"try$i total=%{time_total}s ttfb=%{time_starttransfer}s size=%{size_download} code=%{http_code}\\n\" http://127.0.0.1:19429/api/plugin/list; done", 90).strip())

print("\n== 冷启动后对照接口 ==")
for p in ["/api/bot/status", "/api/task/list", "/api/youyuan/chains"]:
    print(p)
    print(run("for i in 1 2 3; do curl -s -o /dev/null -w '  total=%{time_total}s code=%{http_code}\\n' http://127.0.0.1:19429" + p + "; done", 60).strip())

print("\n== 启动日志时间线 ==")
print(run("journalctl -u youyuan.service --no-pager -o short-iso -n 12 2>&1", 40).strip())

print("\n== 等 15 秒后机器人状态 ==")
time.sleep(15)
print(run("curl -s -m 5 http://127.0.0.1:19429/api/bot/status; echo; systemctl is-active youyuan.service", 20).strip())

print("\n== 首次启动 CPU 峰值 ==")
print(run("systemd-cgtop -P -n 1 --order=cpu 2>/dev/null | head -6 || true", 20).strip())
t.close()
