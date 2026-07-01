#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"
# shellcheck source=redis-common.sh
source "$HERE/redis-common.sh"

sc_redis_ensure_dirs

CONF_DEFAULT="$(sc_redis_conf_default)"
CONFIG_FILE=""
if [[ -f "$CONF_DEFAULT" ]]; then
  CONFIG_FILE="$CONF_DEFAULT"
else
  DATA_DIR="$(sc_redis_default_data_dir)"
  CONFIG_FILE="$HERE/redis.generated.conf"
  LISTEN_PORT="$(sc_redis_port "")"
  cat > "$CONFIG_FILE" <<EOF
port ${LISTEN_PORT}
bind 127.0.0.1
dir ${DATA_DIR}
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000
appendonly no
loglevel notice
EOF
  echo "[redis-start] 未找到 redis.conf，已生成临时配置: $CONFIG_FILE"
fi

cd "$(cd "$(dirname "$CONFIG_FILE")" && pwd)"
PORT="$(sc_redis_port "$CONFIG_FILE")"
REDIS_SERVER="$(sc_redis_server)"
REDIS_CLI="$(sc_redis_cli)"

redis_start_ok() {
  echo "[redis-start] OK — Redis 运行中 (port=$PORT)"
  echo "[redis-start] data=$(sc_redis_resolve_data_dir "$CONFIG_FILE") backup=$(sc_redis_default_backup_dir)"
}

redis_start_fail() {
  echo "[redis-start] FAIL — $*" >&2
  exit 1
}

if sc_redis_ping "$PORT"; then
  redis_start_ok
  sc_redis_backup_cron_register
  exit 0
fi

unit="$(sc_redis_system_unit 2>/dev/null || true)"
if [[ -n "$unit" ]] && command -v systemctl >/dev/null 2>&1; then
  echo "[redis-start] 启动系统 Redis ($unit) ..."
  systemctl start "$unit"
  sleep 1
  if sc_redis_ping "$PORT"; then
    redis_start_ok
    sc_redis_backup_cron_register
    exit 0
  fi
fi

mkdir -p "$HERE/logs"
echo "[redis-start] 启动 Redis (port=$PORT) ..."
echo "[redis-start] $REDIS_SERVER $CONFIG_FILE --port $PORT"
nohup "$REDIS_SERVER" "$CONFIG_FILE" --port "$PORT" >>"$HERE/logs/redis-server.log" 2>&1 &
sleep 1
if sc_redis_ping "$PORT"; then
  redis_start_ok
  sc_redis_backup_cron_register
  exit 0
fi
redis_start_fail "启动后 PING 失败，见 $HERE/logs/redis-server.log"
