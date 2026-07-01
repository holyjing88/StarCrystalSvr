#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=redis-common.sh
source "$HERE/redis-common.sh"
if ! command -v crontab >/dev/null 2>&1; then
  exit 0
fi

TMP="$(mktemp)"
crontab -l 2>/dev/null | grep -vF "$HERE/redis-backup.sh" >"$TMP" || true
crontab "$TMP"
rm -f "$TMP"
echo "[unregister-cron] 已移除包含 $HERE/redis-backup.sh 的 crontab 行。"
