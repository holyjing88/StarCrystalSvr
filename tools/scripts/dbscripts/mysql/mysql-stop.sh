#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../dbscripts-config.sh
source "$SCRIPT_DIR/../dbscripts-config.sh"
HERE="$SCRIPT_DIR"
# shellcheck source=mysql-common.sh
source "$HERE/mysql-common.sh"

BaseDir="${MYSQL_PORTABLE_BASE:-$(sc_default_mysql_base)}"
Port="${MYSQL_PORT:-3306}"
Mode="$(sc_mysql_mode)"
MysqlRootParent="$(dirname "$BaseDir")"
MysqlPidPath="${MYSQL_PID_PATH:-$MysqlRootParent/mysqld-${Port}.pid}"
RootPassword="${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}"

sc_mysql_backup_cron_unregister

mysql_stop_ok() {
  echo "[mysql-stop] OK — MySQL 已停止 (mode=$Mode, port=$Port)"
  exit 0
}

mysql_stop_fail() {
  echo "[mysql-stop] FAIL — $*" >&2
  exit 1
}

stop_pid() {
  local pid="$1"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 10); do
      kill -0 "$pid" 2>/dev/null || return 0
      sleep 1
    done
    kill -9 "$pid" 2>/dev/null || true
  fi
}

if [[ "$Mode" == none ]]; then
  echo "[mysql-stop] OK — 无可用 MySQL，无需停止"
  exit 0
fi

if ! sc_mysql_ping; then
  mysql_stop_ok
fi

if [[ "$Mode" == system ]]; then
  unit="$(sc_mysql_system_unit)" || true
  echo "[mysql-stop] 停止系统 MySQL (${unit:-mysqld}, port=$Port) ..."
  if [[ -n "$unit" ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$unit" 2>/dev/null || true
  elif command -v mysqladmin >/dev/null 2>&1; then
    if [[ -n "$RootPassword" ]]; then
      mysqladmin -h127.0.0.1 -P"$Port" -uroot --password="$RootPassword" shutdown 2>/dev/null || true
    else
      mysqladmin -h127.0.0.1 -P"$Port" -uroot shutdown 2>/dev/null || true
    fi
  fi
  if sc_mysql_wait_stopped; then
    mysql_stop_ok
  fi
  mysql_stop_fail "系统 MySQL 仍在运行 (port=$Port)"
fi

if [[ -f "$MysqlPidPath" ]]; then
  pid="$(tr -d '\r\n' <"$MysqlPidPath")"
  if [[ -n "$pid" ]]; then
    echo "[mysql-stop] 停止便携 MySQL PID=$pid ..."
    stop_pid "$pid"
  fi
  rm -f "$MysqlPidPath"
fi

if command -v mysqladmin >/dev/null 2>&1; then
  mysqladmin -h127.0.0.1 -P"$Port" -uroot shutdown 2>/dev/null || true
elif [[ -x "$BaseDir/bin/mysqladmin" ]]; then
  "$BaseDir/bin/mysqladmin" -h127.0.0.1 -P"$Port" -uroot shutdown 2>/dev/null || true
fi

if sc_mysql_wait_stopped; then
  mysql_stop_ok
fi
mysql_stop_fail "便携 MySQL 仍在运行 (port=$Port)"
