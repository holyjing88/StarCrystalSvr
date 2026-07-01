#!/usr/bin/env bash
# 在可联网机器导出 Docker 基础镜像；离线机 load 后再 docker_* .sh build
#
# 用法:
#   bash fetch-docker-base-images.sh save   # 联网机：pull + save 到 mirror_save/
#   bash fetch-docker-base-images.sh load   # 离线机：load 基础镜像
#   bash fetch-docker-base-images.sh paths
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"

cmd="${1:-paths}"

case "$cmd" in
  save|load|paths)
    ;;
  *)
    echo "Usage: $0 save|load|paths" >&2
    exit 1
    ;;
esac

case "$cmd" in
  paths)
    echo "Redis 基础: $DOCKER_REDIS_BASE_IMAGE"
    echo "  tar: $DOCKER_REDIS_BASE_TAR"
    echo "MySQL 基础: $DOCKER_MYSQL_BASE_IMAGE"
    echo "  tar: $DOCKER_MYSQL_BASE_TAR"
    ;;
  save)
    sc_docker_need
    mkdir -p "$(dirname "$DOCKER_REDIS_BASE_TAR")" "$(dirname "$DOCKER_MYSQL_BASE_TAR")"
    echo "[fetch-base] pull $DOCKER_REDIS_BASE_IMAGE"
    docker pull "$DOCKER_REDIS_BASE_IMAGE"
    echo "[fetch-base] pull $DOCKER_MYSQL_BASE_IMAGE"
    docker pull "$DOCKER_MYSQL_BASE_IMAGE"
    sc_docker_save_to_mirror "$DOCKER_REDIS_BASE_IMAGE" "$DOCKER_REDIS_BASE_TAR" "fetch-base-redis"
    sc_docker_save_to_mirror "$DOCKER_MYSQL_BASE_IMAGE" "$DOCKER_MYSQL_BASE_TAR" "fetch-base-mysql"
    echo ""
    echo "基础镜像已导出。拷到离线机后: bash $0 load"
    ;;
  load)
    sc_docker_need
    loaded=0
    if [[ -f "$DOCKER_REDIS_BASE_TAR" ]]; then
      sc_docker_load_from_mirror "$DOCKER_REDIS_BASE_TAR" "fetch-base-redis"
      loaded=1
    else
      echo "[fetch-base] 未找到 $DOCKER_REDIS_BASE_TAR" >&2
    fi
    if [[ -f "$DOCKER_MYSQL_BASE_TAR" ]]; then
      sc_docker_load_from_mirror "$DOCKER_MYSQL_BASE_TAR" "fetch-base-mysql"
      loaded=1
    else
      echo "[fetch-base] 未找到 $DOCKER_MYSQL_BASE_TAR" >&2
    fi
    if [[ "$loaded" != 1 ]]; then
      echo "请将 redis-7-alpine-base.tar / mysql-8.4-base.tar 放入 mirror_save 对应目录" >&2
      exit 1
    fi
    echo "基础镜像已导入，可执行: bash docker_redis.sh build && bash docker_mysql.sh build"
    ;;
esac
