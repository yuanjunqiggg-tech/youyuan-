#!/usr/bin/env python3
"""查看服务器上命令链 txt 实际内容 + chains 运行状态（真实诊断第一步）。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=60):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")

print("== 链目录列表 ==")
print(run("ls -la '/opt/youyuan-data/插件数据文件/ds_ai_agent/command_txt/' 2>/dev/null || find / -maxdepth 6 -type d -name command_txt 2>/dev/null", 60).strip())
print("== dataDir 定位（health/config）==")
print(run("curl -s -m 5 http://127.0.0.1:19429/health; echo; ls /opt/ | head", 20).strip())
print("== chains API 状态 ==")
for ep in ["/api/chains", "/api/commands/chains", "/api/command/chains"]:
    out = run(f"curl -s -m 5 http://127.0.0.1:19429{ep} | head -c 1500", 15)
    if out and "404" not in out and "page not found" not in out:
        print(f"[{ep}]", out[:1500]); break
    else:
        print(f"[{ep}] 无")
t.close()
