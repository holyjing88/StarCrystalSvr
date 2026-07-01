#!/usr/bin/env bash
# 注册 crontab 行，默认每天 03:15 执行本目录 mysql-backup.sh。
# 自定义: CRON_LINE='0 4 * * * MYSQL_PORT=3306 /path/to/mysql-backup.sh'
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=mysql-common.sh
source "$HERE/mysql-common.sh"

if ! command -v crontab >/dev/null 2>&1; then
  echo "[mysql-register-cron] 未找到 crontab，跳过。" >&2
  exit 0
fi

CRON_LINE="${CRON_LINE:-15 3 * * * $HERE/mysql-backup.sh}"
TMP="$(mktemp)"
crontab -l 2>/dev/null | grep -vF "$HERE/mysql-backup.sh" >"$TMP" || true
echo "$CRON_LINE" >>"$TMP"
crontab "$TMP"
rm -f "$TMP"
echo "[mysql-register-cron] 已写入 crontab: $CRON_LINE"
