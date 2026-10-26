#!/usr/bin/env python3
"""深挖：youyuan 进程数 + 雪球链各段 worker 状态（20 秒窗口）。"""
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

print("== 进程全景 ==")
print(run("ps aux | grep -E 'youyuan|19429' | grep -v grep; echo ---; ss -tlnp 2>/dev/null | grep -E '19429|19430'", 30).strip())
print()
REMOTE = r'''
import json, time, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=15):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
NAME = "雪球菜单_综合功能.txt"
api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
for i in range(4):
    time.sleep(5)
    for f in api("GET","/api/youyuan/chains")["files"]:
        if f["config"]["name"] == NAME:
            print(f"t={5*(i+1)}s state={f['state']} submitted={f['submitted']} hz={f['actual_hz']:.1f} cycles={f['cycles']}")
            for s in (f.get("segments") or []):
                print("   seg", s.get("segment"), "type", s.get("type"), s.get("state"), "submitted", s.get("submitted"), "hz", s.get("actual_hz"), "err", str(s.get("error"))[:60])
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
print("stopped, enabled=", end="")
for f in api("GET","/api/youyuan/chains")["files"]:
    if f["config"]["name"] == NAME: print(f["config"]["enabled"])
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_seg.py", "w") as f:
    f.write(REMOTE)
print("== 雪球链 20 秒段级观察 ==")
print(run("python3 /tmp/chain_seg.py", 150).strip())
t.close()
