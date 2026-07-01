#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v crontab >/dev/null 2>&1; then
  exit 0
fi

TMP="$(mktemp)"
crontab -l 2>/dev/null | grep -vF "$HERE/mysql-backup.sh" >"$TMP" || true
crontab "$TMP"
rm -f "$TMP"
echo "[mysql-unregister-cron] 已移除 mysql-backup.sh 相关 crontab 行。"
