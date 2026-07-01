#!/usr/bin/env bash
# 启动 MySQL（系统或便携，若可用）+ Redis。不启动 starcrystalsvr。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=dbscripts-config.sh
source "$SCRIPT_DIR/dbscripts-config.sh"

MYSQL_PORT="${MYSQL_PORT:-3306}"
REDIS_PORT="${REDIS_PORT:-6379}"
MYSQL_STATUS=""
step=1

if sc_mysql_available; then
  echo "==> [$step/2] MySQL (port $MYSQL_PORT, mode=$(sc_mysql_mode))"
  MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/mysql-start.sh"
  MYSQL_STATUS="OK (mode=$(sc_mysql_mode), port $MYSQL_PORT)"
  step=$((step + 1))
else
  echo "==> [skip/2] MySQL: 本机无可用 MySQL，已跳过"
  MYSQL_STATUS="跳过（本机无可用 MySQL）"
fi

echo ""
echo "==> [$step/2] Redis (port $REDIS_PORT)"
REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/redis-start.sh"
REDIS_STATUS="OK (port $REDIS_PORT)"

echo ""
echo "[startdb] 完成 — MySQL: ${MYSQL_STATUS}; Redis: ${REDIS_STATUS}"
