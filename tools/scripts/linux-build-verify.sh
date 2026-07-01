#!/usr/bin/env bash
set -euo pipefail
REPO=/home/holyjing/starcrystalsvr
cd "$REPO"
export PATH=$REPO/.go-toolchain/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
echo "[linux-build-verify] $(uname -m) $(go version)"
go mod tidy
GOOS=linux GOARCH=amd64 go build -o release/starcrystalsvr ./cmd/api
file release/starcrystalsvr
go test ./internal/service/... -count=1 -short
echo "[linux-build-verify] OK"
