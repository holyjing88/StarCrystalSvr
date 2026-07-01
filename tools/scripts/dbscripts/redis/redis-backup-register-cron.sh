#!/usr/bin/env bash
# 注册 crontab 行，默认每天 03:00 执行本目录 redis-backup.sh。
# 自定义示例（与 Windows 计划任务等价的环境变量，按需改 CRON_LINE 后执行本脚本）:
#   CRON_LINE='15 3 * * * PORT=6380 REDIS_CLI=/usr/bin/redis-cli REDIS_CONF=/path/to/redis.conf /path/to/server-go/scripts/redis/redis-backup.sh'
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=redis-common.sh
source "$HERE/redis-common.sh"

if ! command -v crontab >/dev/null 2>&1; then
  echo "[redis-register-cron] 未找到 crontab，跳过。" >&2
  exit 0
fi

CRON_LINE="${CRON_LINE:-0 3 * * * $HERE/redis-backup.sh}"
TMP="$(mktemp)"
crontab -l 2>/dev/null | grep -vF "$HERE/redis-backup.sh" >"$TMP" || true
echo "$CRON_LINE" >>"$TMP"
crontab "$TMP"
rm -f "$TMP"
echo "[register-cron] 已写入 crontab 行: $CRON_LINE"
