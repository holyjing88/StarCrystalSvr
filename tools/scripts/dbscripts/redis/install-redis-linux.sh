#!/usr/bin/env bash
# 在 Linux 或 WSL 中编译 redis-server / redis-cli 到 dbscripts/redis/linux/。
# 用法:
#   bash tools/scripts/dbscripts/redis/install-redis-linux.sh
#   REDIS_VERSION=7.2.6 bash tools/scripts/dbscripts/redis/install-redis-linux.sh
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
DBSCRIPTS_ROOT="$(cd "$HERE/.." && pwd)"
DEST="$HERE/linux"
TMP="${TMPDIR:-/tmp}/starcrystal-redis-build-$$"

for c in gcc make curl tar; do
  if ! command -v "$c" >/dev/null 2>&1; then
    echo "缺少命令: $c。请先安装 build-essential / curl（例如 Debian: sudo apt install build-essential curl）。" >&2
    exit 1
  fi
done

mkdir -p "$DEST" "$TMP"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

VERSION="${REDIS_VERSION:-7.2.6}"
ARCHIVE="redis-${VERSION}.tar.gz"
URL="https://download.redis.io/releases/${ARCHIVE}"
echo "[install-redis-linux] 下载 $URL"
curl -fsSL "$URL" -o "$TMP/$ARCHIVE"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
SRC="$TMP/redis-${VERSION}"

echo "[install-redis-linux] 编译（BUILD_TLS=no，约 1–3 分钟）…"
(
  cd "$SRC"
  make -j"${NPROC:-$(nproc 2>/dev/null || echo 4)}" BUILD_TLS=no
)

install -m0755 "$SRC/src/redis-server" "$DEST/redis-server"
install -m0755 "$SRC/src/redis-cli" "$DEST/redis-cli"

echo "[install-redis-linux] 已安装到 dbscripts/redis/linux/:"
ls -la "$DEST/redis-server" "$DEST/redis-cli"
echo "[install-redis-linux] 启动: REDIS_SERVER_EXE=$DEST/redis-server bash $DBSCRIPTS_ROOT/redis/redis-start.sh"
