#!/usr/bin/env bash
# shellcheck disable=SC2034
# Source from tools/scripts/*.sh — sets RELEASE_ROOT (server-go/release), REPO_ROOT (server-go).
export LANG="${LANG:-C.UTF-8}"
export LC_ALL="${LC_ALL:-$LANG}"

_SC_SCRIPTS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

if [[ -f "$_SC_SCRIPTS_ROOT/../../release/configs/starcrystal.json" ]]; then
  RELEASE_ROOT="$(cd "$_SC_SCRIPTS_ROOT/../../release" && pwd)"
  REPO_ROOT="$(cd "$_SC_SCRIPTS_ROOT/../.." && pwd)"
elif [[ -f "$_SC_SCRIPTS_ROOT/../configs/starcrystal.json" ]]; then
  RELEASE_ROOT="$(cd "$_SC_SCRIPTS_ROOT/.." && pwd)"
  REPO_ROOT="$(cd "$RELEASE_ROOT/.." && pwd)"
else
  echo "starcrystal-config.sh: cannot locate release/configs/starcrystal.json" >&2
  exit 1
fi

sc_config_json() {
  echo "$RELEASE_ROOT/configs/starcrystal.json"
}

# shellcheck source=mysql/mysql-dsn.sh
source "$_SC_SCRIPTS_ROOT/dbscripts/mysql/mysql-dsn.sh"

sc_api_base_url() {
  local path
  path="$(sc_config_json)"
  if [[ ! -f "$path" ]]; then
    echo "http://0.0.0.0:8080"
    return
  fi
  python3 - "$path" <<'PY' 2>/dev/null || echo "http://0.0.0.0:8080"
import json, sys
p = sys.argv[1]
with open(p, encoding="utf-8") as f:
    j = json.load(f)
host = (j.get("apiListenHost") or "").strip() or "0.0.0.0"
port = int(j.get("apiListenPort") or 8080) or 8080
scheme = "https" if j.get("useHttps") else "http"
print(f"{scheme}://{host}:{port}")
PY
}

sc_default_mysql_base() {
  if [[ -n "${MYSQL_PORTABLE_BASE:-}" ]]; then
    echo "$MYSQL_PORTABLE_BASE"
    return
  fi
  local games_root
  games_root="$(cd "$REPO_ROOT/../.." && pwd)"
  case "$(uname -s 2>/dev/null || echo unknown)" in
    Linux)
      echo "$games_root/mysql-portable-linux/mysql-8.4.8-linux-glibc2.28-x86_64"
      ;;
    *)
      echo "$games_root/mysql-portable/mysql-8.4.8-winx64"
      ;;
  esac
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
  echo "/app/mysql/db"
}

sc_load_linux_host_env() {
  local f="$_SC_SCRIPTS_ROOT/../linux-host.local.env"
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

sc_load_linux_host_env

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

# MYSQL_MODE=system|portable|auto（默认 auto：优先系统 MySQL）
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
  if command -v nc >/dev/null 2>&1; then
    nc -z "$host" "$port" 2>/dev/null
    return $?
  fi
  timeout 1 bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null
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
  echo "$RELEASE_ROOT/log/last-auth-mysql-dsn.txt"
}

sc_save_last_auth_dsn() {
  local dsn="${1:-}"
  [[ -z "$dsn" ]] && return 0
  mkdir -p "$RELEASE_ROOT/log"
  printf '%s\n' "$dsn" >"$(sc_last_auth_dsn_file)"
}

sc_load_smtp_local_env() {
  local f="$RELEASE_ROOT/configs/smtp.local.env"
  [[ -f "$f" ]] || return 0
  set -a
  # shellcheck disable=SC1090
  source "$f"
  set +a
}

TOOLS_ROOT="$(cd "$_SC_SCRIPTS_ROOT/.." && pwd)"
OFFLINE_SOFTS_ROOT="${OFFLINE_SOFTS_ROOT:-$TOOLS_ROOT/offlinesofts}"
DOCKER_ROOT="${DOCKER_ROOT:-$TOOLS_ROOT/docker}"

sc_tools_scripts_root() {
  echo "$_SC_SCRIPTS_ROOT"
}

sc_offline_softs_root() {
  echo "$OFFLINE_SOFTS_ROOT"
}

sc_setup_go_env() {
  local go_root="${STARCRYSTAL_GO_ROOT:-$REPO_ROOT/.go-toolchain/go}"
  if [[ -x "$go_root/bin/go" ]]; then
    export PATH="$go_root/bin:$PATH"
  fi
  if [[ -d "${GOMODCACHE:-$REPO_ROOT/.gomodcache}" ]]; then
    export GOMODCACHE="${GOMODCACHE:-$REPO_ROOT/.gomodcache}"
    if [[ -z "${GOPROXY:-}" ]]; then
      export GOPROXY=off
    fi
  fi
}
