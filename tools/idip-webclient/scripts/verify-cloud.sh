#!/usr/bin/env bash
# 部署后冒烟：index.html、/idip 代理、/healthz
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
DEPLOY_ENV="$SCRIPT_DIR/deploy-env.local.sh"
if [[ -f "$DEPLOY_ENV" ]]; then
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi

VERIFY_HOST="${VERIFY_HOST:-${CLOUD_PUBLIC_HOST:-127.0.0.1}}"
ENABLE_HTTPS="${ENABLE_HTTPS:-0}"
IDIP_KEY="${IDIP_KEY:-change-me-in-production}"

echo "[verify-cloud] VERIFY_HOST=$VERIFY_HOST ENABLE_HTTPS=$ENABLE_HTTPS"

if [[ "${ENABLE_HTTPS}" == "1" ]]; then
  BASE="https://${VERIFY_HOST}"
  curl -fsS -k "$BASE/" | grep -qiE 'StarCrystal|IDIP|运营' \
    || { echo "FAIL: index.html"; exit 1; }
else
  BASE="http://${VERIFY_HOST}"
  curl -fsS "$BASE/" | grep -qiE 'StarCrystal|IDIP|运营' \
    || { echo "FAIL: index.html"; exit 1; }
fi
echo "OK index"

if [[ "${ENABLE_HTTPS}" == "1" ]]; then
  curl -fsS -k -H "X-IDIP-Key: $IDIP_KEY" \
    "$BASE/idip/v1/tasks/definitions" | grep -q '"code"' \
    || { echo "FAIL: /idip proxy"; exit 1; }
else
  curl -fsS -H "X-IDIP-Key: $IDIP_KEY" \
    "$BASE/idip/v1/tasks/definitions" | grep -q '"code"' \
    || { echo "FAIL: /idip proxy"; exit 1; }
fi
echo "OK /idip"

if [[ "${ENABLE_HTTPS}" == "1" ]]; then
  curl -fsS -k "$BASE/healthz" | grep -q '"ok"' \
    || { echo "FAIL: /healthz"; exit 1; }
else
  curl -fsS "$BASE/healthz" | grep -q '"ok"' \
    || { echo "FAIL: /healthz"; exit 1; }
fi
echo "OK /healthz"

echo "[verify-cloud] OK BASE=$BASE"
