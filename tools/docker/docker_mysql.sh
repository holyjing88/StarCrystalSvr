#!/usr/bin/env bash
# Docker MySQL 8.4（开发 / 离线）
#
# 用法:
#   bash docker_mysql.sh build          # 构建并自动导出 tar 到 mirror_save/mysql/
#   bash docker_mysql.sh save           # 仅重新导出 tar（不构建）
#   bash docker_mysql.sh load           # 从 tar 导入本机 Docker
#   bash docker_mysql.sh paths          # 打印镜像与 tar 路径说明
#   bash docker_mysql.sh start|stop|status|logs
#
# start/stop 会同步启停容器内定时备份 crond（脚本已打入镜像，默认每天 03:00）。
# 数据与备份分别挂载到宿主 DOCKER_MYSQL_DATA、DOCKER_MYSQL_BACKUP_DIR。
#
# 环境变量: DOCKER_MYSQL_IMAGE DOCKER_MIRROR_SAVE_DIR DOCKER_MYSQL_DATA DOCKER_MYSQL_BACKUP_DIR SKIP_MIRROR_SAVE
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
    mysql_data="$DOCKER_MYSQL_DATA"
    mysql_backup_dir="$DOCKER_MYSQL_BACKUP_DIR"
    mkdir -p "$mysql_data" "$mysql_backup_dir"
    ;;
  *)
    echo "Usage: $0 build|save|load|paths|image|start|stop|status|logs" >&2
    exit 1
    ;;
esac

case "$cmd" in
  build)
    if [[ ! -f "$DOCKER_MYSQL_CONTEXT/Dockerfile" ]]; then
      echo "缺少 Dockerfile: $DOCKER_MYSQL_CONTEXT/Dockerfile" >&2
      exit 1
    fi
    sc_docker_mysql_stage_scripts
    echo "[docker_mysql] building $DOCKER_MYSQL_IMAGE ..."
    sc_docker_build_image "$DOCKER_MYSQL_IMAGE" "$DOCKER_MYSQL_CONTEXT" "$DOCKER_MYSQL_BASE_IMAGE" "$DOCKER_MYSQL_BASE_TAR"
    echo ""
    echo "Successfully tagged $DOCKER_MYSQL_IMAGE"
    echo "镜像就绪: $DOCKER_MYSQL_IMAGE"
    docker image inspect "$DOCKER_MYSQL_IMAGE" --format 'IMAGE ID: {{.Id}}' 2>/dev/null || true
    echo ""
    sc_docker_auto_save_mirror "$DOCKER_MYSQL_IMAGE" "$DOCKER_MYSQL_TAR" "docker_mysql"
    echo ""
    sc_docker_print_mysql_paths
    ;;

  save)
    if ! sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
      echo "本机无镜像 $DOCKER_MYSQL_IMAGE，请先: bash $0 build" >&2
      exit 1
    fi
    sc_docker_save_to_mirror "$DOCKER_MYSQL_IMAGE" "$DOCKER_MYSQL_TAR" "docker_mysql"
    echo ""
    echo "save 完成: $DOCKER_MYSQL_TAR"
    echo "  其他服务器: scp 后 docker_mysql.sh load，见 doc/DOCKER_IMAGE_SHARE.md"
    ;;

  load)
    sc_docker_load_from_mirror "$DOCKER_MYSQL_TAR" "docker_mysql"
    echo "镜像就绪: $DOCKER_MYSQL_IMAGE"
    docker images "$DOCKER_MYSQL_IMAGE_REPO" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    ;;

  paths|image)
    sc_docker_print_mysql_paths
    if sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
      docker images "$DOCKER_MYSQL_IMAGE" --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}'
    else
      echo "(本机尚未 build/load 该镜像)"
    fi
    ;;

  start)
    mysql_data="$DOCKER_MYSQL_DATA"
    mysql_backup_dir="$DOCKER_MYSQL_BACKUP_DIR"
    if ! sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
      echo "[docker_mysql] 本机无 $DOCKER_MYSQL_IMAGE，尝试 build ..."
      bash "$SCRIPT_DIR/docker_mysql.sh" build
    fi
    if sc_docker_container_running "$DOCKER_MYSQL_CONTAINER"; then
      sc_docker_mysql_backup_start "$DOCKER_MYSQL_CONTAINER"
      echo "[docker_mysql] already running: $DOCKER_MYSQL_CONTAINER"
      exit 0
    fi
    if sc_docker_container_exists "$DOCKER_MYSQL_CONTAINER"; then
      docker start "$DOCKER_MYSQL_CONTAINER"
    else
      docker run -d --name "$DOCKER_MYSQL_CONTAINER" \
        -p "127.0.0.1:${DOCKER_MYSQL_PORT}:3306" \
        -e "MYSQL_ROOT_PASSWORD=$DOCKER_MYSQL_ROOT_PASSWORD" \
        -e MYSQL_DATABASE=starcrystal_auth \
        -v "$mysql_data:/var/lib/mysql" \
        -v "$mysql_backup_dir:/backups" \
        "$DOCKER_MYSQL_IMAGE"
    fi
    echo "[docker_mysql] waiting port $DOCKER_MYSQL_PORT ..."
    sc_docker_wait_tcp 127.0.0.1 "$DOCKER_MYSQL_PORT" 120
    sc_docker_mysql_backup_start "$DOCKER_MYSQL_CONTAINER"
    echo "[docker_mysql] OK ($DOCKER_MYSQL_CONTAINER, data=$mysql_data, backups=$mysql_backup_dir)"
    ;;

  stop)
    if sc_docker_container_running "$DOCKER_MYSQL_CONTAINER"; then
      sc_docker_mysql_backup_stop "$DOCKER_MYSQL_CONTAINER"
      docker stop "$DOCKER_MYSQL_CONTAINER"
    fi
    echo "[docker_mysql] stopped"
    ;;

  status)
    docker ps -a --filter "name=^${DOCKER_MYSQL_CONTAINER}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
    ;;

  logs)
    docker logs --tail 80 "$DOCKER_MYSQL_CONTAINER"
    ;;
esac
