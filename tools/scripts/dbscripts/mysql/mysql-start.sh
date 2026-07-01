#!/usr/bin/env bash
# Linux MySQL 启动：优先系统 mysqld（systemd），否则便携 mysqld。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../dbscripts-config.sh
source "$SCRIPT_DIR/../dbscripts-config.sh"
HERE="$SCRIPT_DIR"
# shellcheck source=mysql-common.sh
source "$HERE/mysql-common.sh"

sc_mysql_ensure_dirs

BaseDir="${MYSQL_PORTABLE_BASE:-$(sc_default_mysql_base)}"
DataDir="${MYSQL_PORTABLE_DATA:-$(sc_mysql_default_data_dir)}"
Port="${MYSQL_PORT:-3306}"
Mode="$(sc_mysql_mode)"

DefaultSqlHost='127.0.0.1'
DefaultDatabase='starcrystal_auth'
DefaultAppUser='star_auth'
DefaultAppPassword='jgyjgyjgy'

publish_auth_dsn() {
  local port_num="$1"
  local dsn
  dsn="${DefaultAppUser}:${DefaultAppPassword}@tcp(${DefaultSqlHost}:${port_num})/${DefaultDatabase}?charset=utf8mb4&parseTime=true&loc=Local"
  export AUTH_MYSQL_DSN="$dsn"
  sc_save_last_auth_dsn "$dsn"
  echo ""
  echo "AUTH_MYSQL_DSN=$dsn"
  echo "Schema: $(dirname "$0")/rebuild-auth-mysql.sh -H $DefaultSqlHost -p $port_num"
}

wait_mysql_up() {
  local i
  for i in $(seq 1 30); do
    if sc_mysql_ping; then
      return 0
    fi
    sleep 1
  done
  return 1
}

mysql_start_ok() {
  echo "[mysql-start] OK — MySQL 运行中 (mode=$Mode, port=$Port)"
  echo "[mysql-start] data=$(sc_mysql_default_data_dir) backup=$(sc_mysql_default_backup_dir)"
}

mysql_start_fail() {
  echo "[mysql-start] FAIL — $*" >&2
  exit 1
}

if [[ "$Mode" == none ]]; then
  mysql_start_fail "无可用 MySQL（请安装系统 MySQL 或设置 MYSQL_MODE=system|portable）"
fi

if sc_mysql_ping; then
  mysql_start_ok
  publish_auth_dsn "$Port"
  sc_mysql_backup_cron_register
  exit 0
fi

if [[ "$Mode" == system ]]; then
  unit="$(sc_mysql_system_unit)"
  echo "[mysql-start] 启动系统 MySQL (${unit}) ..."
  if [[ -n "$unit" ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl start "$unit"
  else
    mysql_start_fail "无法启动系统 MySQL（未找到 systemd unit）"
  fi
  if ! wait_mysql_up; then
    systemctl status "$unit" --no-pager 2>&1 | tail -20 || true
    mysql_start_fail "系统 MySQL 在 30s 内未就绪 (port=$Port)"
  fi
  mysql_start_ok
  publish_auth_dsn "$Port"
  sc_mysql_backup_cron_register
  exit 0
fi

mysqld="$(sc_portable_mysqld)"
MysqlRootParent="$(dirname "$BaseDir")"
logFile="$MysqlRootParent/mysql.log"
MysqlPidPath="$MysqlRootParent/mysqld-${Port}.pid"

mkdir -p "$DataDir"

if [[ ! -d "$DataDir/mysql" ]]; then
  echo "[mysql-start] 初始化便携数据目录 ..."
  "$mysqld" --initialize-insecure --basedir="$BaseDir" --datadir="$DataDir"
fi

echo "[mysql-start] 启动便携 MySQL (port=$Port) ..."
nohup "$mysqld" \
  --basedir="$BaseDir" \
  --datadir="$DataDir" \
  --port="$Port" \
  --bind-address=127.0.0.1 \
  --log-error="$logFile" \
  >>"$MysqlRootParent/mysql.stdout.log" 2>>"$MysqlRootParent/mysql.stderr.log" &
echo $! >"$MysqlPidPath"

if ! wait_mysql_up; then
  mysql_start_fail "便携 MySQL 在 30s 内未就绪，见 $logFile"
fi
mysql_start_ok
echo "[mysql-start] PID=$(cat "$MysqlPidPath")"
publish_auth_dsn "$Port"
sc_mysql_backup_cron_register
