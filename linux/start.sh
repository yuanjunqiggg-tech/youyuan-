#!/bin/sh
set -eu
task_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export YY_DATA_DIR="${YY_DATA_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/youyuan}"
export YY_PORT="${YY_PORT:-19429}"
export YY_TIMEZONE="${YY_TIMEZONE:-Asia/Shanghai}"
mkdir -p "$YY_DATA_DIR"
chmod 700 "$YY_DATA_DIR"
exec "$task_dir/youyuan"
