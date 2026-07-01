#!/usr/bin/env bash
# dbscripts 本地配置（仅引用 dbscripts/ 目录内资源，不依赖 repo/release/tools 外部路径）。
export LANG=C
export LC_ALL=C

DBSCRIPTS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SQL_DIR="$DBSCRIPTS_ROOT/sql"

sc_config_json() {
  echo "$DBSCRIPTS_ROOT/config/starcrystal.json"
}

# shellcheck source=mysql/mysql-dsn.sh
source "$DBSCRIPTS_ROOT/mysql/mysql-dsn.sh"

sc_load_local_env() {
  local f="$DBSCRIPTS_ROOT/local.env"
  [[ -f "$f" ]] || return 0
  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" ]] && continue
    [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    [[ -n "${!key+x}" ]] && continue
    val="${BASH_REMATCH[2]}"
    val="${val%\"}"; val="${val#\"}"
    val="${val%\'}"; val="${val#\'}"
    export "$key=$val"
  done <"$f"
}

sc_load_local_env

sc_default_mysql_base() {
  if [[ -n "${MYSQL_PORTABLE_BASE:-}" ]]; then
    echo "$MYSQL_PORTABLE_BASE"
    return
  fi
  local candidate="$DBSCRIPTS_ROOT/mysql/portable"
  if [[ -d "$candidate" ]]; then
    echo "$candidate"
    return
  fi
  echo "$candidate"
}

sc_default_mysql_data() {
  if [[ -n "${MYSQL_PORTABLE_DATA:-}" ]]; then
    echo "$MYSQL_PORTABLE_DATA"
    return
  fi
  if [[ -n "${MYSQL_DATA_DIR:-}" ]]; then
    echo "$MYSQL_DATA_DIR"
    return
  fi
  echo "$DBSCRIPTS_ROOT/data/mysql"
}

sc_portable_mysqld() {
  local base="${MYSQL_PORTABLE_BASE:-$(sc_default_mysql_base)}"
  echo "$base/bin/mysqld"
}

sc_portable_mysqld_runnable() {
  local mysqld out rc=0
  mysqld="$(sc_portable_mysqld)"
  [[ -x "$mysqld" ]] || return 1
  out="$("$mysqld" --version 2>&1)" || rc=$?
  if echo "$out" | grep -qiE 'GLIBC_|CXXABI_|GLIBCXX_|not found'; then
    return 1
  fi
  [[ "$rc" -eq 0 && -n "$out" ]]
}

sc_mysql_system_unit() {
  if command -v systemctl >/dev/null 2>&1; then
    if systemctl list-unit-files mysqld.service 2>/dev/null | awk '{print $1}' | grep -qx mysqld.service; then
      echo mysqld
      return 0
    fi
    if systemctl list-unit-files mysql.service 2>/dev/null | awk '{print $1}' | grep -qx mysql.service; then
      echo mysql
      return 0
    fi
  fi
  [[ -f /usr/lib/systemd/system/mysqld.service || -f /etc/systemd/system/mysqld.service ]] && { echo mysqld; return 0; }
  [[ -f /usr/lib/systemd/system/mysql.service || -f /etc/systemd/system/mysql.service ]] && { echo mysql; return 0; }
  if command -v mysqld >/dev/null 2>&1 || command -v mysqladmin >/dev/null 2>&1; then
    echo mysqld
    return 0
  fi
  return 1
}

sc_mysql_mode() {
  case "${MYSQL_MODE:-auto}" in
    system|native|host) echo system ;;
    portable) echo portable ;;
    auto|*)
      if sc_mysql_system_unit >/dev/null 2>&1; then
        echo system
      elif sc_portable_mysqld_runnable; then
        echo portable
      elif command -v mysql >/dev/null 2>&1; then
        echo system
      else
        echo none
      fi
      ;;
  esac
}

sc_mysql_available() {
  [[ "$(sc_mysql_mode)" != none ]]
}

sc_mysql_tcp_open() {
  local host="${1:-127.0.0.1}" port="${2:-3306}"
  # Prefer bash /dev/tcp — RHEL/CentOS ncat does not support nc -z.
  if command -v timeout >/dev/null 2>&1; then
    timeout 1 bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null && return 0
  else
    bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null && return 0
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -w 1 "$host" "$port" </dev/null &>/dev/null && return 0
  fi
  return 1
}

sc_mysql_ping() {
  sc_mysql_ping_creds
  local host="${MYSQL_HOST:-127.0.0.1}" port="${MYSQL_PORT:-3306}"
  local user="${MYSQL_PING_USER:-root}" pw="${MYSQL_PING_PASSWORD:-}"
  if command -v mysqladmin >/dev/null 2>&1; then
    if [[ -n "$pw" ]]; then
      mysqladmin --protocol=TCP -h"$host" -P"$port" -u"$user" --password="$pw" ping 2>/dev/null | grep -qi alive && return 0
    fi
    mysqladmin --protocol=TCP -h"$host" -P"$port" -u"$user" ping 2>/dev/null | grep -qi alive && return 0
  fi
  if command -v mysql >/dev/null 2>&1; then
    if [[ -n "$pw" ]]; then
      mysql --protocol=TCP -h"$host" -P"$port" -u"$user" --password="$pw" -e "SELECT 1" >/dev/null 2>&1 && return 0
    fi
    mysql --protocol=TCP -h"$host" -P"$port" -u"$user" -e "SELECT 1" >/dev/null 2>&1 && return 0
  fi
  sc_mysql_tcp_open "$host" "$port"
}

sc_mysql_wait_stopped() {
  local i
  for i in $(seq 1 30); do
    if ! sc_mysql_ping; then
      return 0
    fi
    sleep 1
  done
  return 1
}

sc_last_auth_dsn_file() {
  echo "$DBSCRIPTS_ROOT/log/last-auth-mysql-dsn.txt"
}

sc_save_last_auth_dsn() {
  local dsn="${1:-}"
  [[ -z "$dsn" ]] && return 0
  mkdir -p "$DBSCRIPTS_ROOT/log"
  printf '%s\n' "$dsn" >"$(sc_last_auth_dsn_file)"
}

sc_dbscripts_root() {
  echo "$DBSCRIPTS_ROOT"
}
