#!/usr/bin/env bash
# 构建 idip-webclient/scripts_encrypt/bin/encrypt-idip-operator（Linux amd64 静态二进制）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${SCRIPT_DIR}/bin/encrypt-idip-operator"
SRC="${SCRIPT_DIR}/encrypt"

mkdir -p "${SCRIPT_DIR}/bin"
export GOOS="${GOOS:-linux}"
export GOARCH="${GOARCH:-amd64}"
export CGO_ENABLED=0

go build -trimpath -ldflags="-s -w" -o "${OUT}" "${SRC}"
chmod +x "${OUT}"
if command -v gzip >/dev/null 2>&1; then
  gzip -9 -c "${OUT}" > "${OUT}.gz"
  echo "Packed ${OUT}.gz"
fi
echo "Built ${OUT} (${GOOS}/${GOARCH})"
