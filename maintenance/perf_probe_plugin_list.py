#!/usr/bin/env python3
"""实测 /api/plugin/list 耗时（只读探测）。"""
import hashlib, json, time, sys
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
        time.sleep(0.05)
    try: ch.close()
    except Exception: pass
    return buf.decode("utf-8", errors="replace")

print("== 服务启动时间 ==")
print(run("systemctl show youyuan.service -p ActiveEnterTimestamp -p ExecMainStartTimestamp -p ActiveState -p NRestarts; date -Is", 20).strip())

print("\n== 插件目录 ==")
print(run("ls -la /opt/youyuan/ 2>&1 | head -20; echo ---; du -sh /opt/youyuan/* 2>/dev/null | sort -h | tail -12", 40).strip())

print("\n== 进程/负载 ==")
print(run("uptime; ps -o pid,etime,%cpu,%mem,rss,cmd -C youyuan --no-headers 2>/dev/null | head -5", 20).strip())

print("\n== 插件文件目录 ==")
print(run("for d in $(find / -maxdepth 6 -type d -name '插件文件' 2>/dev/null | head -5); do echo \"DIR=$d\"; ls -la \"$d\" | head -30; du -sh \"$d\"; done", 60).strip())

print("\n== 三连测 plugin/list（含大小/状态码）==")
print(run("for i in 1 2 3; do curl -s -o /tmp/pl.json -w 'try'$i' total=%{time_total}s connect=%{time_connect}s ttfb=%{time_starttransfer}s size=%{size_download} code=%{http_code}\\n' http://127.0.0.1:19429/api/plugin/list; done; echo; head -c 400 /tmp/pl.json", 90).strip())

print("\n== 对照接口 ==")
for p in ["/api/bot/status", "/api/task/list", "/api/youyuan/chains", "/api/youyuan/runtime/list", "/api/youyuan/agent/workspaces", "/health"]:
    print(p)
    print(run("for i in 1 2 3; do curl -s -o /dev/null -w '  total=%{time_total}s size=%{size_download} code=%{http_code}\\n' http://127.0.0.1:19429" + p + "; done", 90).strip())

print("\n== 并发 6 个 plugin/list ==")
print(run("for i in $(seq 6); do curl -s -o /dev/null -w 'c'$i' %{time_total}s\\n' http://127.0.0.1:19429/api/plugin/list & done; wait", 90).strip())

t.close()
