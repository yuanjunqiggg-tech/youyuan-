#!/usr/bin/env python3
"""核查 0.10.78 限速部署结果：sha256 / 服务状态 / health 版本 / 机器人状态 / 隧道地址。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
EXPECT = "36147df8250424c0a9389cd84fee1d992d0017e2868c7db0670ecaf3295b5382"
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

print("== sha256 ==")
print(run("sha256sum /opt/youyuan/youyuan 2>&1", 90).strip())
print("== 残留临时文件 ==")
print(run("ls -la /opt/youyuan/ | head -20", 30).strip())
print("== 服务状态 ==")
print(run("systemctl is-active youyuan.service; systemctl is-active youyuan-direct-tunnel.service 2>&1", 20).strip())
print("== health ==")
print(run("curl -s -m 5 http://127.0.0.1:19429/health", 15).strip())
print()
print("== bot status ==")
print(run("curl -s -m 5 http://127.0.0.1:19429/api/bot/status | head -c 800", 15).strip())
print()
print("== 定时炸弹状态 ==")
print(run("systemctl is-enabled youyuan-update.timer 2>&1", 15).strip())
print("== 隧道地址 ==")
print(run("journalctl -u youyuan-direct-tunnel.service --no-pager -n 8 2>&1 | grep -oE 'https://[a-z0-9-]+\\.trycloudflare\\.com' | tail -2", 30).strip())
t.close()
