#!/usr/bin/env bash
# 从 dbscripts/config/starcrystal.json 的 authMysqlDsn 解析 MySQL 账号（dbscripts 内部共用）。

sc_auth_mysql_json_path() {
  if declare -f sc_config_json >/dev/null 2>&1; then
    local p
    p="$(sc_config_json)"
    [[ -f "$p" ]] && { echo "$p"; return 0; }
  fi
  if [[ -n "${DBSCRIPTS_ROOT:-}" && -f "$DBSCRIPTS_ROOT/config/starcrystal.json" ]]; then
    echo "$DBSCRIPTS_ROOT/config/starcrystal.json"
    return 0
  fi
  return 1
}

sc_parse_auth_mysql_dsn() {
  local dsn="$1"
  [[ -n "$dsn" ]] || return 1
  if [[ "$dsn" =~ ^([^:]+):([^@]+)@tcp\(([^:]+):([0-9]+)\)/([^?]+) ]]; then
    MYSQL_AUTH_USER="${BASH_REMATCH[1]}"
    MYSQL_AUTH_PASSWORD="${BASH_REMATCH[2]}"
    MYSQL_HOST="${BASH_REMATCH[3]}"
    MYSQL_PORT="${BASH_REMATCH[4]}"
    MYSQL_AUTH_DB="${BASH_REMATCH[5]}"
    export MYSQL_AUTH_USER MYSQL_AUTH_PASSWORD MYSQL_HOST MYSQL_PORT MYSQL_AUTH_DB
    return 0
  fi
  return 1
}

sc_load_auth_mysql_dsn_from_json() {
  local json="${1:-}"
  [[ -n "$json" ]] || json="$(sc_auth_mysql_json_path 2>/dev/null || true)"
  [[ -f "${json:-}" ]] || return 1

  local dsn=""
  if command -v python3 >/dev/null 2>&1; then
    dsn="$(python3 - "$json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print((json.load(f).get("authMysqlDsn") or "").strip())
PY
)"
  else
    dsn="$(grep -E '"authMysqlDsn"' "$json" 2>/dev/null | head -1 | sed -E 's/.*"authMysqlDsn"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/')"
  fi
  [[ -n "$dsn" ]] || return 1
  sc_parse_auth_mysql_dsn "$dsn"
}

sc_apply_auth_mysql_creds() {
  [[ -n "${MYSQL_AUTH_USER:-}" && -n "${MYSQL_AUTH_PASSWORD:-}" ]] && return 0
  sc_load_auth_mysql_dsn_from_json || return 0
}

sc_auth_mysql_dsn_string() {
  local port="${1:-${MYSQL_PORT:-3306}}"
  sc_apply_auth_mysql_creds
  local user="${MYSQL_AUTH_USER:-}"
  local pass="${MYSQL_AUTH_PASSWORD:-}"
  local host="${MYSQL_HOST:-127.0.0.1}"
  local db="${MYSQL_AUTH_DB:-starcrystal_auth}"
  [[ -n "$user" && -n "$pass" ]] || return 1
  echo "${user}:${pass}@tcp(${host}:${port})/${db}?charset=utf8mb4&parseTime=true&loc=Local"
}

sc_mysql_ping_creds() {
  sc_apply_auth_mysql_creds
  local user="${MYSQL_AUTH_USER:-}"
  local pw="${MYSQL_AUTH_PASSWORD:-}"
  if [[ -z "$user" || -z "$pw" ]]; then
    user="${MYSQL_ROOT_USER:-root}"
    pw="${MYSQL_ROOT_PASSWORD:-}"
  fi
  if [[ "$user" == "root" && -z "$pw" ]]; then
    pw="${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}"
  fi
  MYSQL_PING_USER="$user"
  MYSQL_PING_PASSWORD="$pw"
  export MYSQL_PING_USER MYSQL_PING_PASSWORD
}

sc_sync_root_creds_from_auth_dsn() {
  sc_apply_auth_mysql_creds
  [[ "${MYSQL_AUTH_USER:-}" == "root" && -n "${MYSQL_AUTH_PASSWORD:-}" ]] || return 0
  [[ -n "${MYSQL_ROOT_PASSWORD:-}" ]] && return 0
  export MYSQL_ROOT_USER="${MYSQL_ROOT_USER:-root}"
  export MYSQL_ROOT_PASSWORD="${MYSQL_AUTH_PASSWORD}"
}
