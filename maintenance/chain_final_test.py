#!/usr/bin/env python3
"""最终方案测试：fast + rate_cps 限速（不动 txt），并解开 remote 模式段级卡死之谜。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=180):
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
def api(method, path, body=None, to=25):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
NAME = "雪球菜单_综合功能.txt"
def full():
    for f in api("GET","/api/youyuan/chains")["files"]:
        if f["config"]["name"] == NAME: return f
def show(f, prev):
    segs = f.get("segments") or []
    segs = sorted(segs, key=lambda s: s.get("segment") or 0)
    detail = " | ".join(f"s{s.get('segment')}:{s.get('state')}/sub{s.get('submitted')}" for s in segs)
    print(f"  submitted={f['submitted']}(+{f['submitted']-prev}) hz={f['actual_hz']:.2f} state={f['state']} confirmed={f.get('results_confirmed')}")
    print(f"  segs: {detail}")
    return f["submitted"]

# --- 1) remote 模式段级谜团（短测 9 秒）---
content = open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/" + NAME, encoding="utf-8").read()
print("== A) remote 模式段级状态 ==")
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,"config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"remote","scheduling":"segments","bootstrap":True,"enabled":False}})
api("POST","/api/youyuan/chains", {"action":"reload","name":NAME})
api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
prev = 0
for i in range(2):
    time.sleep(4)
    prev = show(full(), prev)
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
time.sleep(1)

# --- 2) fast + rate_cps=200 限速 ---
print("== B) fast + rate_cps=200 ==")
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,"config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","rate_cps":200,"scheduling":"segments","bootstrap":True,"enabled":False}})
api("POST","/api/youyuan/chains", {"action":"reload","name":NAME})
api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
prev = 0
for i in range(3):
    time.sleep(4)
    prev = show(full(), prev)
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
time.sleep(1)

# --- 3) 恢复原始 fast 配置（保持现状，最终修复由主会话决定）---
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,"config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","scheduling":"segments","bootstrap":True,"enabled":False}})
print("restored: mode=fast 原配置")
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_final_test.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/chain_final_test.py", 180).strip())
t.close()
