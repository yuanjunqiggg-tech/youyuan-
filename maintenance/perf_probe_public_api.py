#!/usr/bin/env python3
"""用网关 owner console_token 实测「真实公网入口」下 /api/plugin/list 的端到端耗时。
token 只在本进程内使用，不打印。"""
import hashlib, json, time, urllib.request, urllib.error
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22)); t.start_client(timeout=30)
assert hashlib.sha256(t.get_remote_server_key().asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=60):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl=time.time()+to; buf=b""
    while time.time()<dl:
        if ch.recv_ready(): buf+=ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.03)
    ch.close(); return buf.decode("utf-8","replace")

raw = run("cat /etc/youyuan/direct-gateway.json")
conf = json.loads(raw)
tok = conf["console_token"]
print("token 长度:", len(tok), " owner:", conf.get("owner_username"))

W = "https://toolbox.youyuanqi.dpdns.org"

def probe(path, label, n=1, gap=0.0):
    for i in range(n):
        req = urllib.request.Request(W + path, headers={"Authorization": "Bearer " + tok, "Accept": "application/json"})
        t0 = time.perf_counter()
        try:
            with urllib.request.urlopen(req, timeout=40) as r:
                body = r.read()
                dt = time.perf_counter() - t0
                print(f"{label} #{i+1}: {dt*1000:.0f} ms  code={r.status} bytes={len(body)}")
        except urllib.error.HTTPError as e:
            dt = time.perf_counter() - t0
            print(f"{label} #{i+1}: {dt*1000:.0f} ms  code={e.code} body={e.read()[:120]!r}")
        except Exception as e:
            print(f"{label} #{i+1}: ERR {e}")
        if gap: time.sleep(gap)

print("\n== 公网入口 /api/plugin/list（热，连续 3 次）==")
probe("/api/plugin/list", "plugin/list", 3)
print("\n== 公网入口 对照接口 ==")
for p in ["/api/bot/status", "/api/task/list", "/api/youyuan/runtime/list", "/api/config"]:
    probe(p, p, 2)

print("\n== 等 70 秒让网关鉴权缓存(60s)过期，再打第一次 ==")
time.sleep(70)
probe("/api/plugin/list", "plugin/list(冷鉴权)", 3)

print("\n== 网关指标 ==")
print(run("curl -s -H 'Authorization: Bearer '$(python3 -c \"import json;print(json.load(open('/etc/youyuan/direct-gateway.json'))['console_token'])\") http://127.0.0.1:19430/api/cloud/direct-metrics"))
t.close()
