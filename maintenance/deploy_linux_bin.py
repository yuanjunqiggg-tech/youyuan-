#!/usr/bin/env python3
"""把新的 Linux 二进制部署到生产机并重启 systemd 服务。

流程：SFTP 上传到 /opt/youyuan/youyuan.new → 校验远端 sha256 与本地一致
→ systemctl stop → 原子 mv → start → 验证端口与进程。

主机密钥 pin 与账号见 D:/YouYuan-APK/private/linux-access.json。
"""
from __future__ import annotations

import hashlib
import json
import sys
import time

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"
LOCAL = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/build/youyuan-linux-x86_64"
REMOTE = "/opt/youyuan/youyuan.new"
TARGET = "/opt/youyuan/youyuan"
SERVICE = "youyuan.service"


def main() -> int:
    cfg = json.load(open(CFG_PATH))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    hostkey = transport.get_remote_server_key()
    if hashlib.sha256(hostkey.asbytes()).hexdigest() != pin:
        print("主机密钥不匹配，终止", file=sys.stderr)
        return 3
    transport.auth_password(cfg["username"], cfg["password"])

    def run(cmd: str, quiet: bool = False) -> str:
        chan = transport.open_session(timeout=15)
        chan.exec_command(cmd)
        deadline = time.time() + 180
        buf = b""
        while time.time() < deadline:
            if chan.recv_ready():
                buf += chan.recv(65536)
            elif chan.exit_status_ready() and not chan.recv_ready():
                break
            time.sleep(0.2)
        out = buf.decode("utf-8", errors="replace")
        if not quiet:
            print(f"$ {cmd}\n{out}[exit {chan.recv_exit_status()}]")
        chan.close()
        return out

    local_sha = hashlib.sha256(open(LOCAL, "rb").read()).hexdigest()
    print(f"本地 sha256 {local_sha}  大小 {len(open(LOCAL,'rb').read())}")

    sftp = paramiko.SFTPClient.from_transport(transport)
    print("上传中 ...")
    sftp.put(LOCAL, REMOTE)
    print("上传完成")
    remote_sha = run(f"sha256sum {REMOTE}", quiet=True).split()[0]
    if remote_sha != local_sha:
        print(f"远端校验失败：{remote_sha}", file=sys.stderr)
        return 4
    print("远端 sha256 一致")

    run(f"chmod +x {REMOTE} && chown root:root {REMOTE}")
    # Keep one rollback point: the running binary is copied aside before the
    # swap, so a bad build can be restored without re-uploading anything.
    run(f"cp -f {TARGET} {TARGET}.rollback && chmod +x {TARGET}.rollback")
    run(f"systemctl stop {SERVICE}")
    run(f"mv {REMOTE} {TARGET}")
    run(f"systemctl start {SERVICE}")
    time.sleep(3)
    run(f"systemctl is-active {SERVICE}")
    run("ss -tlnp | grep 19429 || echo 'PORT 19429 NOT LISTENING'")
    run("journalctl -u youyuan.service -n 6 --no-pager | tail -6")
    sftp.close()
    transport.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
