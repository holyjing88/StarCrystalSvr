#!/usr/bin/env bash
# 在可联网的 Linux x86_64 上执行：下载 MySQL / Redis 源码 / Go / go mod 缓存 / 可选 .deb 到本目录各子文件夹。
# 用法:
#   cd server-go && bash tools/offlinesofts/fetch-linux-offline-packages.sh
# 环境变量: SKIP_GO=1 | SKIP_GOMOD=1 | SKIP_RUNTIME_DEB=1 | MYSQL_VERSION | REDIS_VERSION | GO_VERSION
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=lib/offline-common.sh
source "$HERE/lib/offline-common.sh"

mkdir -p \
  "$OFFLINE_SOFTS_ROOT/mysql" \
  "$OFFLINE_SOFTS_ROOT/redis" \
  "$OFFLINE_SOFTS_ROOT/go" \
  "$OFFLINE_SOFTS_ROOT/go-modules" \
  "$OFFLINE_SOFTS_ROOT/runtime/deb"

dl() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "[skip] 已存在: $(basename "$dest")"
    return 0
  fi
  echo "[fetch] $url"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 30 -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$dest" "$url"
  else
    echo "需要 curl 或 wget" >&2
    exit 1
  fi
}

echo "==> MySQL ${MYSQL_VERSION}"
dl "$MYSQL_URL" "$OFFLINE_SOFTS_ROOT/mysql/$MYSQL_ARCHIVE"

echo "==> Redis ${REDIS_VERSION} (源码，离线机本地编译)"
dl "$REDIS_URL" "$OFFLINE_SOFTS_ROOT/redis/$REDIS_ARCHIVE"

if [[ "${SKIP_GO:-0}" != "1" ]]; then
  echo "==> Go ${GO_VERSION}"
  dl "$GO_URL" "$OFFLINE_SOFTS_ROOT/go/$GO_ARCHIVE"
fi

if [[ "${SKIP_GOMOD:-0}" != "1" ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "[warn] 未安装 go，跳过 go-modules（可设 SKIP_GOMOD=1 或先安装 Go）"
  else
    echo "==> go mod download 缓存"
    tmpcache="$OFFLINE_SOFTS_ROOT/go-modules/.cache-build"
    rm -rf "$tmpcache"
    mkdir -p "$tmpcache"
    export GOMODCACHE="$tmpcache"
    export GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}"
    (cd "$REPO_ROOT" && go mod download)
    tar -czf "$OFFLINE_SOFTS_ROOT/go-modules/gomod-cache.tar.gz" -C "$tmpcache" .
    rm -rf "$tmpcache"
    echo "  -> go-modules/gomod-cache.tar.gz"
  fi
fi

if [[ "${SKIP_RUNTIME_DEB:-0}" != "1" ]] && command -v apt-get >/dev/null 2>&1; then
  echo "==> 运行时 .deb（需 apt，可选）"
  deb_dir="$OFFLINE_SOFTS_ROOT/runtime/deb"
  pkgs=(libaio1 libncurses6 libtinfo6 numactl)
  if [[ "${FETCH_BUILD_DEB:-0}" == "1" ]]; then
    pkgs+=(gcc make build-essential)
  fi
  (
    cd "$deb_dir"
    for p in "${pkgs[@]}"; do
      echo "  apt-get download $p"
      apt-get download "$p" 2>/dev/null || echo "  [warn] 无法下载 $p（换发行版或手动放入 runtime/deb/）"
    done
  )
fi

echo ""
echo "完成。请将整个 offlinesofts/ 目录打包拷贝到离线机，然后执行:"
echo "  cd server-go/release && bash ../tools/offlinesofts/install-linux-offline.sh"
ls -la "$OFFLINE_SOFTS_ROOT/mysql" "$OFFLINE_SOFTS_ROOT/redis" 2>/dev/null || true
