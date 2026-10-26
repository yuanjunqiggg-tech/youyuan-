#!/usr/bin/env python3
"""读服务器上全部命令链 txt 内容 + 找到正确的 chains 状态 API。"""
import hashlib, json, time
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

base = "/var/lib/youyuan/插件数据文件/ds_ai_agent/command_txt"
print("== 文件列表（ds_ai_agent 主目录）==")
print(run(f"ls -la '{base}/'", 30).strip())
print()
print("== r21b_test 目录 ==")
print(run("ls -la '/var/lib/youyuan/插件数据文件/ds_ai_agent_r21b_test/command_txt/' 2>/dev/null", 30).strip())
print()
print("== 全部 txt 头两行（Hz 配置）+ 行数 ==")
print(run("for f in '" + base + "'/*.txt; do n=$(wc -l < \"$f\"); echo \"--- $f ($n 行)\"; head -2 \"$f\"; done", 60).strip())
print()
print("== 二进制里搜 chains API 路由 ==")
print(run("curl -s -m 5 http://127.0.0.1:19429/api/commands/chains; echo; curl -s -m 5 http://127.0.0.1:19429/api/command-chains; echo", 20).strip())
print(run("strings /opt/youyuan/youyuan 2>/dev/null | grep -oE '/api/[a-z/-]*chain[a-z/-]*' | sort -u | head", 90).strip())
t.close()
