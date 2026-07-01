#!/usr/bin/env bash
# Vitest 回归（需 Node 18+ 与 npm install）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
WEBCLIENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$WEBCLIENT_DIR"

DEPLOY_ENV="$SCRIPT_DIR/deploy-env.local.sh"
if [[ -f "$DEPLOY_ENV" ]]; then
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi

API_BACKEND_HOST="${API_BACKEND_HOST:-127.0.0.1}"
API_BACKEND_PORT="${API_BACKEND_PORT:-8080}"
VERIFY_HOST="${VERIFY_HOST:-${CLOUD_PUBLIC_HOST:-127.0.0.1}}"
ENABLE_HTTPS="${ENABLE_HTTPS:-0}"

export IDIP_BASE_URL="${IDIP_BASE_URL:-http://${API_BACKEND_HOST}:${API_BACKEND_PORT}}"
if [[ "${ENABLE_HTTPS}" == "1" ]]; then
  export IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-https://${VERIFY_HOST}}"
  export NODE_TLS_REJECT_UNAUTHORIZED="${NODE_TLS_REJECT_UNAUTHORIZED:-0}"
else
  export IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-http://${VERIFY_HOST}}"
fi
export IDIP_KEY="${IDIP_KEY:-change-me-in-production}"
export IDIP_USERNAME="${IDIP_USERNAME:-holyjing}"
export IDIP_PASSWORD="${IDIP_PASSWORD:-jgyjgyjgy}"

echo "[run-regression] IDIP_BASE_URL=$IDIP_BASE_URL"
echo "[run-regression] IDIP_WEBCLIENT_URL=$IDIP_WEBCLIENT_URL"

if [[ ! -d node_modules/vitest ]]; then
  echo "[run-regression] npm install ..."
  npm install
fi

npm run regression
