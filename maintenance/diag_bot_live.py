#!/usr/bin/env python3
"""实时体检：机器人连接状态、收包积压（回调延迟代理）、命令链负载、进程 CPU。

只读：只发 GET，不启动/停止任何任务。
"""
from __future__ import annotations

import hashlib
import json
import sys
import time

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"
BASE = "http://127.0.0.1:19429"


def main() -> int:
    cfg = json.load(open(CFG_PATH))
    pin = cfg["host_key_sha256"].replace(":", "").lower()

    transport = paramiko.Transport((cfg["host"], 22))
    transport.start_client(timeout=20)
    if hashlib.sha256(transport.get_remote_server_key().asbytes()).hexdigest().lower() != pin:
        print("[x] host key mismatch")
        return 2
    transport.auth_password(cfg["username"], cfg["password"])
    client = paramiko.SSHClient()
    client._transport = transport

    def run(cmd: str, timeout: int = 60) -> str:
        _, out, err = client.exec_command(cmd, timeout=timeout)
        data = out.read().decode("utf-8", "replace")
        e = err.read().decode("utf-8", "replace")
        return data + (("\n[stderr] " + e) if e.strip() else "")

    print("=== 1. 机器人连接状态 ===")
    print(run(f"curl -s -m 8 {BASE}/api/bot/status"))
    print()

    print("=== 2. 收包积压采样（每 2 秒一次，共 6 次）===")
    print("depth 持续上涨 = 消费跟不上，回调延迟在累积")
    for i in range(6):
        raw = run(f"curl -s -m 8 {BASE}/api/bot/status").strip()
        try:
            d = json.loads(raw)
            print(f"  #{i+1} connected={d.get('connected')} is_op={d.get('is_op')} "
                  f"server={d.get('server')} depth={d.get('packet_queue_depth')}/{d.get('packet_queue_capacity')}")
        except Exception:
            print(f"  #{i+1} 解析失败: {raw[:120]}")
        if i < 5:
            time.sleep(2)
    print()

    print("=== 3. 命令链运行状态 ===")
    chains = run(f"curl -s -m 10 {BASE}/api/youyuan/chains")
    try:
        data = json.loads(chains)
        items = data.get("chains") or data.get("items") or []
        if not items:
            print("  （没有已保存的链）")
        for c in items:
            print(f"  {c.get('name')}  running={c.get('running')}  "
                  f"mode={c.get('mode')}  scheduling={c.get('scheduling')}  "
                  f"interval_ms={c.get('interval_ms')}  submitted={c.get('submitted')}")
    except Exception:
        print("  " + chains[:400])
    print()

    print("=== 4. HTTP 往返（本机回环，连测 5 次）===")
    print(run("for i in 1 2 3 4 5; do "
              f"curl -s -o /dev/null -w '%{{time_total}}s ' -m 8 {BASE}/api/bot/status; done; echo"))

    print("=== 5. 进程 CPU / 内存 ===")
    print(run("PID=$(systemctl show -p MainPID --value youyuan); "
              "ps -p $PID -o pid,pcpu,pmem,rss,etime,cmd --no-headers; "
              "echo '--- 子进程 ---'; "
              "ps --ppid $PID -o pid,pcpu,rss,etime,cmd --no-headers | head -5"))

    print("=== 6. 系统负载与内存 ===")
    print(run("uptime; echo '---'; free -m | head -2"))

    print("=== 7. 近 15 分钟是否发生过踢出/断开 ===")
    print(run("journalctl -u youyuan --since '-15 min' --no-pager "
              "| grep -Ei 'disconnect|踢|断开|panic|error|timeout|超时' | tail -20") or "  （无）")

    transport.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
