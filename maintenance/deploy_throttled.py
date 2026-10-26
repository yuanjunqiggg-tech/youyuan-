#!/usr/bin/env python3
"""限速部署：150KB/s 分块上传 + 每15s检查机器人健康，掉线自动暂停等待恢复。"""
import hashlib, json, sys, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
LOCAL = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/build/youyuan-linux-x86_64"
EXPECT = "2a5a58d5447fc8fbfdb88e794fe297f38a0e2c182ced6edbcc668d56886ff914"
RATE = 150 * 1024  # B/s
CHUNK = 32 * 1024  # 32KB per write
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

def bot_ok():
    out = run("curl -s -m 3 http://127.0.0.1:19429/api/bot/status", 10)
    return '"connected":true' in out

sftp = paramiko.SFTPClient.from_transport(t)
st = sftp.open("/opt/youyuan/youyuan.v078", "wb")
data = open(LOCAL, "rb").read()
total = len(data)
sent = 0
t0 = time.time()
paused = 0
while sent < total:
    n = min(CHUNK, total - sent)
    st.write(data[sent:sent+n])
    sent += n
    # 限速
    expect_t = t0 + sent / RATE
    d = expect_t - time.time()
    if d > 0:
        time.sleep(d)
    if sent % (1024 * 1024) < CHUNK:  # 每 ~1MB 检查一次机器人
        if not bot_ok():
            paused += 1
            print(f"[{sent//1048576}MB] 机器人掉线，暂停 20s（第 {paused} 次）", flush=True)
            time.sleep(20)
st.close()
print(f"上传完成 {total} 字节，暂停 {paused} 次，耗时 {(time.time()-t0)/60:.1f} 分钟", flush=True)
# 远端校验
out = run("sha256sum /opt/youyuan/youyuan.v078", 90)
print("远端:", out.strip(), flush=True)
if EXPECT not in out:
    print("SHA256 不匹配，终止（不换文件）", flush=True); sys.exit(2)
print("=== 校验通过，执行原子替换与重启 ===", flush=True)
print(run("sudo systemctl stop youyuan.service; sudo mv /opt/youyuan/youyuan.v078 /opt/youyuan/youyuan; sudo chmod 755 /opt/youyuan/youyuan; sudo systemctl start youyuan.service; sleep 4; systemctl is-active youyuan.service; curl -s -m 4 http://127.0.0.1:19429/health | head -c 200", 90), flush=True)
t.close()
