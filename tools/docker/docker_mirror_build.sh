#!/usr/bin/env bash
# 一键构建 MySQL / Redis / release 镜像并导出到 tools/docker/mirror_save/
#
# 用法:
#   cd server-go && bash tools/docker/docker_mirror_build.sh
#
# 与 docker_mirror_save.sh 区别:
#   mirror_build  始终 rebuild 三台镜像（build 已含 save）
#   mirror_save   镜像已存在时仅 save，缺失时才 build
#
# 环境变量: SKIP_MIRROR_SAVE=1  跳过导出 tar（仅本地调试）
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"
sc_docker_need

log() { echo ""; echo "==> $*"; }

log "StarCrystal Docker mirror build"
echo "  REPO_ROOT=$REPO_ROOT"
echo "  mirror_save=$DOCKER_MIRROR_SAVE_DIR"
mkdir -p "$DOCKER_MIRROR_SAVE_DIR/mysql" "$DOCKER_MIRROR_SAVE_DIR/redis" "$DOCKER_MIRROR_SAVE_DIR/release"

log "MySQL ($DOCKER_MYSQL_IMAGE)"
bash "$DOCKER_DIR/docker_mysql.sh" build

log "Redis ($DOCKER_REDIS_IMAGE)"
bash "$DOCKER_DIR/docker_redis.sh" build

log "Release ($DOCKER_RELEASE_IMAGE)"
bash "$DOCKER_DIR/docker_release.sh" build

log "完成"
echo "  MySQL:   $DOCKER_MYSQL_TAR"
echo "  Redis:   $DOCKER_REDIS_TAR"
echo "  Release: $DOCKER_RELEASE_TAR"
echo ""
ls -lh "$DOCKER_MIRROR_SAVE_DIR"/*/*.tar 2>/dev/null || true
echo ""
echo "目标机: bash tools/docker/docker_* .sh load && bash tools/docker/docker_startdb.sh"
