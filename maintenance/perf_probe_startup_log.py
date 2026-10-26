#!/usr/bin/env python3
"""只读：拉取 youyuan 启动阶段日志时间线。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22)); t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=90):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.05)
    try: ch.close()
    except Exception: pass
    return buf.decode("utf-8", errors="replace")

print("== 启动阶段日志（02:30 起）==")
print(run("journalctl -u youyuan.service --since '2026-10-21 02:30:00' --no-pager -o short-iso 2>&1 | head -80", 60))
print("\n== 插件相关日志 ==")
print(run("journalctl -u youyuan.service --since '2026-10-21 02:30:00' --no-pager -o short-iso 2>&1 | grep -E '插件|plugin|wordbank|词库' | head -40", 60))
print("\n== 服务单元 ==")
print(run("systemctl cat youyuan.service 2>&1 | head -40", 20))
print("\n== 最近 60 行 ==")
print(run("journalctl -u youyuan.service --no-pager -o short-iso -n 40 2>&1", 60))
t.close()
