#!/usr/bin/env python3
"""分析生产机 youyuan 服务的内存构成，找出可回收的部分。

只读诊断：不重启、不修改任何文件。输出各进程 RSS、Go 堆指标、
匿名映射分布与可能的冗余（旧二进制、日志、临时文件）。

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

    def run(cmd: str) -> str:
        chan = transport.open_session(timeout=15)
        chan.exec_command(cmd)
        deadline = time.time() + 120
        buf = b""
        while time.time() < deadline:
            if chan.recv_ready():
                buf += chan.recv(65536)
            elif chan.exit_status_ready() and not chan.recv_ready():
                break
            time.sleep(0.15)
        return buf.decode("utf-8", errors="replace")

    print("=" * 70)
    print("[1] 整机内存")
    print(run("free -m"))

    print("=" * 70)
    print("[2] youyuan 相关进程")
    print(run("ps -eo pid,ppid,rss,vsz,pcpu,etime,comm --sort=-rss | head -15"))

    print("=" * 70)
    print("[3] 主进程与子进程 RSS（KB）")
    print(run("pgrep -a youyuan || echo 'no youyuan process'"))

    print("=" * 70)
    print("[4] 服务状态")
    print(run("systemctl status youyuan.service --no-pager -l | head -20"))

    print("=" * 70)
    print("[5] 各 PID 的 VmRSS / VmSize / 线程数 / 打开句柄")
    print(run(
        "for p in $(pgrep youyuan); do "
        "echo \"--- PID $p ---\"; "
        "grep -E 'VmRSS|VmSize|VmData|Threads' /proc/$p/status 2>/dev/null; "
        "echo -n 'fds: '; ls /proc/$p/fd 2>/dev/null | wc -l; "
        "done"
    ))

    print("=" * 70)
    print("[6] 主进程内存映射 TOP（匿名段 = 堆/goroutine 栈）")
    print(run(
        "p=$(pgrep -o youyuan); "
        "if [ -n \"$p\" ]; then awk '/^[0-9a-f]/ {addr=$1; perm=$2; path=$6} "
        "/^Rss:/ {rss+=$2} "
        "END {}' /proc/$p/smaps_rollup 2>/dev/null; "
        "cat /proc/$p/smaps_rollup 2>/dev/null | head -20; fi"
    ))

    print("=" * 70)
    print("[7] 日志与临时文件占用")
    print(run(
        "du -sh /opt/youyuan 2>/dev/null; "
        "echo '--- 目录明细 ---'; "
        "du -sh /opt/youyuan/* 2>/dev/null | sort -rh | head -15; "
        "echo '--- 日志 ---'; "
        "journalctl -u youyuan.service --no-pager -n 0 --disk-usage 2>/dev/null; "
        "ls -lh /opt/youyuan/*.log 2>/dev/null | head"
    ))

    print("=" * 70)
    print("[8] 旧二进制 / 备份残留")
    print(run(
        "find /opt/youyuan -maxdepth 2 -type f -size +10M -printf '%s\\t%p\\n' 2>/dev/null "
        "| sort -rn | head -15 | awk '{printf \"%.1f MB\\t%s\\n\", $1/1048576, $2}'"
    ))

    print("=" * 70)
    print("[9] 内存碎片（buddyinfo / 可用）")
    print(run("cat /proc/buddyinfo; echo '---'; cat /proc/meminfo | head -8"))

    print("=" * 70)
    print("[10] Go 运行时指标（若开启 pprof 或状态接口）")
    print(run(
        "curl -s -m 3 http://127.0.0.1:19429/api/youyuan/status 2>/dev/null | head -c 1200; echo"
    ))

    transport.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
