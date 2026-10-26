#!/usr/bin/env python3
"""雪球原文件 preview 解析 + systemd 进程双开排查。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=120):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

print("== systemd 状态 ==")
print(run("systemctl status youyuan.service --no-pager -n 5 | head -20", 30).strip())
print()
print("== 两进程的父与启动参数 ==")
print(run("ps -o pid,ppid,lstart,cmd -p 2024 2034 2>/dev/null; cat /proc/2034/cmdline 2>/dev/null | tr '\\0' ' '; echo; ls -l /proc/2034/cwd /proc/2024/cwd 2>/dev/null", 30).strip())
print()
REMOTE = r'''
import json, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=20):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
content = open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/雪球菜单_综合功能.txt", encoding="utf-8").read()
p = api("POST","/api/youyuan/chains", {"action":"preview","name":"雪球菜单_综合功能.txt","content":content,"config":{"name":"雪球菜单_综合功能.txt","account":"main","interval_ms":50,"channel":"ai","mode":"fast","scheduling":"segments"}})
print("preview ok =", p.get("ok"), "err =", str(p.get("error"))[:200])
plan = p.get("plan") or {}
chains = plan.get("chains") or []
print("段数 =", len(chains))
for c in chains:
    print("  type", c.get("type"), "hz", c.get("hz"), "nodes", len(c.get("nodes") or []))
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_preview.py", "w") as f:
    f.write(REMOTE)
print("== 雪球原文件 preview ==")
print(run("python3 /tmp/chain_preview.py", 90).strip())
t.close()
