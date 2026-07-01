#!/usr/bin/env bash
# Docker MySQL/Redis 验收：离线 load 或 build → start → 探测 → 备份 → stop
#
# 用法:
#   bash accept-docker.sh              # 完整 live 验收（需 docker）
#   bash accept-docker.sh --static     # 仅语法与文件检查
set -uo pipefail
export LANG="${LANG:-C.UTF-8}"

DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
# shellcheck source=../scripts/starcrystal-config.sh
source "$DOCKER_DIR/../scripts/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"

STATIC_ONLY=0
[[ "${1:-}" == "--static" ]] && STATIC_ONLY=1

PASS=0
FAIL=0
SKIP=0
ok() { echo "[PASS] $1 — $2"; PASS=$((PASS + 1)); }
bad() { echo "[FAIL] $1 — $2"; FAIL=$((FAIL + 1)); }
skip() { echo "[SKIP] $1 — $2"; SKIP=$((SKIP + 1)); }

echo "=== accept-docker ==="
echo "  REPO=$REPO_ROOT"
echo "  uname=$(uname -s 2>/dev/null || echo unknown)"
echo ""

STATIC_SH=(
  tools/docker/lib/docker-common.sh
  tools/docker/docker_mysql.sh
  tools/docker/docker_redis.sh
  tools/docker/docker_startdb.sh
  tools/docker/docker_stopdb.sh
  tools/docker/fetch-docker-base-images.sh
  tools/docker/setup-docker-linux.sh
  tools/docker/accept-docker.sh
)
for f in "${STATIC_SH[@]}"; do
  path="$REPO_ROOT/$f"
  if [[ ! -f "$path" ]]; then
    bad "ACC-static-missing" "$f"
    continue
  fi
  if bash -n "$path" 2>/dev/null; then
    ok "ACC-static-syntax" "$f"
  else
    bad "ACC-static-syntax" "$f"
  fi
done

[[ -f "$DOCKER_MYSQL_CONTEXT/conf.d/starcrystal.cnf" ]] && ok "ACC-mysql-conf" "镜像内 starcrystal.cnf" || bad "ACC-mysql-conf" "missing"
[[ -f "$DOCKER_REDIS_CONTEXT/redis.conf" ]] && ok "ACC-redis-conf" "镜像内 redis.conf" || bad "ACC-redis-conf" "missing"
[[ -f "$DOCKER_MYSQL_CONTEXT/Dockerfile" ]] && ok "ACC-mysql-dockerfile" "Dockerfile" || bad "ACC-mysql-dockerfile" "missing"
[[ -f "$DOCKER_REDIS_CONTEXT/Dockerfile" ]] && ok "ACC-redis-dockerfile" "Dockerfile" || bad "ACC-redis-dockerfile" "missing"

if [[ "$STATIC_ONLY" == 1 ]]; then
  echo ""
  echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip (static only) ==="
  [[ "$FAIL" -eq 0 ]]
  exit $?
fi

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
  skip "ACC-live" "docker 不可用"
  echo ""
  echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="
  exit 0
fi

sc_docker_need || { bad "ACC-docker" "docker daemon"; echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="; exit 1; }

ensure_redis_image() {
  if sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
    ok "ACC-redis-image" "已有 $DOCKER_REDIS_IMAGE"
    return 0
  fi
  if [[ -f "$DOCKER_REDIS_TAR" ]]; then
    echo "==> load $DOCKER_REDIS_TAR"
    bash "$SCRIPT_DIR/docker_redis.sh" load && ok "ACC-redis-load" "$DOCKER_REDIS_TAR" && return 0
    bad "ACC-redis-load" "$DOCKER_REDIS_TAR"
    return 1
  fi
  if sc_docker_try_load_base_image "$DOCKER_REDIS_BASE_IMAGE" "$DOCKER_REDIS_BASE_TAR"; then
    echo "==> build redis (offline base)"
    if bash "$SCRIPT_DIR/docker_redis.sh" build; then
      ok "ACC-redis-build" "offline build"
      return 0
    fi
  fi
  bad "ACC-redis-image" "无镜像/tar/基础镜像"
  return 1
}

ensure_mysql_image() {
  if sc_docker_image_exists "$DOCKER_MYSQL_IMAGE"; then
    ok "ACC-mysql-image" "已有 $DOCKER_MYSQL_IMAGE"
    return 0
  fi
  if [[ -f "$DOCKER_MYSQL_TAR" ]]; then
    echo "==> load $DOCKER_MYSQL_TAR"
    bash "$SCRIPT_DIR/docker_mysql.sh" load && ok "ACC-mysql-load" "$DOCKER_MYSQL_TAR" && return 0
    bad "ACC-mysql-load" "$DOCKER_MYSQL_TAR"
    return 1
  fi
  if sc_docker_try_load_base_image "$DOCKER_MYSQL_BASE_IMAGE" "$DOCKER_MYSQL_BASE_TAR"; then
    echo "==> build mysql (offline base)"
    if bash "$SCRIPT_DIR/docker_mysql.sh" build; then
      ok "ACC-mysql-build" "offline build"
      return 0
    fi
  fi
  bad "ACC-mysql-image" "无镜像/tar/基础镜像"
  return 1
}

ensure_redis_image || true
ensure_mysql_image || true

if ! sc_docker_image_exists "$DOCKER_REDIS_IMAGE"; then
  bad "ACC-live" "缺少 Redis 镜像，无法继续"
  echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="
  exit 1
fi

bash "$SCRIPT_DIR/docker_stopdb.sh" 2>/dev/null || true

if bash "$SCRIPT_DIR/docker_startdb.sh"; then
  ok "ACC-startdb" "mysql+redis up"
else
  bad "ACC-startdb" "failed"
  echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="
  exit 1
fi

if sc_docker_wait_tcp 127.0.0.1 "$DOCKER_REDIS_PORT" 15; then
  ok "ACC-redis-port" "$DOCKER_REDIS_PORT"
else
  bad "ACC-redis-port" "$DOCKER_REDIS_PORT"
fi

if sc_docker_container_running "$DOCKER_REDIS_CONTAINER"; then
  if docker exec "$DOCKER_REDIS_CONTAINER" redis-cli ping 2>/dev/null | grep -q PONG; then
    ok "ACC-redis-ping" "PONG"
  else
    bad "ACC-redis-ping" "no PONG"
  fi
  if docker exec "$DOCKER_REDIS_CONTAINER" test -f /etc/redis/redis.conf; then
    ok "ACC-redis-conf-in-image" "/etc/redis/redis.conf"
  else
    bad "ACC-redis-conf-in-image" "missing in container"
  fi
  if docker exec "$DOCKER_REDIS_CONTAINER" env PORT=6379 REDIS_CLI=/usr/local/bin/redis-cli REDIS_DIR=/data BACKUP_ROOT=/data/backups /opt/starcrystal/redis-scripts/redis-backup.sh; then
    ok "ACC-redis-backup" "manual backup"
  else
    bad "ACC-redis-backup" "manual backup failed"
  fi
  if [[ -d "${DOCKER_REDIS_DATA:-$DOCKER_DATA_DIR/redis}/backups" ]]; then
    ok "ACC-redis-backup-host" "${DOCKER_REDIS_DATA}/backups"
  else
    bad "ACC-redis-backup-host" "宿主 backups 目录"
  fi
fi

if sc_docker_image_exists "$DOCKER_MYSQL_IMAGE" && sc_docker_container_running "$DOCKER_MYSQL_CONTAINER"; then
  if sc_docker_wait_tcp 127.0.0.1 "$DOCKER_MYSQL_PORT" 15; then
    ok "ACC-mysql-port" "$DOCKER_MYSQL_PORT"
  else
    bad "ACC-mysql-port" "$DOCKER_MYSQL_PORT"
  fi
  if sc_docker_wait_mysql_ready "$DOCKER_MYSQL_CONTAINER" 60; then
    ok "ACC-mysql-ready" "mysqld accepting connections"
  else
    bad "ACC-mysql-ready" "mysqld not ready"
  fi
  if docker exec "$DOCKER_MYSQL_CONTAINER" mysql -h127.0.0.1 -P3306 -uroot "-p${DOCKER_MYSQL_ROOT_PASSWORD}" -e "CREATE DATABASE IF NOT EXISTS starcrystal_auth DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null; then
    :
  fi
  if docker exec "$DOCKER_MYSQL_CONTAINER" mysql -h127.0.0.1 -P3306 -uroot "-p${DOCKER_MYSQL_ROOT_PASSWORD}" -e "SHOW DATABASES;" 2>/dev/null | grep -q starcrystal_auth; then
    ok "ACC-mysql-db" "starcrystal_auth"
  else
    bad "ACC-mysql-db" "starcrystal_auth"
  fi
  if docker exec "$DOCKER_MYSQL_CONTAINER" test -f /etc/mysql/conf.d/starcrystal.cnf; then
    ok "ACC-mysql-conf-in-image" "starcrystal.cnf"
  else
    bad "ACC-mysql-conf-in-image" "missing in container"
  fi
  if docker exec "$DOCKER_MYSQL_CONTAINER" env MYSQL_UNIX_SOCKET=/var/run/mysqld/mysqld.sock MYSQL_ROOT_PASSWORD="$DOCKER_MYSQL_ROOT_PASSWORD" BACKUP_ROOT=/backups /opt/starcrystal/mysql-scripts/mysql-backup.sh; then
    ok "ACC-mysql-backup" "manual backup"
  else
    bad "ACC-mysql-backup" "manual backup failed"
  fi
fi

bash "$SCRIPT_DIR/docker_stopdb.sh" && ok "ACC-stopdb" "down"

echo ""
echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="
[[ "$FAIL" -eq 0 ]]
