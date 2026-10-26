#!/usr/bin/env python3
"""只读：确认对外访问链路（隧道/公网）、资源压缩与缓存头、整页加载字节数。"""
import hashlib, json, time
import paramiko

cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22)); t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])

def run(cmd, to=120):
    ch = t.open_session(timeout=20); ch.settimeout(to); ch.exec_command(cmd)
    dl = time.time()+to; buf = b""
    while time.time() < dl:
        if ch.recv_ready(): buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready(): break
        time.sleep(0.05)
    try: ch.close()
    except Exception: pass
    return buf.decode("utf-8", errors="replace")

print("== 相关服务 ==")
print(run("systemctl list-units --type=service --state=running --no-pager --no-legend 2>&1 | grep -iE 'youyuan|tunnel|cloudflare' ; echo ---; systemctl list-units --all --no-pager --no-legend 2>&1 | grep -iE 'tunnel' | head", 30).strip())

print("\n== 监听端口 ==")
print(run("ss -lntp 2>/dev/null | head -20", 20).strip())

print("\n== 隧道日志中的公网地址 ==")
print(run("journalctl -u youyuan-direct-tunnel.service --no-pager -n 40 2>&1 | grep -oE 'https://[a-z0-9-]+\\.trycloudflare\\.com' | tail -3; ls -la /opt/youyuan/*.log 2>/dev/null | head", 40).strip())

print("\n== 单资源响应头（压缩/缓存）==")
print(run("curl -s -D - -o /dev/null http://127.0.0.1:19429/js/vendor/three.min.js; echo '--- gzip 试探 ---'; curl -s -H 'Accept-Encoding: gzip, br' -D - -o /dev/null http://127.0.0.1:19429/js/vendor/three.min.js | grep -iE 'content-encoding|content-length|etag|last-modified|cache-control'", 40).strip())

print("\n== index.html 引用的脚本数与总体积 ==")
print(run("curl -s http://127.0.0.1:19429/ -o /tmp/idx.html; grep -oE 'src=\"[^\"]+\"|href=\"[^\"]+\"' /tmp/idx.html | sed 's/.*=\"//;s/\"//' | sort -u > /tmp/assets.txt; wc -l < /tmp/assets.txt; total=0; while read a; do sz=$(curl -s -o /dev/null -w '%{size_download}' \"http://127.0.0.1:19429$a\"); total=$((total+sz)); done < /tmp/assets.txt; echo \"total_bytes=$total\"", 120).strip())

print("\n== 本机 loopback 整页资源串行耗时（无网络延迟基线）==")
print(run("time (while read a; do curl -s -o /dev/null \"http://127.0.0.1:19429$a\"; done < /tmp/assets.txt) 2>&1 | tail -5", 120).strip())

print("\n== 出网带宽探测（服务器下载 5MB）==")
print(run("curl -s -o /dev/null -w 'down %{speed_download} B/s  time=%{time_total}s size=%{size_download}\\n' --max-time 20 https://speed.cloudflare.com/__down?bytes=5000000", 40).strip())
t.close()
