#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=starcrystal-config.sh
source "$SCRIPT_DIR/starcrystal-config.sh"

REDIS_PORT="${REDIS_PORT:-6379}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
SVR_STATUS=""
REDIS_STATUS=""
MYSQL_STATUS=""

echo "==> [1/3] starcrystalsvr"
if bash "$RELEASE_ROOT/stopsvr.sh"; then
  SVR_STATUS="OK"
else
  SVR_STATUS="FAIL"
fi

echo ""
echo "==> [2/3] Redis (port $REDIS_PORT)"
if REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/dbscripts/redis/redis-stop.sh"; then
  REDIS_STATUS="OK (port $REDIS_PORT)"
else
  REDIS_STATUS="FAIL (port $REDIS_PORT)"
fi

echo ""
if sc_mysql_available; then
  echo "==> [3/3] MySQL (port $MYSQL_PORT, mode=$(sc_mysql_mode))"
  if MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/dbscripts/mysql/mysql-stop.sh"; then
    MYSQL_STATUS="OK (mode=$(sc_mysql_mode), port $MYSQL_PORT)"
  else
    MYSQL_STATUS="FAIL (port $MYSQL_PORT)"
  fi
else
  echo "==> [3/3] MySQL: 本机无可用 MySQL，已跳过"
  MYSQL_STATUS="跳过（本机无可用 MySQL）"
fi

echo ""
echo "[stopallsvr] 完成 — starcrystalsvr: ${SVR_STATUS}; Redis: ${REDIS_STATUS}; MySQL: ${MYSQL_STATUS}"
