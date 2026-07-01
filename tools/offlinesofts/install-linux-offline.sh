#!/usr/bin/env bash
# 离线机：从 offlinesofts/ 各子目录安装便携 MySQL、编译 Redis、可选 Go 与 go mod 缓存。
# 用法: cd server-go/release && bash ../tools/offlinesofts/install-linux-offline.sh
# 环境变量: SKIP_GO=1 | SKIP_GOMOD=1 | SKIP_RUNTIME_DEB=1 | SKIP_MYSQL=1 | SKIP_REDIS=1
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=lib/offline-common.sh
source "$HERE/lib/offline-common.sh"

install_runtime_debs() {
  [[ "${SKIP_RUNTIME_DEB:-0}" == "1" ]] && return 0
  local deb_dir="$OFFLINE_SOFTS_ROOT/runtime/deb"
  shopt -s nullglob
  local debs=("$deb_dir"/*.deb)
  shopt -u nullglob
  if [[ ${#debs[@]} -eq 0 ]]; then
    echo "[runtime] 无 .deb，请确认已安装 libaio（MySQL）及 gcc/make（编译 Redis）"
    return 0
  fi
  echo "==> 安装 runtime .deb (${#debs[@]} 个，需要 root)"
  if [[ "$(id -u)" -ne 0 ]]; then
    sudo dpkg -i "${debs[@]}" || sudo apt-get install -f -y
  else
    dpkg -i "${debs[@]}" || apt-get install -f -y
  fi
}

install_mysql() {
  [[ "${SKIP_MYSQL:-0}" == "1" ]] && return 0
  local arc dest parent
  arc="$(offline_find_archive mysql "mysql-*.tar.xz")" || {
    offline_require_file "$OFFLINE_SOFTS_ROOT/mysql/$MYSQL_ARCHIVE" \
      "请将 $MYSQL_ARCHIVE 放入 offlinesofts/mysql/"
    arc="$OFFLINE_SOFTS_ROOT/mysql/$MYSQL_ARCHIVE"
  }
  dest="$(offline_mysql_install_base)"
  parent="$(dirname "$dest")"
  mkdir -p "$parent"
  if [[ -x "$dest/bin/mysqld" ]]; then
    echo "[mysql] 已安装: $dest"
    return 0
  fi
  echo "[mysql] 解压到 $parent"
  rm -rf "$dest"
  tar -xJf "$arc" -C "$parent"
  echo "[mysql] OK: $dest/bin/mysqld"
}

install_redis() {
  [[ "${SKIP_REDIS:-0}" == "1" ]] && return 0
  local arc dest src
  arc="$(offline_find_archive redis "redis-*.tar.gz")" || {
    offline_require_file "$OFFLINE_SOFTS_ROOT/redis/$REDIS_ARCHIVE" \
      "请将 $REDIS_ARCHIVE 放入 offlinesofts/redis/"
    arc="$OFFLINE_SOFTS_ROOT/redis/$REDIS_ARCHIVE"
  }
  dest="$(offline_redis_linux_dest)"
  if [[ -x "$dest/redis-server" && -x "$dest/redis-cli" ]]; then
    echo "[redis] 已安装: $dest"
    return 0
  fi
  for c in gcc make; do
    command -v "$c" >/dev/null 2>&1 || {
      echo "编译 Redis 需要 $c（apt install build-essential 或 FETCH_BUILD_DEB=1 下载 .deb）" >&2
      exit 1
    }
  done
  TMP="${TMPDIR:-/tmp}/starcrystal-redis-offline-$$"
  mkdir -p "$TMP" "$dest"
  cleanup() { rm -rf "$TMP"; }
  trap cleanup EXIT
  tar -xzf "$arc" -C "$TMP"
  src="$TMP/redis-${REDIS_VERSION}"
  [[ -d "$src" ]] || src="$(find "$TMP" -maxdepth 1 -type d -name 'redis-*' | head -1)"
  echo "[redis] 编译中…"
  (cd "$src" && make -j"${NPROC:-$(nproc 2>/dev/null || echo 2)}" BUILD_TLS=no)
  install -m0755 "$src/src/redis-server" "$dest/redis-server"
  install -m0755 "$src/src/redis-cli" "$dest/redis-cli"
  echo "[redis] OK: $dest"
}

install_go() {
  [[ "${SKIP_GO:-0}" == "1" ]] && return 0
  local arc root
  arc="$(offline_find_archive go "go*.linux-amd64.tar.gz")" || {
    offline_require_file "$OFFLINE_SOFTS_ROOT/go/$GO_ARCHIVE" "SKIP_GO=1 可跳过"
    return 0
  }
  root="$(offline_go_local_root)"
  if [[ -x "$root/bin/go" ]]; then
    echo "[go] 已安装: $root"
    return 0
  fi
  mkdir -p "$(dirname "$root")"
  echo "[go] 解压到 $(dirname "$root")"
  tar -xzf "$arc" -C "$(dirname "$root")"
  if [[ ! -d "$root" ]]; then
    mv "$(dirname "$root")/go" "$root"
  fi
  echo "[go] 使用: export PATH=\"$root/bin:\$PATH\""
}

install_gomod_cache() {
  [[ "${SKIP_GOMOD:-0}" == "1" ]] && return 0
  local cache_tgz="$OFFLINE_SOFTS_ROOT/go-modules/gomod-cache.tar.gz"
  [[ -f "$cache_tgz" ]] || { echo "[gomod] 无缓存，跳过"; return 0; }
  local dest="${GOMODCACHE:-$REPO_ROOT/.gomodcache}"
  rm -rf "$dest"
  mkdir -p "$dest"
  echo "[gomod] 解压到 $dest"
  tar -xzf "$cache_tgz" -C "$dest"
  echo "[gomod] 离线编译: export GOMODCACHE=\"$dest\" GOPROXY=off"
}

echo "StarCrystal 离线安装 (offlinesofts)"
echo "  ROOT=$OFFLINE_SOFTS_ROOT"
echo "  REPO=$REPO_ROOT"
echo ""

install_runtime_debs
install_mysql
install_redis
install_go
install_gomod_cache

echo ""
echo "一键完成环境: cd $RELEASE_ROOT && bash install-linux.sh"
echo "或分步: dbscripts/startdb.sh → dbscripts/mysql/bootstrap-mysql-auth.sh → dbscripts/mysql/rebuild-auth-mysql.sh → startsvr.sh"
