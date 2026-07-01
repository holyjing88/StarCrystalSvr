#!/usr/bin/env bash
set -uo pipefail
export LANG=C LC_ALL=C
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPORT=()
FAIL=0
record() {
  REPORT+=("[$1] $2 — $3")
  [[ "$1" == FAIL ]] && FAIL=$((FAIL+1))
  return 0
}
echo "=== dbscripts backup test ==="
if bash -n "$SCRIPT_DIR/mysql/mysql-backup.sh" && bash -n "$SCRIPT_DIR/redis/redis-backup.sh"; then
  record PASS SYNTAX "bash -n ok"
else
  record FAIL SYNTAX "bash -n failed"
fi
if bash "$SCRIPT_DIR/startdb.sh"; then record PASS STARTDB "ok"; else record FAIL STARTDB "failed"; fi
MYSQL_BACKUP_DIR="${MYSQL_BACKUP_DIR:-$SCRIPT_DIR/data/mysql_backup_test}"
export MYSQL_BACKUP_DIR BACKUP_ROOT="$MYSQL_BACKUP_DIR"
export MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-$MYSQL_ROOT_PASSWORD}" MYSQL_USER="${MYSQL_USER:-root}" KEEP=3
if bash "$SCRIPT_DIR/mysql/mysql-backup.sh"; then
  latest="$(ls -1t "$MYSQL_BACKUP_DIR"/mysql-dump-*.sql.gz 2>/dev/null | head -1 || true)"
  if [[ -n "$latest" && -s "$latest" ]]; then
    record PASS MYSQL-BACKUP "$latest"
  else
    record FAIL MYSQL-BACKUP "no file"
  fi
else record FAIL MYSQL-BACKUP "script failed"; fi
REDIS_BACKUP_DIR="${REDIS_BACKUP_DIR:-$SCRIPT_DIR/redis/data_backup_test}"
export REDIS_BACKUP_DIR BACKUP_ROOT="$REDIS_BACKUP_DIR"
if bash "$SCRIPT_DIR/redis/redis-backup.sh"; then
  latest_r="$(ls -1t "$REDIS_BACKUP_DIR"/redis-dump-*.rdb 2>/dev/null | head -1 || true)"
  if [[ -n "$latest_r" && -s "$latest_r" ]]; then
    record PASS REDIS-BACKUP "$latest_r"
  else
    record FAIL REDIS-BACKUP "no file"
  fi
else record FAIL REDIS-BACKUP "script failed"; fi
if command -v crontab >/dev/null 2>&1; then
  CRON_LINE="59 23 * * * MYSQL_BACKUP_DIR=$MYSQL_BACKUP_DIR MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD $SCRIPT_DIR/mysql/mysql-backup.sh" bash "$SCRIPT_DIR/mysql/mysql-backup-register-cron.sh"
  CRON_LINE="58 23 * * * REDIS_BACKUP_DIR=$REDIS_BACKUP_DIR $SCRIPT_DIR/redis/redis-backup.sh" bash "$SCRIPT_DIR/redis/redis-backup-register-cron.sh"
  if crontab -l 2>/dev/null | grep -qF "$SCRIPT_DIR/mysql/mysql-backup.sh" && crontab -l 2>/dev/null | grep -qF "$SCRIPT_DIR/redis/redis-backup.sh"; then
    record PASS CRON-REGISTER "ok"
  else record FAIL CRON-REGISTER "missing lines"; fi
  bash "$SCRIPT_DIR/mysql/mysql-backup-unregister-cron.sh"
  bash "$SCRIPT_DIR/redis/redis-backup-unregister-cron.sh"
  if crontab -l 2>/dev/null | grep -qF "$SCRIPT_DIR/mysql/mysql-backup.sh" || crontab -l 2>/dev/null | grep -qF "$SCRIPT_DIR/redis/redis-backup.sh"; then
    record FAIL CRON-UNREGISTER "still present"
  else record PASS CRON-UNREGISTER "ok"; fi
  bash "$SCRIPT_DIR/mysql/mysql-backup-register-cron.sh" || true
  bash "$SCRIPT_DIR/redis/redis-backup-register-cron.sh" || true
  record PASS CRON-RESTORE "default lines restored"
else record SKIP CRON "no crontab"; fi
echo ""
for line in "${REPORT[@]}"; do echo "$line"; done
echo ""
echo "FAIL=$FAIL"
[[ "$FAIL" -eq 0 ]]
