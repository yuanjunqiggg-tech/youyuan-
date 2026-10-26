#!/usr/bin/env python3
"""下载雪球菜单 txt 全文 + 查 chains 运行状态。"""
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
# 全部链 txt 拉到本地
sftp = paramiko.SFTPClient.from_transport(t)
import os
os.makedirs(r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/maintenance/chain-dump", exist_ok=True)
for name in ["雪球菜单_综合功能.txt", "药水系统.txt", "yygun_native_hud_r56.txt"]:
    lp = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/maintenance/chain-dump/" + name
    sftp.get(base + "/" + name, lp)
    print("已下载", name, os.path.getsize(lp), "字节")
sftp.close()
print()
print("== 雪球菜单各段状态头 ==")
print(run("grep -nE '^[0-9]+(,[0-9]+)*,[0-9]+([ \t]+[0-9.]+)?' '" + base + "/雪球菜单_综合功能.txt'", 30).strip())
print()
print("== 药水系统各段状态头 ==")
print(run("grep -nE '^[0-9]+(,[0-9]+)*,[0-9]+([ \t]+[0-9.]+)?' '" + base + "/药水系统.txt'", 30).strip())
print()
print("== json 配置 ==")
print(run("cat '" + base + "/雪球菜单_综合功能.txt.json'; echo; cat '" + base + "/药水系统.txt.json'", 20).strip())
print()
print("== chains 状态 API ==")
for ep in ["/api/youyuan/chains", "/api/chains/list"]:
    out = run("curl -s -m 5 http://127.0.0.1:19429" + ep + " | head -c 3000", 15)
    print("[" + ep + "]", out[:3000])
    print()
t.close()
