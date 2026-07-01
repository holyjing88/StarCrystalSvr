#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"
# shellcheck source=redis-common.sh
source "$HERE/redis-common.sh"

CONF="$(sc_redis_conf_default)"
PORT="$(sc_redis_port "$CONF")"
KEEP="${KEEP:-30}"
BACKUP_ROOT="${BACKUP_ROOT:-$(sc_redis_default_backup_dir)}"
USE_BG="${USE_BGSAVE:-0}"
REDIS_CLI="$(sc_redis_cli)"

sc_redis_ensure_dirs
mkdir -p "$BACKUP_ROOT"

if [[ "$USE_BG" == "1" ]]; then
  echo "[redis-backup] BGSAVE"
  t0="$("$REDIS_CLI" -p "$PORT" LASTSAVE 2>/dev/null | tr -d '\r' || echo 0)"
  "$REDIS_CLI" -p "$PORT" BGSAVE >/dev/null
  start_ts=$(date +%s)
  while true; do
    t1="$("$REDIS_CLI" -p "$PORT" LASTSAVE 2>/dev/null | tr -d '\r' || echo 0)"
    if [[ "$t1" != "$t0" ]]; then break; fi
    now_ts=$(date +%s)
    if (( now_ts - start_ts > 900 )); then echo "[redis-backup] BGSAVE 等待 LASTSAVE 超时（900s）" >&2; exit 1; fi
    sleep 1
  done
else
  echo "[redis-backup] SAVE"
  "$REDIS_CLI" -p "$PORT" SAVE >/dev/null
fi

if ! data_dir="$(sc_redis_resolve_data_dir "$CONF")"; then
  echo "[redis-backup] 无法确定数据目录。请设置 REDIS_DIR / REDIS_DATA_DIR，或保证 $CONF 中 dir 与 dump.rdb 可访问。" >&2
  exit 1
fi

DUMP="$data_dir/dump.rdb"
if [[ ! -f "$DUMP" ]]; then
  echo "[redis-backup] 找不到 $DUMP" >&2
  exit 1
fi

TS="$(date +%Y%m%d-%H%M%S)"
cp -f "$DUMP" "$BACKUP_ROOT/redis-dump-$TS.rdb"
echo "[redis-backup] 已复制 $BACKUP_ROOT/redis-dump-$TS.rdb"

if [[ -f "$data_dir/appendonly.aof" ]]; then
  cp -f "$data_dir/appendonly.aof" "$BACKUP_ROOT/redis-appendonly-$TS.aof"
  echo "[redis-backup] 已复制 appendonly"
fi

# 仅清理 redis-dump-*.rdb，与 Windows 脚本一致（不用 xargs -r，兼容 Alpine/busybox）
while IFS= read -r old; do
  [[ -n "$old" ]] && rm -f "$old"
done < <(ls -1t "$BACKUP_ROOT"/redis-dump-*.rdb 2>/dev/null | tail -n +"$((KEEP + 1))" || true)
echo "[redis-backup] 完成（保留最近 $KEEP 个 dump 备份）。"
