#!/usr/bin/env python3
"""只读探查生产机状态：服务是否在跑、二进制 mtime、端口、机器人状态。"""
import hashlib, json, sys, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=90)
hk = t.get_remote_server_key()
if hashlib.sha256(hk.asbytes()).hexdigest() != pin:
    print("PIN_MISMATCH"); sys.exit(3)
t.auth_password(cfg["username"], cfg["password"])

def run(cmd):
    ch = t.open_session(timeout=15)
    ch.exec_command(cmd)
    dl = time.time() + 30
    buf = b""
    while time.time() < dl:
        if ch.recv_ready():
            buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready():
            break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

for c in [
    "systemctl is-active youyuan.service; systemctl show youyuan.service -p ActiveEnterTimestamp -p MainPID | head -2",
    "ls -l --time-style=full-iso /opt/youyuan/youyuan /opt/youyuan/youyuan.new 2>&1",
    "ss -ltn | grep 19429 || echo PORT_DOWN",
    "curl -s -m 3 http://127.0.0.1:19429/health || echo HEALTH_FAIL",
    "uptime",
]:
    print(f"$ {c}\n{run(c)}{'-'*40}")
t.close()
