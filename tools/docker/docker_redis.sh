#!/usr/bin/env bash
# Docker Redis 7（开发 / 离线）
#
# 用法:
#   bash docker_redis.sh build          # 构建并自动导出 tar 到 mirror_save/redis/
#   bash docker_redis.sh save           # 仅重新导出 tar（不构建）
#   bash docker_redis.sh load           # 从 tar 导入本机 Docker
#   bash docker_redis.sh paths          # 打印镜像与 tar 路径说明
#   bash docker_redis.sh start|stop|status|logs
#
# start/stop 会同步启停容器内定时备份 crond（脚本已打入镜像，默认每天 03:00）。
# 配置在镜像 /etc/redis/redis.conf；数据与备份在宿主 DOCKER_REDIS_DATA（/data、/data/backups）。
#
# 环境变量: DOCKER_REDIS_IMAGE DOCKER_MIRROR_SAVE_DIR DOCKER_REDIS_DATA SKIP_MIRROR_SAVE
set -euo pipefail
DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"

cmd="${1:-start}"

case "$cmd" in
  build|save|load|paths|image)
    sc_docker_need
    ;;
  start|stop|status|logs)
    sc_docker_need
    mkdir -p "${DOCKER_REDIS_DATA:-$DOCKER_DATA_DIR/redis}/backups"
    ;;
  *)
    echo "Usage: $0 build|save|load|paths|image|start|stop|status|logs" >&2
    exit 1
    ;;
esac

case "$cmd" in
  build)
    if [[ ! -f "$DOCKER_REDIS_CONTEXT/Dockerfile" ]]; then
      echo "缺少 Dockerfile: $DOCKER_REDIS_CONTEXT/Dockerfile" >&2
      exit 1
    fi
    sc_docker_redis_stage_scripts
    echo "[docker_redis] building $DOCKER_REDIS_IMAGE ..."
    sc_docker_build_image "$DOCKER_REDIS_IMAGE" "$DOCKER_REDIS_CONTEXT" "$DOCKER_REDIS_BASE_IMAGE" "$DOCKER_REDIS_BASE_TAR"
    echo ""
    echo "Successfully tagged $DOCKER_REDIS_IMAGE"
    echo "镜像就绪: $DOCKER_REDIS_IMAGE"
    docker image inspect "$DOCKER_REDIS_IMAGE" --format 'IMAGE ID: {{.Id}}' 2>/dev/null || true
    echo ""
    sc_docker_auto_save_mirror "$DOCKER_REDIS_IMAGE" "$DOCKER_REDIS_TAR" "docker_redis"
    echo ""
    sc_docker_print_redis_paths
    ;;

  save)
    if ! sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
      echo "本机无镜像 $DOCKER_REDIS_IMAGE，请先: bash $0 build" >&2
      exit 1
    fi
    sc_docker_save_to_mirror "$DOCKER_REDIS_IMAGE" "$DOCKER_REDIS_TAR" "docker_redis"
    echo ""
    echo "save 完成: $DOCKER_REDIS_TAR"
    echo "  其他服务器: scp 后 docker_redis.sh load，见 doc/DOCKER_IMAGE_SHARE.md"
    ;;

  load)
    sc_docker_load_from_mirror "$DOCKER_REDIS_TAR" "docker_redis"
    echo "镜像就绪: $DOCKER_REDIS_IMAGE"
    docker images "$DOCKER_REDIS_IMAGE_REPO" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    ;;

  paths|image)
    sc_docker_print_redis_paths
    if sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
      docker images "$DOCKER_REDIS_IMAGE" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    else
      echo "(本机尚未 build/load 该镜像)"
    fi
    ;;

  start)
    redis_data="${DOCKER_REDIS_DATA:-$DOCKER_DATA_DIR/redis}"
    if ! sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
      echo "[docker_redis] 本机无 $DOCKER_REDIS_IMAGE，尝试 build ..."
      bash "$SCRIPT_DIR/docker_redis.sh" build
    fi
    if sc_docker_container_running "$DOCKER_REDIS_CONTAINER"; then
      sc_docker_redis_backup_start "$DOCKER_REDIS_CONTAINER"
      echo "[docker_redis] already running: $DOCKER_REDIS_CONTAINER"
      exit 0
    fi
    if sc_docker_container_exists "$DOCKER_REDIS_CONTAINER"; then
      docker start "$DOCKER_REDIS_CONTAINER"
    else
      docker run -d --name "$DOCKER_REDIS_CONTAINER" \
        -p "127.0.0.1:${DOCKER_REDIS_PORT}:6379" \
        -v "$redis_data:/data" \
        "$DOCKER_REDIS_IMAGE"
    fi
    echo "[docker_redis] waiting port $DOCKER_REDIS_PORT ..."
    sc_docker_wait_tcp 127.0.0.1 "$DOCKER_REDIS_PORT" 30
    sc_docker_redis_backup_start "$DOCKER_REDIS_CONTAINER"
    echo "[docker_redis] OK ($DOCKER_REDIS_CONTAINER, data=$redis_data)"
    ;;

  stop)
    if sc_docker_container_running "$DOCKER_REDIS_CONTAINER"; then
      sc_docker_redis_backup_stop "$DOCKER_REDIS_CONTAINER"
      docker stop "$DOCKER_REDIS_CONTAINER"
    fi
    echo "[docker_redis] stopped"
    ;;

  status)
    docker ps -a --filter "name=^${DOCKER_REDIS_CONTAINER}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
    ;;

  logs)
    docker logs --tail 80 "$DOCKER_REDIS_CONTAINER"
    ;;
esac
