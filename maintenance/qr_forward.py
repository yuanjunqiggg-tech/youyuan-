#!/usr/bin/env python3
"""把生产机的 NapCat WebUI（127.0.0.1:6099）转发到本机 127.0.0.1:6099。

为什么不用 cloudflared 隧道：生产机上那个 tunnel 的凭据已经 retired
（token 解出来的 tunnel id 就是 JSON 里标记 deleted 的那个），连不上。
SSH 本地端口转发不需要动生产配置、不留公网入口，扫完码关掉即可。

用法：python maintenance/qr_forward.py
"""
from __future__ import annotations

import base64
import hashlib
import json
import select
import socket
import sys
import threading

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"
LOCAL_PORT = 6099


def pipe(a: socket.socket, b) -> None:
    try:
        while True:
            r, _, _ = select.select([a], [], [], 30)
            if not r:
                continue
            data = a.recv(65536)
            if not data:
                break
            b.sendall(data)
    except Exception:
        pass
    finally:
        try:
            b.close()
        except Exception:
            pass
        try:
            a.close()
        except Exception:
            pass


def main() -> int:
    cfg = json.load(open(CFG_PATH, encoding="utf-8"))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    hk = transport.get_remote_server_key()
    if hashlib.sha256(hk.asbytes()).hexdigest() != pin:
        print("主机密钥不匹配，终止", file=sys.stderr)
        return 3
    transport.auth_password(cfg["username"], cfg["password"])

    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", LOCAL_PORT))
    srv.listen(16)
    print(f"已就绪：http://127.0.0.1:{LOCAL_PORT}/webui  （Ctrl+C 结束）", flush=True)

    while True:
        client, addr = srv.accept()
        try:
            chan = transport.open_channel(
                "direct-tcpip", ("127.0.0.1", 6099), addr, timeout=10
            )
        except Exception as e:
            print("通道打开失败：", e, flush=True)
            client.close()
            continue
        threading.Thread(target=pipe, args=(client, chan), daemon=True).start()
        # 必须双向：只转发"客户端→服务端"的话，响应永远回不来，
        # 浏览器会一直转圈（我第一版就踩了这个坑）。
        threading.Thread(target=pipe, args=(chan, client), daemon=True).start()


if __name__ == "__main__":
    raise SystemExit(main())
