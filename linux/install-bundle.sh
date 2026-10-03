#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Run as root.' >&2; exit 1; }
task_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$task_dir"
command -v python3 >/dev/null || { echo 'Python 3 is required.' >&2; exit 1; }
command -v systemctl >/dev/null || { echo 'systemd is required.' >&2; exit 1; }
case "$(uname -m)" in
  x86_64) arch=x86_64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo 'Unsupported architecture.' >&2; exit 1 ;;
esac
sha256sum -c checksums.txt
# Validate the existing listener before modifying any installation.
python3 - <<'PY'
import socket,json,urllib.request
with socket.socket() as sock:
 sock.settimeout(2)
 occupied=sock.connect_ex(('127.0.0.1',19429))==0
if occupied:
 try:
  with urllib.request.build_opener(urllib.request.ProxyHandler({})).open('http://127.0.0.1:19429/health',timeout=3) as r:data=json.load(r)
  assert data.get('ready') and isinstance(data.get('version'),str)
 except Exception:raise SystemExit('Port 19429 belongs to an unidentified service; installation stopped.')
PY
if systemctl cat youyuan.service >/dev/null 2>&1; then
  systemctl show -p ExecStart --value youyuan.service | grep -F '/opt/youyuan/youyuan' >/dev/null || { echo 'Existing service uses another path; stopped.' >&2; exit 1; }
fi
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_dir="/var/backups/youyuan/$stamp"
install -d -m 700 "$backup_dir"
systemctl stop youyuan.service 2>/dev/null || true
if [ -d /opt/youyuan ]; then tar -C /opt -czf "$backup_dir/program.tar.gz" youyuan; fi
if [ -d /var/lib/youyuan ]; then tar -C /var/lib -czf "$backup_dir/data.tar.gz" youyuan; fi
id youyuan >/dev/null 2>&1 || useradd --system --home-dir /var/lib/youyuan --create-home --shell /usr/sbin/nologin youyuan
install -d -m 755 /opt/youyuan /opt/youyuan/packages
install -d -m 700 /var/lib/youyuan
chown -R youyuan:youyuan /var/lib/youyuan
tar -xzf "youyuan-v0.10.0-linux-$arch.tar.gz"
install -m 755 youyuan-0.10.0/youyuan /opt/youyuan/youyuan
install -m 755 youyuan-0.10.0/cloudflared /opt/youyuan/cloudflared
install -m 644 plugin.zip /opt/youyuan/packages/ds_ai_agent.zip
install -m 644 youyuan-0.10.0/youyuan.service /etc/systemd/system/youyuan.service
systemctl daemon-reload
systemctl enable --now youyuan.service
python3 validate_plugin_install.py --plugin /opt/youyuan/packages/ds_ai_agent.zip --report /var/lib/youyuan/deployment-check.json
chown youyuan:youyuan /var/lib/youyuan/deployment-check.json
chmod 600 /var/lib/youyuan/deployment-check.json
systemctl status youyuan.service --no-pager
printf 'Installed. Existing files backed up in %s. Game login and live tests remain separate.\n' "$backup_dir"
