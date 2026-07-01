#!/usr/bin/env bash
# 在 Linux 宿主机上执行 pack-publish + verify（通过 tools/scripts/ssh-linux.sh）
# 用法（Windows Git Bash / Linux）:
#   bash tools/0publish/scripts/pack-publish-linux-remote.sh
#   bash tools/0publish/scripts/pack-publish-linux-remote.sh --skip-build
set -euo pipefail

SKIP_BUILD=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build) SKIP_BUILD=1 ;;
    -h|--help)
      echo "Usage: $0 [--skip-build]" >&2
      exit 0
      ;;
    *) echo "Unknown: $1" >&2; exit 1 ;;
  esac
  shift
done

HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
SSH="$HERE/../../scripts/ssh-linux.sh"
[[ -x "$SSH" ]] || SSH="$HERE/../../scripts/ssh-linux.sh"

ENV_FILE="$HERE/../../linux-host.local.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE（从 linux-host.env.example 复制）" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"
: "${LINUX_REPO:=/home/holyjing/starcrystalsvr}"

skip_flag=""
[[ "$SKIP_BUILD" -eq 1 ]] && skip_flag="--skip-build"

remote_cmd="set -euo pipefail
export LANG=C.UTF-8
cd '$LINUX_REPO'
svn update -q tools/0publish/scripts tools/scripts/starcrystal-server-root.sh 2>/dev/null || true
bash tools/0publish/scripts/pack-publish.sh $skip_flag
bash tools/0publish/scripts/pack-publish-verify.sh --latest
tag=\$(ls -1t tools/0publish | grep -E '^[0-9]{8}-[0-9]{6}\$' | head -1)
echo \"--- subdir tools/0publish/\$tag ---\"
ls -la \"tools/0publish/\$tag\"
echo \"--- bundle ---\"
ls -lh \"tools/0publish/\${tag}.tar.gz\"
"

echo "==> remote pack-publish on ${LINUX_USER}@${LINUX_HOST}:$LINUX_REPO"
bash "$SSH" "$remote_cmd"
