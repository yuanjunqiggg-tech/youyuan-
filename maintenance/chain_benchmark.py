#!/usr/bin/env python3
"""命令链卡顿对照测试：在服务器上跑 4 条测试链，定位卡顿来源。
矩阵：
  A 低频少量(1Hz x 1)  —— 基线
  B 高频少量(20Hz x 3) —— 执行层高频开销
  C 高频大量(20Hz x 160 轻命令) —— 命令量x频率（模拟雪球规模）
  D 雪球降频(2Hz x 雪球原文件) —— 同文件低频
监控：packet_queue_depth / youyuan CPU% / submitted 增速。
安全线：packet_queue_depth > 3000 或 CPU > 85% 立即停止当前链。
"""
import hashlib, json, time, subprocess
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=90):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

# ---- 在服务器上执行的测试脚本 ----
REMOTE = r'''
import json, time, subprocess, urllib.request

BASE = "http://127.0.0.1:19429"

def api(method, path, body=None, to=15):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE+path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=to) as r:
        return json.loads(r.read().decode())

def cpu_of_youyuan():
    out = subprocess.run(["sh","-c","top -bn1 | grep youyuan | head -1"], capture_output=True, text=True).stdout
    parts = out.split()
    return parts[8] if len(parts) > 8 else "?"

def qdepth():
    try: return api("GET","/api/bot/status").get("packet_queue_depth", -1)
    except Exception as e: return -999

def submitted_of(name):
    try:
        for f in api("GET","/api/youyuan/chains")["files"]:
            if f["config"]["name"] == name: return f.get("submitted",0), f.get("actual_hz",0), f.get("state","")
    except Exception: pass
    return -1,0,"?"

light = "/scoreboard players add @r yytest 1"
boot  = "1,0,0\n/scoreboard objectives add yytest dummy\n."
A_TXT = boot + "\n2,0,0 1\n" + light + "\n"
B_TXT = boot + "\n2,0,0 20\n" + light + "\n" + light + "\n" + light + "\n"
C_TXT = boot + "\n2,0,0 20\n" + (light + "\n") * 160
# D: 雪球原文件降频 2Hz（20 只出现于段头行）
src = open("/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt/雪球菜单_综合功能.txt", encoding="utf-8").read()
D_TXT = src.replace("2,0,0 20", "2,0,0 2")

TESTS = [
    ("yytest_a_低频少量.txt", A_TXT, 12),
    ("yytest_b_高频少量.txt", B_TXT, 12),
    ("yytest_c_高频大量.txt", C_TXT, 15),
    ("yytest_d_雪球降频2hz.txt", D_TXT, 20),
]

results = []
for name, content, dur in TESTS:
    cfgd = {"name": name, "account": "main", "interval_ms": 50, "channel": "ai", "mode": "fast", "scheduling": "segments", "enabled": False}
    r = api("POST","/api/youyuan/chains", {"action":"save","name":name,"content":content,"config":cfgd})
    if not r.get("ok"):
        results.append({"test": name, "error": "save失败: "+str(r.get("error"))[:120]}); continue
    s0,_,_ = submitted_of(name)
    row = {"test": name, "samples": []}
    api("POST","/api/youyuan/chains", {"action":"start","name":name})
    t0 = time.time()
    while time.time() - t0 < dur:
        time.sleep(5)
        s1,hz,st = submitted_of(name)
        q = qdepth(); c = cpu_of_youyuan()
        row["samples"].append({"t": round(time.time()-t0), "submitted": s1-s0, "hz": round(hz,1), "q": q, "cpu": c})
        if isinstance(q,int) and q > 3000:
            row["abort"] = "队列暴涨"; break
    api("POST","/api/youyuan/chains", {"action":"stop","name":name})
    time.sleep(2)
    s2,_,_ = submitted_of(name)
    row["total_submitted"] = s2 - s0
    results.append(row)

# 清理：全部停用 + 删除测试链
for name,_,_ in TESTS:
    try: api("POST","/api/youyuan/chains", {"action":"stop","name":name})
    except Exception: pass
    try: api("POST","/api/youyuan/chains", {"action":"delete","name":name})
    except Exception: pass

print(json.dumps(results, ensure_ascii=False, indent=1))
'''

sftp = paramiko.SFTPClient.from_transport(t)
with sftp.open("/tmp/chain_bench.py", "w") as f:
    f.write(REMOTE)
print("== 服务器环境 ==")
print(run("which python3; top -bn1 | grep youyuan | head -1", 30).strip())
print()
print("== 前置状态（机器人必须在线才测）==")
print(run("curl -s -m 5 http://127.0.0.1:19429/api/bot/status", 15).strip())
print()
print("== 跑对照测试（约 80 秒）==")
out = run("python3 /tmp/chain_bench.py", 240)
print(out.strip())
t.close()
