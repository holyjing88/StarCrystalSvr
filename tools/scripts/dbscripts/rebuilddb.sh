#!/usr/bin/env bash
# 重建 MySQL auth schema + 清空 Redis 运行时键（sr:*）。
# 会删除 auth 库全部业务数据；Redis 仅删 sr:*（或 REDIS_REBUILD_FLUSHDB=1 时整库 FLUSHDB）。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=dbscripts-config.sh
source "$SCRIPT_DIR/dbscripts-config.sh"

echo "==> [1/2] MySQL rebuild (auth schema)"
bash "$SCRIPT_DIR/mysql/rebuild-auth-mysql.sh" "$@"

echo ""
echo "==> [2/2] Redis rebuild (sr:*)"
bash "$SCRIPT_DIR/redis/rebuild-redis.sh"

echo ""
echo "[rebuilddb] done"
