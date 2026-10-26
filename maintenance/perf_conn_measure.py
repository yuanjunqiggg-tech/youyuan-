#!/usr/bin/env python3
"""测量 /api/bot/status 等接口：服务器本机直连 vs 线上经 Cloudflare 中继。只读，不改动服务。"""
import hashlib, json, time, subprocess, sys
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin, "host key mismatch"
t.auth_password(cfg["username"], cfg["password"])


def run(cmd, to=120):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time() + to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.05)
    return buf.decode("utf-8", errors="replace")


PATHS = ["/health", "/api/bot/status", "/api/config",
         "/api/youyuan/connection-policy", "/api/youyuan/remote", "/api/youyuan/events"]

print("== 服务器本机 127.0.0.1:19429 直连（3 次，秒） ==")
for p in PATHS:
    out = run('for i in 1 2 3; do curl -s -m 10 -o /dev/null -w "%{time_total} " '
              f'http://127.0.0.1:19429{p}; done; echo')
    print(f"{p:38s} {out.strip()}")

print()
print("== 服务器本机经 19430 入口（3 次，秒） ==")
for p in ["/health", "/api/bot/status"]:
    out = run('for i in 1 2 3; do curl -s -m 10 -o /dev/null -w "%{time_total} " '
              f'http://127.0.0.1:19430{p}; done; echo')
    print(f"{p:38s} {out.strip()}")

print()
print("== bot/status 返回体 ==")
print(run("curl -s -m 5 http://127.0.0.1:19429/api/bot/status").strip())
print()
print("== events 返回体大小 ==")
print(run("curl -s -m 5 -o /tmp/ev.json -w 'bytes=%{size_download} total=%{time_total}\\n' 'http://127.0.0.1:19429/api/youyuan/events?after=0'").strip())
print()
print("== 中继 WS 是否在线 / 隧道 ==")
print(run("systemctl is-active youyuan.service youyuan-direct.service youyuan-direct-tunnel.service 2>&1").strip())
print(run("ss -tnp 2>/dev/null | grep -c ESTAB").strip())
t.close()
