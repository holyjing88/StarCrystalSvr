#!/usr/bin/env bash
# 停止 Redis + MySQL（若可用）。不停止 starcrystalsvr。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=dbscripts-config.sh
source "$SCRIPT_DIR/dbscripts-config.sh"

REDIS_PORT="${REDIS_PORT:-6379}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
REDIS_STATUS=""
MYSQL_STATUS=""

echo "==> [1/2] Redis (port $REDIS_PORT)"
if REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/redis-stop.sh"; then
  REDIS_STATUS="OK (port $REDIS_PORT)"
else
  REDIS_STATUS="FAIL (port $REDIS_PORT)"
fi

echo ""
if sc_mysql_available; then
  echo "==> [2/2] MySQL (port $MYSQL_PORT, mode=$(sc_mysql_mode))"
  if MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/mysql-stop.sh"; then
    MYSQL_STATUS="OK (mode=$(sc_mysql_mode), port $MYSQL_PORT)"
  else
    MYSQL_STATUS="FAIL (port $MYSQL_PORT)"
  fi
else
  echo "==> [2/2] MySQL: 本机无可用 MySQL，已跳过"
  MYSQL_STATUS="跳过（本机无可用 MySQL）"
fi

echo ""
echo "[stopdb] 完成 — Redis: ${REDIS_STATUS}; MySQL: ${MYSQL_STATUS}"
