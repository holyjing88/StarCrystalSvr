#!/usr/bin/env bash
# MySQL (系统或便携) → Redis → starcrystalsvr
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=starcrystal-config.sh
source "$SCRIPT_DIR/starcrystal-config.sh"

MYSQL_PORT="${MYSQL_PORT:-3306}"
REDIS_PORT="${REDIS_PORT:-6379}"
MYSQL_STATUS=""
step=1

if sc_mysql_available; then
  echo "==> [$step/3] MySQL (port $MYSQL_PORT, mode=$(sc_mysql_mode))"
  bash "$SCRIPT_DIR/dbscripts/mysql/mysql-start.sh"
  MYSQL_STATUS="OK (mode=$(sc_mysql_mode), port $MYSQL_PORT)"
  step=$((step + 1))
else
  echo "==> [skip/3] MySQL: 本机无可用 MySQL，已跳过"
  MYSQL_STATUS="跳过（本机无可用 MySQL）"
fi

echo ""
echo "==> [$step/3] Redis (port $REDIS_PORT)"
REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/dbscripts/redis/redis-start.sh"
REDIS_STATUS="OK (port $REDIS_PORT)"
step=$((step + 1))

echo ""
echo "==> [$step/3] starcrystalsvr"
bash "$RELEASE_ROOT/startsvr.sh"

echo ""
echo "[startallsvr] 完成 — MySQL: ${MYSQL_STATUS}; Redis: ${REDIS_STATUS}; starcrystalsvr: 已启动"
