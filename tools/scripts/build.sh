#!/usr/bin/env bash
# Build starcrystalsvr into release/ (run from anywhere).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=starcrystal-config.sh
source "$SCRIPT_DIR/starcrystal-config.sh"
cd "$REPO_ROOT"
OUT="$RELEASE_ROOT/starcrystalsvr"
sc_setup_go_env
if ! command -v go >/dev/null 2>&1; then
  echo "go: command not found" >&2
  echo "Install Go or unpack go*.linux-amd64.tar.gz via tools/offlinesofts/install-linux-offline.sh" >&2
  echo "Then: export PATH=\"\${STARCRYSTAL_GO_ROOT:-$REPO_ROOT/.go-toolchain/go}/bin:\$PATH\"" >&2
  exit 127
fi
if [[ -z "${GOPROXY:-}" ]]; then
  export GOPROXY='https://goproxy.cn,https://proxy.golang.org,direct'
fi
echo "go mod tidy ($REPO_ROOT)"
go mod tidy
echo "go build -> $OUT"
go build -o "$OUT" ./cmd/api
echo "Build complete: $OUT"
