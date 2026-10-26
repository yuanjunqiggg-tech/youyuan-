#!/usr/bin/env python3
"""复现测试：雪球菜单原文件原频率 20Hz 跑 8 秒，采样 CPU/速率，然后立即停止。"""
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

REMOTE = r'''
import json, time, subprocess, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=15):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
def cpu():
    out = subprocess.run(["sh","-c","top -bn1 | grep 'youyuan$\\|youyuan ' | head -3"], capture_output=True, text=True).stdout
    return out.strip().splitlines()
NAME = "雪球菜单_综合功能.txt"   # 用户原文件，enabled=false（当前停止状态）
def stat():
    try:
        for f in api("GET","/api/youyuan/chains")["files"]:
            if f["config"]["name"] == NAME:
                return f.get("submitted",0), f.get("actual_hz",0), f.get("state",""), f.get("last_round_ms",0)
    except Exception: pass
    return -1,0,"?",0
print("start前:", stat())
s0,_,_,_ = stat()
api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
for i in range(3):
    time.sleep(3)
    s1,hz,st,lr = stat()
    print(f"t={3*(i+1)}s submitted=+{s1-s0} (~{(s1-s0)/(3*(i+1)):.0f}/s) hz={hz:.1f} state={st} last_round={lr}ms")
    print("  top:", cpu())
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
time.sleep(2)
print("stop后:", stat(), "(enabled应回到false)")
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_repro.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/chain_repro.py", 120).strip())
t.close()
