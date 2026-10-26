#!/usr/bin/env python3
"""拉取 youyuan.service 最近的日志，用于诊断三个稳定性问题。

只读：不重启、不改配置。
"""
from __future__ import annotations

import hashlib
import json
import sys

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"


def run(client, cmd: str) -> str:
    _, out, err = client.exec_command(cmd, timeout=60)
    data = out.read().decode("utf-8", "replace")
    errs = err.read().decode("utf-8", "replace")
    return data + (("\n[stderr] " + errs) if errs.strip() else "")


def main() -> int:
    cfg = json.load(open(CFG_PATH))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    key = transport.get_remote_server_key()
    got = hashlib.sha256(key.asbytes()).hexdigest()
    if got.lower() != pin:
        print(f"[x] host key mismatch: got={got} pinned={pin}")
        return 2
    transport.auth_password(cfg["username"], cfg["password"])
    client = paramiko.SSHClient()
    client._transport = transport

    print("=== 1. 服务运行状态 ===")
    print(run(client, "systemctl status youyuan --no-pager -n 0 | head -12"))

    print("=== 2. 最近的断开/踢出/错误（200 行内）===")
    print(run(client,
              "journalctl -u youyuan -n 400 --no-pager "
              "| grep -Ei 'disconnect|踢|断开|死亡|death|respawn|重生|panic|error|超时|timeout' "
              "| tail -40"))

    print("=== 3. 重启次数与时间 ===")
    print(run(client, "systemctl show youyuan -p NRestarts -p ExecMainStartTimestamp"))

    print("=== 4. 内存与句柄 ===")
    print(run(client, "free -m | head -2; echo '---'; "
                      "ls /proc/$(systemctl show -p MainPID --value youyuan)/fd 2>/dev/null | wc -l"))

    transport.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
