import hashlib, json, sys, time
import paramiko
cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])
def run(cmd, to=40):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.1)
    return buf.decode("utf-8", errors="replace")
for name, cmd in [
    ("19430监听", "ss -ltnp | grep 19430 || echo NO_LISTENER_19430"),
    ("全监听表", "ss -ltnp | head -12"),
    ("direct_gateway进程", "ps aux | grep -iE 'gateway|direct' | grep -v grep | head -5"),
    ("direct-tunnel服务定义", "systemctl cat youyuan-direct-tunnel.service 2>/dev/null | head -20"),
    ("隧道后端测试", "curl -s -m 3 http://127.0.0.1:19430/health || echo BACKEND_19430_DEAD"),
    ("19429直测", "curl -s -m 3 http://127.0.0.1:19429/health | head -c 200"),
]:
    print(f"===== {name} ====="); print(run(cmd))
t.close()
