#!/usr/bin/env bash
# 官网绿色 nginx 启停
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
INFO="${DEPLOY_ROOT:-/wwwroot/OfficialSite}/deploy-info.env"

usage() {
  echo "Usage: $0 {start|stop|restart|status}"
  exit 1
}

[[ $# -ge 1 ]] || usage
if [[ -f "$INFO" ]]; then
  # shellcheck disable=SC1090
  source "$INFO"
fi

NGINX_PREFIX="${NGINX_PREFIX:-/wwwroot/OfficialSite/nginx}"
PID_FILE="$NGINX_PREFIX/logs/nginx.pid"
NGINX_CONF="$NGINX_PREFIX/conf/nginx.conf"

case "${1:-}" in
  start)
    command -v nginx >/dev/null 2>&1 || { echo "nginx not found"; exit 1; }
    [[ -f "$NGINX_CONF" ]] || { echo "missing $NGINX_CONF — run deploy-official-site.sh"; exit 1; }
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "already running pid=$(cat "$PID_FILE")"
      exit 0
    fi
    nginx -p "$NGINX_PREFIX" -t -c conf/nginx.conf
    nginx -p "$NGINX_PREFIX" -c conf/nginx.conf
    echo "started pid=$(cat "$PID_FILE")"
    ;;
  stop)
    if [[ -f "$PID_FILE" ]]; then
      nginx -p "$NGINX_PREFIX" -s quit 2>/dev/null || kill "$(cat "$PID_FILE")" 2>/dev/null || true
      echo "stopped"
    else
      echo "not running"
    fi
    ;;
  restart)
    "$0" stop
    sleep 1
    "$0" start
    ;;
  status)
    if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "running pid=$(cat "$PID_FILE") url=${SITE_URL:-}"
    else
      echo "not running"
      exit 1
    fi
    ;;
  *)
    usage
    ;;
esac
