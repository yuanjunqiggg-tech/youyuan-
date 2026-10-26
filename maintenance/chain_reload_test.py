#!/usr/bin/env python3
"""验证：chains.reload 刷新运行时 Plan 后，雪球链能否真正跑起来。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=150):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

REMOTE = r'''
import json, time, subprocess, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=20):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
NAME = "雪球菜单_综合功能.txt"
def full():
    for f in api("GET","/api/youyuan/chains")["files"]:
        if f["config"]["name"] == NAME:
            return f
def cpu():
    out = subprocess.run(["sh","-c","top -bn1 | head -20 | grep youyuan"], capture_output=True, text=True).stdout
    return [l.split()[8:10] for l in out.strip().splitlines()][:2]
print("1) reload:", str(api("POST","/api/youyuan/chains", {"action":"reload","name":NAME}))[:120])
r = api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
print("2) start:", str(r)[:120])
for i in range(3):
    time.sleep(4)
    f = full()
    segs = f.get("segments") or []
    print(f"t={4*(i+1)}s state={f['state']} submitted={f['submitted']} hz={f['actual_hz']:.1f} segs={len(segs)}")
    for s in segs:
        print("   seg", s.get("segment"), s.get("state"), "sub", s.get("submitted"), "hz", s.get("actual_hz"), "err", str(s.get("error"))[:80])
    print("   cpu:", cpu())
    if f['submitted'] > 0 and i >= 1:
        break
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
time.sleep(2)
print("3) stopped:", full()["state"])
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_reload_test.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/chain_reload_test.py", 150).strip())
t.close()
