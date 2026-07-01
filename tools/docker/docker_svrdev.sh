#!/usr/bin/env bash
# 开发环境：Docker MySQL/Redis → bootstrap schema → 启动 release/starcrystalsv
# 用法: cd server-go && bash tools/docker/docker_svrdev.sh
# 环境变量: SKIP_BUILD=1 | SKIP_SCHEMA=1
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SCRIPT_DIR="$DOCKER_DIR"
SC_SCRIPTS_ROOT="$(cd "$DOCKER_DIR/../scripts" && pwd)"
# shellcheck source=../scripts/starcrystal-config.sh
source "$SC_SCRIPTS_ROOT/starcrystal-config.sh"
# shellcheck source=lib/docker-common.sh
source "$DOCKER_DIR/lib/docker-common.sh"
cd "$RELEASE_ROOT"

export AUTH_MYSQL_DSN="$(sc_docker_dev_auth_dsn)"
export AUTH_SMS_MOCK="${AUTH_SMS_MOCK:-1}"
export REDIS_PORT="${DOCKER_REDIS_PORT}"
export PORT="${DOCKER_REDIS_PORT}"

echo "==> [1/4] docker_startdb"
bash "$SCRIPT_DIR/docker_startdb.sh"

if [[ "${SKIP_SCHEMA:-0}" != "1" ]]; then
  echo ""
  echo "==> [2/4] bootstrap + schema (Docker MySQL root)"
  docker exec "$DOCKER_MYSQL_CONTAINER" mysql -uroot "-p${DOCKER_MYSQL_ROOT_PASSWORD}" -e "
CREATE DATABASE IF NOT EXISTS starcrystal_auth DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'star_auth'@'%' IDENTIFIED BY 'star_auth_123456';
CREATE USER IF NOT EXISTS 'star_auth'@'localhost' IDENTIFIED BY 'star_auth_123456';
GRANT ALL PRIVILEGES ON starcrystal_auth.* TO 'star_auth'@'%';
GRANT ALL PRIVILEGES ON starcrystal_auth.* TO 'star_auth'@'localhost';
FLUSH PRIVILEGES;
" 2>/dev/null || true
  sql_file="$REPO_ROOT/tools/scripts/dbscripts/sql/starcrystal_auth_mysql.sql"
  if [[ -f "$sql_file" ]]; then
    docker exec -i "$DOCKER_MYSQL_CONTAINER" mysql -ustar_auth -pstar_auth_123456 starcrystal_auth \
      <"$sql_file" || docker exec -i "$DOCKER_MYSQL_CONTAINER" mysql -uroot "-p${DOCKER_MYSQL_ROOT_PASSWORD}" starcrystal_auth <"$sql_file"
  fi
  export AUTH_MYSQL_DSN="$(sc_docker_dev_auth_dsn)"
  sc_save_last_auth_dsn "$AUTH_MYSQL_DSN"
else
  echo "==> [2/4] skip schema (SKIP_SCHEMA=1)"
fi

if [[ "${SKIP_BUILD:-0}" != "1" ]] && [[ ! -f "$RELEASE_ROOT/starcrystalsvr" && ! -f "$RELEASE_ROOT/starcrystalsvr.exe" ]]; then
  echo ""
  echo "==> [3/4] build"
  if command -v go >/dev/null 2>&1; then
    bash "$SC_SCRIPTS_ROOT/build.sh"
  else
    echo "[warn] no go; use existing binary or build on Windows"
  fi
else
  echo "==> [3/4] skip build"
fi

echo ""
echo "==> [4/4] startsvr"
bash "$RELEASE_ROOT/startsvr.sh"
echo ""
echo "docker_svrdev: API at $(sc_api_base_url)"
