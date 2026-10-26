#!/usr/bin/env python3
"""修复验证：雪球链 mode=remote（旧行为）对照 fast+rate_cps，采样提交速率与 CPU。"""
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
def cpu():
    out = subprocess.run(["sh","-c","top -bn1 | grep youyuan | head -2"], capture_output=True, text=True).stdout
    return [l.split()[8] for l in out.strip().splitlines()][:2]

def trial(label, cfg_over, dur=12):
    print(f"--- {label} ---")
    content = open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/" + NAME, encoding="utf-8").read()
    cfgd = {"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":"fast","scheduling":"segments","bootstrap":True,"enabled":False}
    cfgd.update(cfg_over)
    r = api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,"config":cfgd})
    print("save:", str(r)[:150])
    api("POST","/api/youyuan/chains", {"action":"reload","name":NAME})
    api("POST","/api/youyuan/chains", {"action":"start","name":NAME})
    s_prev = 0
    for i in range(dur//4):
        time.sleep(4)
        f = full()
        segs = f.get("segments") or []
        seg2 = next((s for s in segs if s.get("segment")==2), {})
        print(f"t={4*(i+1)}s submitted={f['submitted']}(+{f['submitted']-s_prev}) hz={f['actual_hz']:.1f} state={f['state']} seg2.hz={seg2.get('actual_hz',0):.1f} cpu={cpu()}")
        s_prev = f["submitted"]
    api("POST","/api/youyuan/chains", {"action":"stop","name":NAME})
    time.sleep(2)
    print("stopped:", full()["state"])

trial("remote 模式（旧行为·逐条确认）", {"mode": "remote"})
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_remote_test.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/chain_remote_test.py", 180).strip())
t.close()
