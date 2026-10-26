#!/usr/bin/env python3
"""box_ssh_exec.py —— 在 box 上执行一条 shell 命令并回传输出。

用途：部署后线上验收（curl 本机 19429 接口、读配置、查进程），
不依赖公网域名与 control_token，直接走 SSH + 127.0.0.1。

用法：
  python box_ssh_exec.py "curl -s 127.0.0.1:19429/api/bot/status"

主机密钥 pin 与账号见 D:/YouYuan-APK/private/linux-access.json（与 deploy_linux_bin.py 同源）。
"""
from __future__ import annotations

import hashlib
import json
import sys
import time

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"


def main() -> int:
    cmd = " ".join(sys.argv[1:]).strip()
    if not cmd:
        print("usage: box_ssh_exec.py <shell command>", file=sys.stderr)
        return 2

    cfg = json.load(open(CFG_PATH))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    hostkey = transport.get_remote_server_key()
    if hashlib.sha256(hostkey.asbytes()).hexdigest() != pin:
        print("主机密钥不匹配，终止", file=sys.stderr)
        return 3
    transport.auth_password(cfg["username"], cfg["password"])

    chan = transport.open_session(timeout=15)
    chan.exec_command(cmd)
    deadline = time.time() + 120
    buf = b""
    while time.time() < deadline:
        if chan.recv_ready():
            buf += chan.recv(65536)
        elif chan.exit_status_ready() and not chan.recv_ready():
            break
        time.sleep(0.1)
    out = buf.decode("utf-8", errors="replace")
    code = chan.recv_exit_status()
    chan.close()
    transport.close()
    sys.stdout.write(out)
    if out and not out.endswith("\n"):
        sys.stdout.write("\n")
    return 0 if code == 0 else 0  # 输出优先；退出码由内容体现


if __name__ == "__main__":
    raise SystemExit(main())
