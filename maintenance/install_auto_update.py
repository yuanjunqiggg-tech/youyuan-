#!/usr/bin/env python3
"""把自动更新脚本与 systemd timer 装到 Linux 生产机并启用。"""
from __future__ import annotations

import json
import time

import paramiko

CFG = r"D:/YouYuan-APK/private/linux-access.json"
LOCAL = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/maintenance/youyuan-auto-update.sh"
REMOTE_SH = "/opt/youyuan/auto-update.sh"
REMOTE_STATE = "/var/lib/youyuan-update/installed-code"

SERVICE = """[Unit]
Description=YouYuan toolbox auto update
After=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/youyuan/auto-update.sh
TimeoutStartSec=600
"""

TIMER = """[Unit]
Description=Run youyuan auto update every 6 hours

[Timer]
OnBootSec=10min
OnUnitActiveSec=6h
Persistent=true

[Install]
WantedBy=timers.target
"""


def main() -> int:
    cfg = json.load(open(CFG))
    t = paramiko.Transport((cfg["host"], 22))
    t.start_client(timeout=20)
    import hashlib
    if hashlib.sha256(t.get_remote_server_key().asbytes()).hexdigest() != cfg["host_key_sha256"].replace(":", "").lower():
        print("主机密钥不匹配"); return 3
    t.auth_password(cfg["username"], cfg["password"])

    def run(cmd, quiet=False):
        ch = t.open_session(timeout=15); ch.exec_command(cmd)
        buf = b""
        deadline = time.time() + 120
        while time.time() < deadline:
            if ch.recv_ready(): buf += ch.recv(65536)
            elif ch.exit_status_ready() and not ch.recv_ready(): break
            time.sleep(0.2)
        out = buf.decode("utf-8", errors="replace")
        if not quiet: print(f"$ {cmd}\n{out}[exit {ch.recv_exit_status()}]")
        ch.close(); return out

    sftp = paramiko.SFTPClient.from_transport(t)
    # Windows 上传必须转 LF，否则 bash 报 syntax error near `done`
    body = open(LOCAL, "rb").read().replace(b"\r\n", b"\n")
    import io as _io
    sftp.putfo(_io.BytesIO(body), REMOTE_SH)
    sftp.close()
    run(f"chmod +x {REMOTE_SH} && chown root:root {REMOTE_SH}")
    # 当前已装版本写进状态文件，避免首轮重复重启（当前即 0.10.77=1077）
    run(f"mkdir -p {REMOTE_STATE.rsplit('/',1)[0]} && echo 1077 > {REMOTE_STATE}")
    sftp2 = paramiko.SFTPClient.from_transport(t)
    import tempfile, os
    for name, content in [("youyuan-update.service", SERVICE), ("youyuan-update.timer", TIMER)]:
        tmp = os.path.join(tempfile.gettempdir(), name)
        open(tmp, "w", encoding="utf-8", newline="\n").write(content)
        sftp2.put(tmp, f"/etc/systemd/system/{name}")
    sftp2.close()
    run("systemctl daemon-reload && systemctl enable --now youyuan-update.timer")
    run("systemctl list-timers youyuan-update.timer --no-pager | head -4")
    # 立即手动触发一次，验证脚本逻辑（应因 code 相同而静默退出）
    run(f"{REMOTE_SH}; echo MANUAL-RUN-EXIT=$?")
    t.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
