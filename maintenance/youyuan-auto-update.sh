#!/bin/bash
# 游园 Linux 生产机自动更新：拉取 latest.json，比对版本后分片下载、校验、原子替换、重启。
# 由 youyuan-update.timer 每 6 小时触发；仅当 version_code 大于本机记录时才更新。
set -u
BASE="https://updates.youyuanqi.dpdns.org"
STATE="/var/lib/youyuan-update/installed-code"
TARGET="/opt/youyuan/youyuan"
LOG_TAG="youyuan-update"
UA="Mozilla/5.0 (YouYuan-AutoUpdate)"
# updates 域对裸 urllib 403，必须带浏览器样 UA
fetch() { curl -fsSL -A "$UA" --max-time 120 "$1"; }

manifest=$(fetch "$BASE/updates/android/latest.json") || { logger -t "$LOG_TAG" "拉取 latest.json 失败"; exit 1 }
code=$(printf '%s' "$manifest" | sed -n 's/.*"version_code":\([0-9]*\).*/\1/p' | head -1)
version=$(printf '%s' "$manifest" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p' | head -1)
sha=$(printf '%s' "$manifest" | sed -n 's/.*"sha256":"\([^"]*\)".*/\1/p' | head -1)
[ -z "$code" ] || [ -z "$sha" ] && { logger -t "$LOG_TAG" "清单解析失败"; exit 1 }

installed=$(cat "$STATE" 2>/dev/null || echo 0)
if [ "$code" -le "$installed" ]; then exit 0; fi
logger -t "$LOG_TAG" "发现新版本 $version (code $code)，开始更新"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
> "$work/binary"
# 按 parts 顺序拉分片拼装（Worker 按 Range 回源动态切）
for p in $(printf '%s' "$manifest" | grep -o '"path":"[^"]*"' | sed 's/"path":"//;s/"//'); do
  fetch "$BASE$p" >> "$work/binary" || { logger -t "$LOG_TAG" "分片拉取失败 $p"; exit 1 }
done

actual=$(sha256sum "$work/binary" | cut -d' ' -f1)
if [ "$actual" != "$sha" ]; then logger -t "$LOG_TAG" "sha256 不一致：$actual != $sha"; exit 1 fi

install -m 755 "$work/binary" "$TARGET.new" || exit 1
systemctl stop youyuan.service
mv "$TARGET.new" "$TARGET"
systemctl start youyuan.service
mkdir -p "$(dirname "$STATE")"; echo "$code" > "$STATE"
logger -t "$LOG_TAG" "已更新到 $version (code $code) 并重启服务"
