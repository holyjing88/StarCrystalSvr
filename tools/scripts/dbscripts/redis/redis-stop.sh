#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"
# shellcheck source=redis-common.sh
source "$HERE/redis-common.sh"

CONF="$(sc_redis_conf_default)"
PORT="$(sc_redis_port "$CONF")"
REDIS_CLI="$(sc_redis_cli)"

sc_redis_backup_cron_unregister

redis_stop_ok() {
  echo "[redis-stop] OK — Redis 已停止 (port=$PORT)"
  exit 0
}

redis_stop_fail() {
  echo "[redis-stop] FAIL — $*" >&2
  exit 1
}

if ! sc_redis_ping "$PORT"; then
  redis_stop_ok
fi

echo "[redis-stop] 停止 Redis (port=$PORT) ..."

set +e
"$REDIS_CLI" -p "$PORT" SHUTDOWN
rc=$?
set -e

if [[ "$rc" -ne 0 ]]; then
  unit="$(sc_redis_system_unit 2>/dev/null || true)"
  if [[ -n "$unit" ]] && command -v systemctl >/dev/null 2>&1; then
    echo "[redis-stop] SHUTDOWN 失败，尝试 systemctl stop $unit ..."
    systemctl stop "$unit" 2>/dev/null || true
  elif [[ "${FORCE_KILL:-0}" == "1" ]]; then
    echo "[redis-stop] FORCE_KILL=1：尝试结束 redis-server 进程 ..."
    if command -v pkill >/dev/null 2>&1; then
      pkill -x redis-server 2>/dev/null || true
    elif command -v killall >/dev/null 2>&1; then
      killall redis-server 2>/dev/null || true
    fi
  fi
fi

if sc_redis_wait_stopped "$PORT"; then
  redis_stop_ok
fi
redis_stop_fail "Redis 仍在运行 (port=$PORT)"
