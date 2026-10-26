#!/usr/bin/env python3
"""远古模式（remote 逐条确认）长测 50 秒：看 bootstrap 确认耗时、最终状态、提交速率。"""
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
import json, time, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=30):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())
NAME = "雪球菜单_综合功能.txt"
def row():
    for f in api("GET","/api/youyuan/chains")["files"]:
        if f["config"]["name"] == NAME: return f
content = open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/" + NAME, encoding="utf-8").read()
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,
  "config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"remote","scheduling":"segments","bootstrap":True,"enabled":False}})
api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
prev=0; t0=time.time()
for i in range(10):
    time.sleep(5)
    f=row(); segs=f.get("segments") or []
    d=" | ".join(f"s{s.get('segment')}:{s.get('state')}/sub{s.get('submitted')}/err{str(s.get('error'))[:20]}" for s in segs)
    print(f"t={int(time.time()-t0)}s state={f['state']} sub={f['submitted']}(+{f['submitted']-prev}) hz={f['actual_hz']:.2f} segs={len(segs)}")
    if d: print("   ", d)
    prev=f['submitted']
    if f['state'] in ("错误暂停","已完成","部分段错误暂停"): break
api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
time.sleep(1)
# 恢复 fast（保持用户原配置）
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,
  "config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","scheduling":"segments","bootstrap":True,"enabled":False}})
print("restored mode=fast")
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/diag_remote_long.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/diag_remote_long.py", 180).strip())
t.close()
