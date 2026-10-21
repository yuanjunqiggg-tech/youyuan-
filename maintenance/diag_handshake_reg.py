#!/usr/bin/env python3
"""取证：机器人对外握手包/重连风暴 + 注册链路健康。

只读诊断：不重启、不修改任何文件。
1) youyuan 服务状态与 PID
2) 出站 UDP 连接（机器人连的游戏服地址）
3) 日志中重连/断开/踢出/RakNet idle 的频次与最近样本
4) 10 秒抓包：按对端地址统计 UDP 包量（看是否有握手风暴）
5) 本机 MCP/认证端点健康（注册链路相关）

主机密钥 pin 与账号见 D:/YouYuan-APK/private/linux-access.json。
"""
from __future__ import annotations

import hashlib
import json
import sys
import time

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"


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

    def run(cmd: str, timeout: int = 60) -> str:
        chan = transport.open_session(timeout=15)
        chan.exec_command(cmd)
        deadline = time.time() + timeout
        buf = b""
        while time.time() < deadline:
            if chan.recv_ready():
                buf += chan.recv(65536)
            elif chan.exit_status_ready() and not chan.recv_ready():
                break
            time.sleep(0.15)
        return buf.decode("utf-8", errors="replace")

    print("=" * 70)
    print("[1] 服务状态")
    print(run("systemctl status youyuan.service --no-pager -l | head -12"))

    print("=" * 70)
    print("[2] youyuan 进程")
    print(run("pgrep -a youyuan || echo none"))

    print("=" * 70)
    print("[3] 出站 UDP（游戏服方向）")
    print(run("ss -unp | head -25"))

    print("=" * 70)
    print("[4] 近6小时断开/重连/踢出/idle 计数")
    print(run(
        "journalctl -u youyuan.service --no-pager --since '-6 hours' "
        "| grep -c -E '连接断开|被服务器踢出' ; "
        "echo '---上面是断开+踢出总数---'; "
        "journalctl -u youyuan.service --no-pager --since '-6 hours' "
        "| grep -c -E 'RakNet idle timeout' ; "
        "echo '---上面是 RakNet idle timeout 数---'; "
        "journalctl -u youyuan.service --no-pager --since '-6 hours' "
        "| grep -c -E '第1次尝试|第2次尝试|第3次尝试|尝试连接' ; "
        "echo '---上面是登录尝试数---'; "
        "journalctl -u youyuan.service --no-pager --since '-6 hours' "
        "| grep -c -E '登录失败|加入房间失败' ; "
        "echo '---上面是登录失败数---'"
    ))

    print("=" * 70)
    print("[5] 最近 50 条断开/重连/idle 样本")
    print(run(
        "journalctl -u youyuan.service --no-pager --since '-3 hours' "
        "| grep -E '连接断开|被服务器踢出|RakNet idle timeout|尝试连接|登录失败|初始化阶段断开' "
        "| tail -50"
    ))

    print("=" * 70)
    print("[6] 10 秒抓包：UDP 对端 TOP（看握手风暴）")
    print(run(
        "timeout 12 tcpdump -nn -q -c 200000 2>/dev/null | awk '{print $5}' "
        "| sed 's/\\.$//' | sort | uniq -c | sort -rn | head -15",
        timeout=30,
    ))

    print("=" * 70)
    print("[7] 本机端点健康（注册/MCP 链路）")
    print(run(
        "echo '--- status ---'; curl -s -m 3 -o /dev/null -w '%{http_code} %{time_total}s\\n' http://127.0.0.1:19429/api/youyuan/status; "
        "echo '--- mcp-links GET ---'; curl -s -m 3 -o /dev/null -w '%{http_code} %{time_total}s\\n' http://127.0.0.1:19429/api/youyuan/mcp-links; "
        "echo '--- auth 站 ---'; curl -s -m 5 -o /dev/null -w '%{http_code} %{time_total}s\\n' https://auth.youyuanqi.dpdns.org/ 2>&1; "
        "echo '--- identity worker ---'; curl -s -m 5 -o /dev/null -w '%{http_code} %{time_total}s\\n' https://identity.youyuanqi.dpdns.org/ 2>&1"
    ))

    transport.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
