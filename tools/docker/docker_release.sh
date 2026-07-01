#!/usr/bin/env bash
# Docker release 运行包（configs/assets/启停脚本；不含 starcrystalsvr.exe，log/ 仅空目录）
#
# 用法:
#   bash docker_release.sh build          # 构建并自动导出 tar 到 mirror_save/release/
#   bash docker_release.sh save           # 仅重新导出 tar（不构建）
#   bash docker_release.sh load|paths|image|start|stop|status|logs
#
# 环境变量:
#   DOCKER_RELEASE_IMAGE  DOCKER_RELEASE_SVR_BIN（挂载到 /app/starcrystalsvr）
#   DOCKER_RELEASE_PORT   AUTH_MYSQL_DSN / REDIS_ADDR（start 时默认连 host 上 MySQL/Redis）
#   SKIP_MIRROR_SAVE      build 时不导出 tar
set -euo pipefail
DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"

cmd="${1:-paths}"

case "$cmd" in
  build|save|load|paths|image)
    sc_docker_need
    ;;
  start|stop|status|logs)
    sc_docker_need
    ;;
  *)
    echo "Usage: $0 build|save|load|paths|image|start|stop|status|logs" >&2
    exit 1
    ;;
esac

case "$cmd" in
  build)
    sc_docker_release_apply_ignore
    trap sc_docker_release_restore_ignore EXIT
    echo "[docker_release] building $DOCKER_RELEASE_IMAGE (context=$RELEASE_ROOT) ..."
    docker build -f "$DOCKER_RELEASE_CONTEXT/Dockerfile" -t "$DOCKER_RELEASE_IMAGE" "$RELEASE_ROOT"
    sc_docker_release_restore_ignore
    trap - EXIT
    echo ""
    echo "Successfully tagged $DOCKER_RELEASE_IMAGE"
    docker image inspect "$DOCKER_RELEASE_IMAGE" --format 'IMAGE ID: {{.Id}}' 2>/dev/null || true
    echo ""
    sc_docker_auto_save_mirror "$DOCKER_RELEASE_IMAGE" "$DOCKER_RELEASE_TAR" "docker_release"
    echo ""
    sc_docker_print_release_paths
    ;;

  save)
    if ! sc_docker_image_exists "$DOCKER_RELEASE_IMAGE"; then
      echo "本机无镜像 $DOCKER_RELEASE_IMAGE，请先: bash $0 build" >&2
      exit 1
    fi
    sc_docker_save_to_mirror "$DOCKER_RELEASE_IMAGE" "$DOCKER_RELEASE_TAR" "docker_release"
    echo ""
    echo "save 完成: $DOCKER_RELEASE_TAR"
    echo "  目标机: bash tools/docker/docker_release.sh load && 挂载 starcrystalsvr 后 start"
    ;;

  load)
    sc_docker_load_from_mirror "$DOCKER_RELEASE_TAR" "docker_release"
    echo "镜像就绪: $DOCKER_RELEASE_IMAGE"
    docker images "$DOCKER_RELEASE_IMAGE_REPO" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    ;;

  paths|image)
    sc_docker_print_release_paths
    if sc_docker_image_exists "$DOCKER_RELEASE_IMAGE"; then
      docker images "$DOCKER_RELEASE_IMAGE" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    else
      echo "(本机尚未 build/load 该镜像)"
    fi
    ;;

  start)
    if ! sc_docker_image_exists "$DOCKER_RELEASE_IMAGE"; then
      echo "[docker_release] 本机无 $DOCKER_RELEASE_IMAGE，尝试 build ..."
      bash "$DOCKER_DIR/docker_release.sh" build
    fi
    svr_bin="$(sc_docker_resolve_release_svr_bin)" || {
      echo "未找到 starcrystalsvr 二进制，请编译后放到 release/ 或设置 DOCKER_RELEASE_SVR_BIN" >&2
      exit 1
    }
    release_log="$DOCKER_DATA_DIR/release-log"
    mkdir -p "$release_log"
    auth_dsn="${AUTH_MYSQL_DSN:-$(sc_docker_container_auth_dsn)}"
    redis_addr="${REDIS_ADDR:-host.docker.internal:${DOCKER_REDIS_PORT}}"

    if sc_docker_container_running "$DOCKER_RELEASE_CONTAINER"; then
      echo "[docker_release] already running: $DOCKER_RELEASE_CONTAINER"
      exit 0
    fi
    if sc_docker_container_exists "$DOCKER_RELEASE_CONTAINER"; then
      docker rm -f "$DOCKER_RELEASE_CONTAINER" >/dev/null 2>&1 || true
    fi
    echo "[docker_release] binary=$svr_bin"
    docker run -d --name "$DOCKER_RELEASE_CONTAINER" \
      --add-host=host.docker.internal:host-gateway \
      -p "127.0.0.1:${DOCKER_RELEASE_PORT}:8080" \
      -v "$svr_bin:/app/starcrystalsvr:ro" \
      -v "$release_log:/app/log" \
      -e "AUTH_MYSQL_DSN=$auth_dsn" \
      -e "REDIS_ADDR=$redis_addr" \
      -e AUTH_SMS_MOCK=1 \
      "$DOCKER_RELEASE_IMAGE"
    echo "[docker_release] waiting port $DOCKER_RELEASE_PORT ..."
    sc_docker_wait_tcp 127.0.0.1 "$DOCKER_RELEASE_PORT" 45
    echo "[docker_release] OK ($DOCKER_RELEASE_CONTAINER)"
    echo "  API: http://127.0.0.1:${DOCKER_RELEASE_PORT}"
    ;;

  stop)
    if sc_docker_container_running "$DOCKER_RELEASE_CONTAINER"; then
      docker exec "$DOCKER_RELEASE_CONTAINER" ./stopsvr.sh 2>/dev/null || docker stop "$DOCKER_RELEASE_CONTAINER"
    elif sc_docker_container_exists "$DOCKER_RELEASE_CONTAINER"; then
      docker stop "$DOCKER_RELEASE_CONTAINER" 2>/dev/null || true
    fi
    echo "[docker_release] stopped"
    ;;

  status)
    docker ps -a --filter "name=^${DOCKER_RELEASE_CONTAINER}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
    ;;

  logs)
    docker logs --tail 80 "$DOCKER_RELEASE_CONTAINER"
    ;;
esac
