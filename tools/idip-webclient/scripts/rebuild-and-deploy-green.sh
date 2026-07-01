#!/bin/bash
# idip-webclient 绿色重新构建并部署（仅 Linux）
#
# 用途：npm build（或 --skip-build 用已有 dist）→ 原子替换静态 → reload nginx
#
# 用法（在 starcrystalsvr 仓库内）：
#   bash tools/idip-webclient/scripts/rebuild-and-deploy-green.sh
#   bash tools/idip-webclient/scripts/rebuild-and-deploy-green.sh --skip-smoke
#   bash tools/idip-webclient/scripts/rebuild-and-deploy-green.sh --skip-build   # 无 Node18 时常用
#
# CentOS 7 等无 Node 18+：在 Windows 构建 dist/ 后同步到本仓库，再执行本脚本（会自动跳过 npm build）。   bash tools/idip-webclient/scripts/rebuild-and-deploy-green.sh --init         # 首次完整 green deploy
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
WEBCLIENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DEPLOY_SCRIPT="$SCRIPT_DIR/deploy-idip-webclient.sh"

log() { echo "[rebuild-deploy-green] $*"; }
warn() { echo "[rebuild-deploy-green] WARN: $*" >&2; }
die() { echo "[rebuild-deploy-green] FAIL: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

# Windows 检出 CRLF 会导致 ": No such file or directory" / $'\r': command not found
strip_crlf_file() {
  local f="$1"
  [[ -f "$f" ]] || return 0
  if grep -q $'\r' "$f" 2>/dev/null; then
    sed -i 's/\r$//' "$f"
    log "stripped CRLF: $f"
  fi
}

safe_source() {
  local f="$1"
  [[ -f "$f" ]] || return 0
  strip_crlf_file "$f"
  # shellcheck disable=SC1090
  source "$f"
}

strip_crlf_file "${BASH_SOURCE[0]:-$0}"
strip_crlf_file "$SCRIPT_DIR/deploy-env.local.example.sh"
strip_crlf_file "$SCRIPT_DIR/deploy-env.local.sh"
strip_crlf_file "$SCRIPT_DIR/deploy-idip-webclient.sh"

[[ -f "$WEBCLIENT_DIR/package.json" ]] || die "package.json not found: $WEBCLIENT_DIR"
[[ -f "$DEPLOY_SCRIPT" ]] || die "missing $DEPLOY_SCRIPT"

REPO_ROOT="${REPO_ROOT:-}"
if [[ -z "$REPO_ROOT" ]]; then
  if [[ -d "$WEBCLIENT_DIR/../../release" || -d "$WEBCLIENT_DIR/../../tools/nginx" ]]; then
    REPO_ROOT="$(cd "$WEBCLIENT_DIR/../.." && pwd)"
  else
    REPO_ROOT="$WEBCLIENT_DIR"
  fi
fi

SKIP_SMOKE=0
WITH_REGRESSION=0
FORCE_INIT=0
SKIP_BUILD=0

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-smoke) SKIP_SMOKE=1; shift ;;
      --skip-build) SKIP_BUILD=1; shift ;;
      --with-regression) WITH_REGRESSION=1; shift ;;
      --init|--first-time) FORCE_INIT=1; shift ;;
      -h|--help)
        sed -n '2,14p' "$0"
        exit 0
        ;;
      --) shift; break ;;
      -*) die "unknown option: $1 (try --help)" ;;
      *) break ;;
    esac
  done
}

parse_args "$@"

nginx_system_root() {
  local conf root_line root_path
  for conf in /etc/nginx/conf.d/starcrystal-idip.conf /etc/nginx/conf.d/starcrystal.conf /etc/nginx/conf.d/starcrystal-http.conf; do
    [[ -f "$conf" ]] || continue
    root_line="$(grep -E '^\s*root\s+' "$conf" 2>/dev/null | head -1 || true)"
    [[ -n "$root_line" ]] || continue
    root_path="$(echo "$root_line" | sed -E 's/^[[:space:]]*root[[:space:]]+([^;]+);.*/\1/')"
    root_path="${root_path//\'/}"
    root_path="${root_path//\"/}"
    root_path="${root_path//;/}"
    root_path="$(echo "$root_path" | awk '{$1=$1;print}')"
    if [[ -n "$root_path" ]]; then
      echo "$root_path"
      return 0
    fi
  done
  return 1
}

load_deploy_env() {
  safe_source "$SCRIPT_DIR/deploy-env.local.example.sh"
  safe_source "$SCRIPT_DIR/deploy-env.local.sh"

  GREEN_DEPLOY="${GREEN_DEPLOY:-1}"
  SVRIP="${SVRIP:-${API_BACKEND_HOST:-127.0.0.1}}"
  API_BACKEND_HOST="$SVRIP"
  API_BACKEND_PORT="${API_BACKEND_PORT:-8080}"
  WEB_WAN_IP="${WEB_WAN_IP:-${WAN_IP:-${CLOUD_PUBLIC_HOST:-}}}"
  VERIFY_HOST="${VERIFY_HOST:-${WEB_WAN_IP:-$( (hostname -I 2>/dev/null || true) | awk '{print $1}')}}"
  VERIFY_HOST="${VERIFY_HOST:-127.0.0.1}"
  WEB_WAN_IP="${WEB_WAN_IP:-$VERIFY_HOST}"
  ENABLE_HTTPS="${ENABLE_HTTPS:-0}"

  if [[ $EUID -ne 0 ]]; then
    WEB_HTTP_PORT="${WEB_HTTP_PORT:-8088}"
    WEB_HTTPS_PORT="${WEB_HTTPS_PORT:-8443}"
  else
    WEB_HTTP_PORT="${WEB_HTTP_PORT:-80}"
    WEB_HTTPS_PORT="${WEB_HTTPS_PORT:-443}"
  fi

  DEPLOY_ROOT="${DEPLOY_ROOT:-$REPO_ROOT/.deploy/idip-webclient}"
  NGINX_PREFIX="${NGINX_PREFIX:-$DEPLOY_ROOT/nginx}"

  local info="$DEPLOY_ROOT/deploy-info.env"
  if [[ -f "$info" ]]; then
    log "source deploy state: $info"
    safe_source "$info"
  fi

  # 静态目录：系统 nginx conf > deploy-info > 绿色默认
  if WEB_ROOT="$(nginx_system_root 2>/dev/null)"; then
    log "WEB_ROOT from system nginx: $WEB_ROOT"
  else
    WEB_ROOT="${WEB_ROOT:-$DEPLOY_ROOT/www}"
  fi

  if [[ "${ENABLE_HTTPS}" == "1" ]]; then
    if [[ "${WEB_HTTPS_PORT}" == "443" ]]; then
      IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-https://${WEB_WAN_IP}}"
    else
      IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-https://${WEB_WAN_IP}:${WEB_HTTPS_PORT}}"
    fi
  else
    if [[ "${WEB_HTTP_PORT}" == "80" ]]; then
      IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-http://${WEB_WAN_IP}}"
    else
      IDIP_WEBCLIENT_URL="${IDIP_WEBCLIENT_URL:-http://${WEB_WAN_IP}:${WEB_HTTP_PORT}}"
    fi
  fi

  export GREEN_DEPLOY REPO_ROOT WEBCLIENT_DIR DEPLOY_ROOT NGINX_PREFIX WEB_ROOT
  export SVRIP API_BACKEND_HOST API_BACKEND_PORT WEB_WAN_IP VERIFY_HOST ENABLE_HTTPS
  export WEB_HTTP_PORT WEB_HTTPS_PORT IDIP_WEBCLIENT_URL
}

has_node_18() {
  command -v node >/dev/null 2>&1 || return 1
  local major
  major="$(node -p "process.versions.node.split('.')[0]" 2>/dev/null || echo 0)"
  [[ "$major" -ge 18 ]]
}

build_webclient() {
  cd "$WEBCLIENT_DIR"
  if [[ "$SKIP_BUILD" == "1" ]]; then
    [[ -f dist/index.html ]] || die "dist/index.html missing — build on dev machine (Windows Node 18+) then sync dist/"
    log "SKIP_BUILD=1 — use existing $WEBCLIENT_DIR/dist"
    return 0
  fi
  if ! has_node_18; then
    if [[ -f dist/index.html ]]; then
      warn "Node.js 18+ not found on this host — using existing dist/ (build on Windows if UI is stale)"
      return 0
    fi
    die "Node.js 18+ required and dist/index.html missing — on Windows: cd tools/idip-webclient && npm install && npm run build"
  fi
  log "node $(node -v)"
  if [[ ! -d node_modules/vite ]]; then
    log "npm install ..."
    npm install
  fi
  log "npm run build ..."
  npm run build
  [[ -f dist/index.html ]] || die "build failed: dist/index.html missing"
  log "build OK → $WEBCLIENT_DIR/dist"
}

fix_nginx_read_access() {
  local root="$1"
  [[ $EUID -eq 0 ]] || return 0
  local d="$root"
  while [[ "$d" != "/" && -n "$d" ]]; do
    chmod a+x "$d" 2>/dev/null || true
    d="$(dirname "$d")"
  done
  chmod -R a+rX "$root" 2>/dev/null || true
  if id nginx >/dev/null 2>&1; then
    chown -R nginx:nginx "$root" 2>/dev/null || true
  elif id www-data >/dev/null 2>&1; then
    chown -R www-data:www-data "$root" 2>/dev/null || true
  fi
  if command -v getenforce >/dev/null 2>&1 && [[ "$(getenforce 2>/dev/null)" == "Enforcing" ]]; then
    chcon -R -t httpd_sys_content_t "$root" 2>/dev/null || true
  fi
}

install_static_atomic() {
  need_cmd mkdir
  need_cmd cp
  need_cmd mv
  need_cmd rm

  local staging="${WEB_ROOT}.staging.$$"
  mkdir -p "$staging"
  cp -a "$WEBCLIENT_DIR/dist/." "$staging/"
  fix_nginx_read_access "$staging"

  mkdir -p "$(dirname "$WEB_ROOT")"
  if [[ -d "$WEB_ROOT" ]]; then
    rm -rf "${WEB_ROOT}.old" 2>/dev/null || true
    mv "$WEB_ROOT" "${WEB_ROOT}.old"
  fi
  mv "$staging" "$WEB_ROOT"
  rm -rf "${WEB_ROOT}.old" 2>/dev/null || true
  log "static atomically updated → $WEB_ROOT"
}

green_nginx_running() {
  local pid_file="$NGINX_PREFIX/logs/nginx.pid"
  [[ -f "$pid_file" ]] || return 1
  kill -0 "$(cat "$pid_file")" 2>/dev/null
}

reload_green_nginx() {
  local conf="$NGINX_PREFIX/conf/nginx.conf"
  [[ -f "$conf" ]] || return 1
  command -v nginx >/dev/null 2>&1 || die "nginx not found"
  nginx -p "$NGINX_PREFIX" -t -c conf/nginx.conf
  nginx -p "$NGINX_PREFIX" -s reload
  log "green nginx reloaded (prefix=$NGINX_PREFIX)"
}

reload_system_nginx() {
  command -v nginx >/dev/null 2>&1 || die "nginx not found"
  nginx -t || die "nginx -t failed"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl reload nginx
  else
    nginx -s reload
  fi
  log "system nginx reloaded"
}

reload_web_server() {
  if green_nginx_running; then
    reload_green_nginx
    return 0
  fi
  if command -v systemctl >/dev/null 2>&1 && systemctl is-active nginx >/dev/null 2>&1; then
    reload_system_nginx
    return 0
  fi
  if [[ -f "$NGINX_PREFIX/conf/nginx.conf" ]]; then
    log "starting green nginx ..."
    bash "$SCRIPT_DIR/idip-webclient-ctl.sh" start
    return 0
  fi
  warn "no nginx to reload — static updated only"
  return 0
}

run_full_green_deploy() {
  log "running full green deploy (--init) ..."
  local extra=(--green --force-build --skip-regression)
  [[ "$WITH_REGRESSION" == "1" ]] && extra=(--green --force-build)
  strip_crlf_file "$DEPLOY_SCRIPT"
  bash "$DEPLOY_SCRIPT" "${extra[@]}"
}

run_smoke() {
  [[ "$SKIP_SMOKE" == "1" ]] && { log "SKIP_SMOKE=1"; return 0; }
  export IDIP_WEBCLIENT_URL VERIFY_HOST ENABLE_HTTPS WEB_HTTP_PORT WEB_HTTPS_PORT
  strip_crlf_file "$SCRIPT_DIR/verify-cloud.sh"
  bash "$SCRIPT_DIR/verify-cloud.sh"
}

run_regression_optional() {
  [[ "$WITH_REGRESSION" != "1" ]] && return 0
  has_node_18 || { warn "skip regression (Node 18+ missing)"; return 0; }
  export IDIP_BASE_URL="${IDIP_BASE_URL:-http://${SVRIP}:${API_BACKEND_PORT}}"
  bash "$SCRIPT_DIR/run-regression.sh"
}

print_summary() {
  log "OK — idip-webclient redeploy complete"
  log "  URL:       ${IDIP_WEBCLIENT_URL}/"
  log "  WEB_ROOT:  $WEB_ROOT"
  log "  API:       http://${SVRIP}:${API_BACKEND_PORT}"
  log "  浏览器 Ctrl+F5 强刷"
}

main() {
  load_deploy_env
  log "REPO_ROOT=$REPO_ROOT"
  log "WEBCLIENT_DIR=$WEBCLIENT_DIR"
  log "WEB_ROOT=$WEB_ROOT NGINX_PREFIX=$NGINX_PREFIX"

  if [[ "$FORCE_INIT" == "1" ]]; then
    run_full_green_deploy
    print_summary
    return 0
  fi

  build_webclient
  install_static_atomic
  reload_web_server
  run_smoke
  run_regression_optional
  print_summary
}

main "$@"
