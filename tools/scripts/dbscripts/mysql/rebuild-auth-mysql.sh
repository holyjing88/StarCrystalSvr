#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../dbscripts-config.sh
source "$SCRIPT_DIR/../dbscripts-config.sh"

SqlHost="${MYSQL_HOST:-127.0.0.1}"
Port="${MYSQL_PORT:-3306}"
Database="${MYSQL_AUTH_DB:-starcrystal_auth}"
User="${MYSQL_AUTH_USER:-star_auth}"
Password="${MYSQL_AUTH_PASSWORD:-jgyjgyjgy}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -H|--host) SqlHost="$2"; shift 2 ;;
    -p|--port) Port="$2"; shift 2 ;;
    *) echo "Usage: $0 [-H host] [-p port]" >&2; exit 1 ;;
  esac
done

sqlFile="$SQL_DIR/starcrystal_auth_mysql.sql"
[[ -f "$sqlFile" ]] || { echo "SQL not found: $sqlFile" >&2; exit 1; }

mysql_bin=""
if [[ -n "${MYSQL_CLIENT:-}" && -x "${MYSQL_CLIENT}" ]]; then
  mysql_bin="$MYSQL_CLIENT"
elif command -v mysql >/dev/null 2>&1; then
  mysql_bin="$(command -v mysql)"
else
  portable="$(sc_default_mysql_base)/bin/mysql"
  [[ -x "$portable" ]] && mysql_bin="$portable"
fi
[[ -n "$mysql_bin" ]] || { echo "mysql client not found" >&2; exit 1; }

echo "Rebuilding auth tables in ${Database}@${SqlHost}:${Port} ..."
"$mysql_bin" --protocol=TCP --host="$SqlHost" --port="$Port" --user="$User" --password="$Password" "$Database" <<EOF
SOURCE $sqlFile;
SHOW TABLES LIKE 'auth_%';
SELECT COUNT(*) AS auth_accounts_rows FROM auth_accounts;
EOF
echo "Done."
