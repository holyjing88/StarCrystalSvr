#!/usr/bin/env bash
# 在 fetch 完成后打包整个 offlinesofts 目录，便于 U 盘 / scp 到离线机。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
OUT="${1:-$HERE/../starcrystal-offlinesofts-linux-amd64.tar.gz}"
parent="$(dirname "$HERE")"
name="$(basename "$HERE")"
echo "Creating $OUT"
tar -czf "$OUT" -C "$parent" "$name"
ls -lh "$OUT"
echo "离线机解压: tar -xzf $(basename "$OUT") -C server-go/tools"
