#!/usr/bin/env bash
# Docker 版 stopdb：停止 Redis + MySQL 容器
set -euo pipefail
DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
bash "$DOCKER_DIR/docker_redis.sh" stop || true
echo ""
bash "$DOCKER_DIR/docker_mysql.sh" stop || true
echo ""
echo "docker_stopdb: finished."
