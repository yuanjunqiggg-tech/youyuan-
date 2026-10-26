#!/usr/bin/env python3
"""流畅度对比：fast+rate_cps 200 vs 400，测有效轮速与提交速率（恢复原配置）。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=200):
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
def trial(cps, dur=14):
    api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,
      "config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","rate_cps":cps,"scheduling":"segments","bootstrap":True,"enabled":False}})
    api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
    time.sleep(3)  # 跳过 bootstrap 首轮
    s0=row()["submitted"]; t0=time.time()
    time.sleep(dur)
    f=row(); dt=time.time()-t0; rate=(f["submitted"]-s0)/dt
    seg2=next((s for s in (f.get("segments") or []) if s.get("segment")==2), {})
    print(f"rate_cps={cps}: 实测 {rate:.0f} 条/秒  seg2轮速 {seg2.get('actual_hz',0):.2f}Hz  一轮约 {1000/max(0.01,seg2.get('actual_hz',0)):.0f}ms")
    api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
    time.sleep(1)
trial(200)
trial(400)
trial(600)
# 恢复原配置（fast 不限速，保持用户现状）
api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,
  "config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","scheduling":"segments","bootstrap":True,"enabled":False}})
print("restored mode=fast 原配置（未限速）")
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/diag_rate_trial.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/diag_rate_trial.py", 200).strip())
t.close()
