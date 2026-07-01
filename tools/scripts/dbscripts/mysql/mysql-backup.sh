#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=mysql-common.sh
source "$HERE/mysql-common.sh"
sc_mysql_ensure_dirs
sc_mysql_backup_defaults
mkdir -p "$BACKUP_ROOT"

if ! command -v mysqldump >/dev/null 2>&1; then
  echo "[mysql-backup] 找不到 mysqldump" >&2
  exit 1
fi

TS="$(date +%Y%m%d-%H%M%S)"
OUT="$BACKUP_ROOT/mysql-dump-${TS}.sql.gz"
echo "[mysql-backup] dumping $MYSQL_DATABASE -> $OUT"

dump_args=(-h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER")
if [[ -n "$MYSQL_UNIX_SOCKET" ]]; then
  dump_args=(--socket="$MYSQL_UNIX_SOCKET" -u"$MYSQL_USER")
fi

MYSQL_PWD="$MYSQL_PASSWORD" mysqldump \
  "${dump_args[@]}" \
  --single-transaction --routines --triggers --events \
  --databases "$MYSQL_DATABASE" \
  | gzip -c >"$OUT"

echo "[mysql-backup] 已写入 $OUT"

while IFS= read -r old; do
  [[ -n "$old" ]] && rm -f "$old"
done < <(ls -1t "$BACKUP_ROOT"/mysql-dump-*.sql.gz 2>/dev/null | tail -n +"$((KEEP + 1))" || true)

echo "[mysql-backup] 完成（保留最近 $KEEP 个备份）。"
