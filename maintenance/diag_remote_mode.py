#!/usr/bin/env python3
"""诊断 remote 模式：save 后看 config.scheduling/segmented 字段是否被重置。"""
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
import json, urllib.request
BASE = "http://127.0.0.1:19429"
def api(method, path, body=None, to=25):
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
for mode in ["remote","fast"]:
    r = api("POST","/api/youyuan/chains", {"action":"save","name":NAME,"content":content,
      "config":{"name":NAME,"account":"main","interval_ms":50,"channel":"ai","mode":mode,"scheduling":"segments","bootstrap":True,"enabled":False}})
    f = row()
    print(f"[{mode}] save.ok={r.get('ok')} cfg.scheduling={f['config'].get('scheduling')!r} segmented={f['segmented']} commands={f['commands']} state={f['state']}")
print("--- 磁盘 json ---")
print(open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/" + NAME + ".json", encoding="utf-8").read())
'''
sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/diag_remote.py", "w") as f:
    f.write(REMOTE)
print(run("python3 /tmp/diag_remote.py", 120).strip())
t.close()
