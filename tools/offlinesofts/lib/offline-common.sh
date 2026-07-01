#!/usr/bin/env bash
# shellcheck disable=SC2034
# Source from tools/offlinesofts/*.sh
export LANG="${LANG:-C.UTF-8}"

_OFFLINE_LIB="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
OFFLINE_SOFTS_ROOT="$(cd "$_OFFLINE_LIB/.." && pwd)"
_SC_SCRIPTS_FOR_OFFLINE="$(cd "$OFFLINE_SOFTS_ROOT/../scripts" && pwd)"
# shellcheck source=../../scripts/starcrystal-config.sh
source "$_SC_SCRIPTS_FOR_OFFLINE/starcrystal-config.sh"

MYSQL_VERSION="${MYSQL_VERSION:-8.4.8}"
REDIS_VERSION="${REDIS_VERSION:-7.2.6}"
GO_VERSION="${GO_VERSION:-1.24.0}"

MYSQL_ARCHIVE="mysql-${MYSQL_VERSION}-linux-glibc2.28-x86_64.tar.xz"
REDIS_ARCHIVE="redis-${REDIS_VERSION}.tar.gz"
GO_ARCHIVE="go${GO_VERSION}.linux-amd64.tar.gz"

MYSQL_URL="https://dev.mysql.com/get/Downloads/MySQL-8.4/${MYSQL_ARCHIVE}"
REDIS_URL="https://download.redis.io/releases/${REDIS_ARCHIVE}"
GO_URL="https://go.dev/dl/${GO_ARCHIVE}"

offline_mysql_install_base() {
  local games_root
  games_root="$(cd "$REPO_ROOT/../.." && pwd)"
  echo "$games_root/mysql-portable-linux/mysql-${MYSQL_VERSION}-linux-glibc2.28-x86_64"
}

offline_mysql_data_dir() {
  local base parent
  base="$(offline_mysql_install_base)"
  parent="$(dirname "$base")"
  echo "$parent/data"
}

offline_redis_linux_dest() {
  echo "$REPO_ROOT/redis/linux"
}

offline_go_local_root() {
  echo "${STARCRYSTAL_GO_ROOT:-$REPO_ROOT/.go-toolchain/go}"
}

offline_require_file() {
  local f="$1" hint="${2:-}"
  if [[ ! -f "$f" ]]; then
    echo "缺少文件: $f" >&2
    [[ -n "$hint" ]] && echo "$hint" >&2
    return 1
  fi
}

offline_find_archive() {
  local subdir="$1" pattern="$2"
  local d="$OFFLINE_SOFTS_ROOT/$subdir"
  local f
  shopt -s nullglob
  for f in "$d"/$pattern; do
    if [[ -f "$f" ]]; then
      echo "$f"
      return 0
    fi
  done
  shopt -u nullglob
  return 1
}
