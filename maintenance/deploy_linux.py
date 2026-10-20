#!/usr/bin/env python3
"""通过 SSH 连接 Linux 生产机执行命令。

- 主机密钥按 linux-access.json 的 host_key_sha256 pin 校验，不匹配直接退出
- 用法：python deploy_linux.py "命令1" "命令2" ...
- 不带命令时进入探查模式：打印服务状态与二进制信息
"""
from __future__ import annotations

import base64
import hashlib
import json
import sys
import time

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"

PROBE = (
    "uname -sr; "
    "systemctl is-active youyuan.service; "
    "systemctl cat youyuan.service 2>/dev/null | grep -E 'ExecStart|WorkingDirectory'; "
    "ls -la /opt/youyuan 2>/dev/null | head -10; "
    "ss -tlnp 2>/dev/null | grep 19429 || echo 'PORT 19429 NOT LISTENING'"
)


def main() -> int:
    cfg = json.load(open(CFG_PATH))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    hostkey = transport.get_remote_server_key()
    blob = hostkey.get_name().encode() + b" " + base64.b64encode(hostkey.asbytes())
    # pin 既可能是 sha256(blob) 的 hex，也可能是 OpenSSH 风格的 base64(SHA256)
    candidates = {
        hashlib.sha256(hostkey.asbytes()).hexdigest(),
        base64.b64encode(hashlib.sha256(hostkey.asbytes()).digest()).rstrip(b"=").decode().lower(),
    }
    if pin not in candidates:
        print(f"主机密钥不匹配：期望 {pin}，实际候选 {candidates}", file=sys.stderr)
        return 3
    transport.auth_password(cfg["username"], cfg["password"])

    def run(cmd: str) -> None:
        chan = transport.open_session(timeout=15)
        chan.exec_command(cmd)
        deadline = time.time() + 120
        buf = b""
        while time.time() < deadline:
            if chan.recv_ready():
                buf += chan.recv(65536)
            elif chan.exit_status_ready() and not chan.recv_ready():
                break
            else:
                time.sleep(0.2)
        sys.stdout.write(buf.decode("utf-8", errors="replace"))
        code = chan.recv_exit_status()
        print(f"[exit {code}]")
        chan.close()

    cmds = sys.argv[1:] or [PROBE]
    for cmd in cmds:
        print(f"$ {cmd}")
        run(cmd)
    transport.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
