#!/usr/bin/env bash
# Docker 版 startdb：启动 MySQL + Redis 容器（不启 starcrystalsvr）
set -euo pipefail
DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"

bash "$SCRIPT_DIR/docker_mysql.sh" start
echo ""
bash "$SCRIPT_DIR/docker_redis.sh" start
echo ""
export AUTH_MYSQL_DSN="$(sc_docker_dev_auth_dsn)"
sc_save_last_auth_dsn "$AUTH_MYSQL_DSN"
echo "docker_startdb: MySQL + Redis containers started."
echo "  AUTH_MYSQL_DSN=$AUTH_MYSQL_DSN"
