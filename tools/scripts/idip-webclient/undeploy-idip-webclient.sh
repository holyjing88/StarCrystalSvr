#!/usr/bin/env bash
# 兼容旧路径 tools/scripts/idip-webclient/undeploy-idip-webclient.sh
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
TARGET="$(cd "$HERE/../../idip-webclient/scripts" && pwd)/undeploy-idip-webclient.sh"
[[ -f "$TARGET" ]] || { echo "missing $TARGET" >&2; exit 1; }
exec bash "$TARGET" "$@"
