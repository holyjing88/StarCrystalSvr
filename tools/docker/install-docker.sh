#!/usr/bin/env bash
# 使用 Docker 拉起 MySQL + Redis（无需 offlinesofts 便携包）
# 用法: cd server-go && bash tools/docker/install-docker.sh
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"
sc_docker_need

echo "StarCrystal install-docker"
echo "  RELEASE_ROOT=$RELEASE_ROOT"
echo "  REPO_ROOT=$REPO_ROOT"
echo ""

if sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
  echo "==> MySQL 镜像已存在: $DOCKER_MYSQL_IMAGE"
else
  if [[ -f "$DOCKER_MYSQL_TAR" ]]; then
    echo "==> load MySQL tar"
    bash "$SCRIPT_DIR/docker_mysql.sh" load
  else
    bash "$SCRIPT_DIR/fetch-docker-base-images.sh" load 2>/dev/null || true
    echo "==> build MySQL 镜像"
    bash "$SCRIPT_DIR/docker_mysql.sh" build
  fi
fi
if sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
  echo "==> Redis 镜像已存在: $DOCKER_REDIS_IMAGE"
else
  if [[ -f "$DOCKER_REDIS_TAR" ]]; then
    echo "==> load Redis tar"
    bash "$SCRIPT_DIR/docker_redis.sh" load
  else
    bash "$SCRIPT_DIR/fetch-docker-base-images.sh" load 2>/dev/null || true
    echo "==> build Redis 镜像"
    bash "$SCRIPT_DIR/docker_redis.sh" build
  fi
fi
echo ""
bash "$SCRIPT_DIR/docker_startdb.sh"

echo ""
echo "install-docker: done."
echo "  下一步: bash tools/docker/docker_svrdev.sh"
echo "  停止:   bash tools/docker/docker_stopdb.sh"
