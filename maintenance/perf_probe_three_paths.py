#!/usr/bin/env python3
"""三路径 A/B：必须都带令牌才可比。token 不打印。"""
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

TOK = json.loads(run("cat /etc/youyuan/direct-gateway.json"))["console_token"]
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36"
EDGE = "https://toolbox.youyuanqi.dpdns.org"
TUN = "https://discussed-photographs-ten-chosen.trycloudflare.com"

def hit(url, headers, label, n=3):
    res = []
    for i in range(n):
        req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept": "application/json", **headers})
        t0 = time.perf_counter()
        try:
            with urllib.request.urlopen(req, timeout=40) as r:
                body = r.read(); dt = (time.perf_counter()-t0)*1000
                res.append(f"{dt:.0f}ms/{r.status}/{len(body)}B")
        except urllib.error.HTTPError as e:
            dt = (time.perf_counter()-t0)*1000
            res.append(f"{dt:.0f}ms/{e.code}/{len(e.read())}B")
        except Exception as e:
            res.append(f"ERR {e}")
    print(f"{label:34s} " + "  ".join(res))

print("== 三路径 A/B（同接口 /api/plugin/list，热连接）==")
hit(EDGE + "/api/plugin/list", {}, "A 边缘 401（无令牌）")
hit(EDGE + "/api/plugin/list", {"Authorization": "Bearer "+TOK, "X-YouYuan-Device": "linux-server"}, "B 中继全程（带令牌）")
hit(TUN + "/api/plugin/list", {"Authorization": "Bearer "+TOK}, "C 直连全程（带令牌）")
hit(TUN + "/api/plugin/list", {}, "D 直连 401（无令牌）")

print("\n== 网关回环开销（含新建上游连接+close+Go）==")
print(run("for i in 1 2 3; do curl -s -o /dev/null -w '%{time_total}s\\n' -H 'Authorization: Bearer '$(python3 -c \"import json;print(json.load(open('/etc/youyuan/direct-gateway.json'))['console_token'])\") http://127.0.0.1:19430/api/plugin/list; done"))
print(run("echo '--- 直连Go ---'; for i in 1 2 3; do curl -s -o /dev/null -w '%{time_total}s\\n' http://127.0.0.1:19429/api/plugin/list; done"))
t.close()
