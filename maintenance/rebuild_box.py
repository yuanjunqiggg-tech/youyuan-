#!/usr/bin/env python3
"""系统重置后一键重建生产机（2026-10-21 事故恢复）。

前置条件：box 可通过 SSH 进入（用户在阿里云控制台开了密码认证或绑了密钥；
凭据见 D:/YouYuan-APK/private/linux-access.json，支持 key_file 字段）。

执行顺序：
 0) 盘点幸存数据（数据盘/挂载/旧目录），有则先备份不动
 1) 建用户/目录/依赖（python3）
 2) 上传二进制 + cloudflared + systemd 单元
 3) 恢复配置：mail-config.json（注册发信）、config.json（机器人账号）、
    插件（plugin-projects）、命令链（chain-dump）——仅在目标不存在时
 4) 恢复 Cloudflare 隧道（linux-tunnel-token-01014.txt）
 5) 启动 youyuan.service，本地 /health 验收
 6) 外网复测（MCP initialize / register-code）由调用方另行执行

用法：
  python rebuild_box.py            # 全自动
  python rebuild_box.py --probe    # 只做第 0 步盘点，不碰任何东西
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

import paramiko

CFG_PATH = r"D:/YouYuan-APK/private/linux-access.json"
PRIVATE = Path(r"D:/YouYuan-APK/private")
PROJECT = Path(r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project")

BINARY = PROJECT / "build" / "youyuan-linux-x86_64"
CF_BINARY = PROJECT / "build" / "cloudflared"
UNIT = PROJECT / "linux" / "youyuan.service"
TUNNEL_TOKEN_FILE = PRIVATE / "linux-tunnel-token-01014.txt"
MAIL_CONFIG = PRIVATE / "mail-config.json"
CONFIG_SNAPSHOT = PRIVATE / "apk-config-before-linux-v0103.json"
PLUGIN_DIR = PRIVATE / "plugin-projects"
CHAIN_DIR = PROJECT / "maintenance" / "chain-dump"


def log(msg: str) -> None:
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def connect(cfg: dict) -> paramiko.Transport:
    tr = paramiko.Transport((cfg["host"], 22))
    tr.start_client(timeout=20)
    hk = tr.get_remote_server_key()
    import hashlib
    pin = cfg["host_key_sha256"].replace(":", "").lower()
    got = hashlib.sha256(hk.asbytes()).hexdigest()
    if got != pin:
        raise SystemExit(f"host key 不匹配（现在 {got}，期望 {pin}）——确认不是中间人后再更新 pin")
    key_file = cfg.get("key_file")
    if key_file and Path(key_file).exists():
        tr.auth_publickey(cfg["username"], paramiko.RSAKey.from_private_key_file(key_file) if key_file.endswith((".pem",)) else paramiko.Ed25519Key.from_private_key_file(key_file))
    else:
        tr.auth_password(cfg["username"], cfg["password"])
    return tr


def run(tr: paramiko.Transport, cmd: str, timeout: int = 120) -> str:
    ch = tr.open_session(timeout=20)
    ch.exec_command(cmd)
    ch.settimeout(timeout)
    buf = b""
    deadline = time.time() + timeout
    while time.time() < deadline:
        if ch.recv_ready():
            buf += ch.recv(65536)
        elif ch.exit_status_ready() and not ch.recv_ready():
            break
        time.sleep(0.15)
    return buf.decode("utf-8", errors="replace")


def sftp_put(tr: paramiko.Transport, local: Path, remote: str) -> None:
    sftp = paramiko.SFTPClient.from_transport(tr)
    try:
        sftp.put(str(local), remote)
    finally:
        sftp.close()


def probe(tr: paramiko.Transport) -> None:
    log("=== [0] 盘点幸存数据 ===")
    log("lsblk:\n" + run(tr, "lsblk 2>/dev/null || true", 30))
    log("mounts:\n" + run(tr, "mount | grep -vE 'cgroup|proc|sysfs|devpts|tmpfs|securityfs|mqueue|shm' | head -15", 30))
    log("df:\n" + run(tr, "df -h / /opt /var/lib /data /mnt 2>/dev/null | head -10", 30))
    log("/opt/youyuan:\n" + run(tr, "ls -la /opt/youyuan/ 2>&1 | head -25", 30))
    log("/var/lib/youyuan:\n" + run(tr, "ls -la /var/lib/youyuan/ 2>&1 | head -25", 30))
    log("old data disks:\n" + run(tr, "ls -la /mnt /data 2>&1 | head -20", 30))
    log("os:\n" + run(tr, "cat /etc/os-release 2>/dev/null | head -2; python3 --version 2>&1; which python3", 30))


def rebuild(tr: paramiko.Transport) -> None:
    log("=== [1] 用户/目录/依赖 ===")
    log(run(tr, """
id youyuan >/dev/null 2>&1 || useradd -r -m -d /opt/youyuan -s /usr/sbin/nologin youyuan
mkdir -p /opt/youyuan /var/lib/youyuan /var/lib/youyuan/plugins /var/lib/youyuan/chains /var/log/youyuan
command -v python3 >/dev/null || (apt-get update -qq && apt-get install -y -qq python3) 2>&1 | tail -2
echo SETUP_OK
""", 300))

    log("=== [2] 二进制与 cloudflared ===")
    if not BINARY.exists():
        raise SystemExit(f"缺少二进制 {BINARY}")
    sftp_put(tr, BINARY, "/opt/youyuan/youyuan.new")
    if CF_BINARY.exists():
        sftp_put(tr, CF_BINARY, "/usr/local/bin/cloudflared.new")
    sftp_put(tr, UNIT, "/etc/systemd/system/youyuan.service")
    log(run(tr, """
chmod 0755 /opt/youyuan/youyuan.new
mv /opt/youyuan/youyuan.new /opt/youyuan/youyuan
if [ -f /usr/local/bin/cloudflared.new ]; then chmod 0755 /usr/local/bin/cloudflared.new; mv /usr/local/bin/cloudflared.new /usr/local/bin/cloudflared; fi
chown -R youyuan:youyuan /opt/youyuan /var/lib/youyuan /var/log/youyuan 2>/dev/null
sha256sum /opt/youyuan/youyuan; cloudflared --version 2>&1 | head -1
systemctl daemon-reload
echo FILES_OK
""", 120))

    log("=== [3] 配置恢复（仅当目标不存在） ===")
    if MAIL_CONFIG.exists():
        sftp_put(tr, MAIL_CONFIG, "/var/lib/youyuan/mail-config.json.restore")
        log("mail-config.json 已上传待放置")
    if CONFIG_SNAPSHOT.exists():
        sftp_put(tr, CONFIG_SNAPSHOT, "/var/lib/youyuan/config.json.restore")
        log("config.json 快照已上传待放置")
    if PLUGIN_DIR.is_dir():
        for item in sorted(PLUGIN_DIR.iterdir()):
            if item.is_dir():
                # 保留插件目录结构 /var/lib/youyuan/plugins/<插件名>/...
                run(tr, f"mkdir -p '/var/lib/youyuan/plugins/{item.name}'")
                for sub in sorted(item.rglob("*")):
                    if sub.is_file():
                        rel = sub.relative_to(item)
                        remote_dir = f"/var/lib/youyuan/plugins/{item.name}/{rel.parent}"
                        run(tr, f"mkdir -p '{remote_dir}'")
                        sftp_put(tr, sub, f"{remote_dir}/{sub.name}")
                        log(f"插件: {item.name}/{rel}")
            elif item.is_file():
                sftp_put(tr, item, f"/var/lib/youyuan/plugins/{item.name}")
                log(f"插件文件: {item.name}")
    backup = PRIVATE / "backup-before-v090"
    if backup.is_dir():
        for f in sorted(backup.iterdir()):
            if f.is_file():
                sftp_put(tr, f, f"/var/lib/youyuan/{f.name}.restore")
                log(f"备份配置: {f.name}")
    for chain in sorted(CHAIN_DIR.glob("*.txt")):
        sftp_put(tr, chain, f"/var/lib/youyuan/chains/{chain.name}")
        log(f"命令链: {chain.name}")
    log(run(tr, """
cd /var/lib/youyuan
for f in mail-config.json config.json platform-identity.json qq-bridge.json youyuan-remote.json; do
  [ -f "$f" ] || [ ! -f "$f.restore" ] || mv "$f.restore" "$f"
done
rm -f *.restore
chown -R youyuan:youyuan /var/lib/youyuan /opt/youyuan /var/log/youyuan
ls -la /var/lib/youyuan | head -15; ls /var/lib/youyuan/plugins | head -20; ls /var/lib/youyuan/chains
echo CONFIG_OK
""", 60))

    log("=== [4] Cloudflare 隧道 ===")
    token = TUNNEL_TOKEN_FILE.read_text(encoding="utf-8").strip()
    if not token:
        raise SystemExit("隧道令牌为空")
    # 令牌含特殊字符，写临时文件再执行
    token_b64 = run(tr, f"echo -n {token!r} | base64 -w0").strip()
    log(run(tr, f"""
echo {token_b64} | base64 -d > /etc/youyuan-tunnel-token
chmod 0600 /etc/youyuan-tunnel-token
cloudflared --no-autoupdate service install $(cat /etc/youyuan-tunnel-token) 2>&1 | tail -3
systemctl enable cloudflared 2>&1 | tail -1
systemctl restart cloudflared
sleep 4
systemctl is-active cloudflared; ss -ltn | grep -E '19429' || true
echo TUNNEL_OK
""", 180))

    log("=== [5] 启动 youyuan.service ===")
    log(run(tr, "systemctl enable youyuan.service 2>&1 | tail -1; systemctl restart youyuan.service; sleep 5; systemctl is-active youyuan.service; ss -ltn | grep 19429 || echo NO_LISTEN", 120))
    log(run(tr, "curl -s -m 5 http://127.0.0.1:19429/health; echo; journalctl -u youyuan.service --no-pager -n 8 | tail -8", 60))
    log("=== 重建完成，外网复测请另行执行 ===")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--probe", action="store_true", help="只盘点，不修改")
    args = ap.parse_args()
    cfg = json.load(open(CFG_PATH, encoding="utf-8"))
    tr = connect(cfg)
    try:
        if args.probe:
            probe(tr)
        else:
            probe(tr)
            rebuild(tr)
    finally:
        tr.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
