#!/usr/bin/env bash
# 将 MySQL / Redis / release 镜像导出到 tools/docker/mirror_save/（镜像缺失时才 build）
#
# 强制 rebuild 三台镜像请用: bash tools/docker/docker_mirror_build.sh
#
# 用法: cd server-go && bash tools/docker/docker_mirror_save.sh
set -euo pipefail
DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"
sc_docker_need

echo "==> mirror_save 目录: $DOCKER_MIRROR_SAVE_DIR"
mkdir -p "$DOCKER_MIRROR_SAVE_DIR/mysql" "$DOCKER_MIRROR_SAVE_DIR/redis" "$DOCKER_MIRROR_SAVE_DIR/release"

if ! sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
  bash "$DOCKER_DIR/docker_mysql.sh" build
else
  bash "$DOCKER_DIR/docker_mysql.sh" save
fi

if ! sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
  bash "$DOCKER_DIR/docker_redis.sh" build
else
  bash "$DOCKER_DIR/docker_redis.sh" save
fi

if ! sc_docker_image_exists "$DOCKER_RELEASE_IMAGE"; then
  bash "$DOCKER_DIR/docker_release.sh" build
else
  bash "$DOCKER_DIR/docker_release.sh" save
fi

echo ""
echo "全部已导出到 $DOCKER_MIRROR_SAVE_DIR"
ls -lh "$DOCKER_MIRROR_SAVE_DIR"/*/*.tar 2>/dev/null || true
