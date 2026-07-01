#!/usr/bin/env bash
# StarCrystal idip-webclient 一键部署（任意 Linux 主机，支持绿色部署）
#
# 典型场景：Web/nginx 在本机，游戏 API 在内网 SVRIP；外网用户通过 WEB_WAN_IP 访问运营台。
#
# 无参数（推荐）：自动加载同目录 deploy-env.local.example.sh 中的默认配置
#   bash tools/idip-webclient/scripts/deploy-idip-webclient.sh
#
# 绿色部署（环境变量覆盖）：
#   GREEN_DEPLOY=1 SVRIP=192.168.75.99 WEB_WAN_IP=203.0.113.10 \
#     bash tools/idip-webclient/scripts/deploy-idip-webclient.sh
#
# 命令行：
#   bash deploy-idip-webclient.sh --green --svrip 192.168.75.99 --wan-ip 203.0.113.10
#
# 系统 nginx（需 root，写入 /etc/nginx/conf.d）：
#   sudo GREEN_DEPLOY=0 SVRIP=192.168.75.99 WEB_WAN_IP=203.0.113.10 \
#     bash tools/idip-webclient/scripts/deploy-idip-webclient.sh
#
# 环境变量：
#   SVRIP / API_BACKEND_HOST     内网 API 地址（nginx upstream，默认 127.0.0.1）
#   API_BACKEND_PORT             API 端口（默认 8080）
#   WEB_WAN_IP / WAN_IP          外网 IP 或域名（浏览器访问、冒烟测试）
#   VERIFY_HOST                  同 WEB_WAN_IP（未设时自动探测本机 LAN IP）
#   GREEN_DEPLOY                 1=仓库 .deploy 前缀 nginx；0=系统 nginx
#   ENABLE_HTTPS                 1=HTTPS，0=仅 HTTP
#   WEB_HTTP_PORT / WEB_HTTPS_PORT  监听端口（非 root 默认 8088/8443）
#   IDIP_BASE_URL                Vitest 直连 API（默认 http://SVRIP:8080）
#   SKIP_REGRESSION=1            跳过 Vitest
#
set -euo pipefail

log() { echo "[deploy-idip-webclient] $*"; }
die() { echo "[deploy-idip-webclient] FAIL: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

# ---- 路径探测 ----
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
WEBCLIENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
[[ -f "$WEBCLIENT_DIR/package.json" ]] || die "package.json not found under $WEBCLIENT_DIR"

REPO_ROOT="${REPO_ROOT:-}"
if [[ -z "$REPO_ROOT" ]]; then
  if [[ -d "$WEBCLIENT_DIR/../../release" || -d "$WEBCLIENT_DIR/../../tools/nginx" ]]; then
    REPO_ROOT="$(cd "$WEBCLIENT_DIR/../.." && pwd)"
  else
    REPO_ROOT="$WEBCLIENT_DIR"
  fi
fi

# ---- 命令行参数（先于 deploy-env.local.sh，便于 --help）----
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --green) GREEN_DEPLOY=1; shift ;;
      --system-nginx) GREEN_DEPLOY=0; shift ;;
      --svrip) SVRIP="${2:-}"; shift 2 ;;
      --wan-ip|--web-wan-ip) WEB_WAN_IP="${2:-}"; shift 2 ;;
      --https) ENABLE_HTTPS=1; shift ;;
      --http) ENABLE_HTTPS=0; shift ;;
      --skip-regression) SKIP_REGRESSION=1; shift ;;
      --force-build) FORCE_NPM_BUILD=1; shift ;;
      -h|--help)
        sed -n '2,28p' "$0"
        exit 0
        ;;
      --) shift; break ;;
      -*) die "unknown option: $1 (try --help)" ;;
      *) break ;;
    esac
  done
}

HAS_CLI_ARGS=0
[[ $# -gt 0 ]] && HAS_CLI_ARGS=1
parse_args "$@"

DEFAULT_ENV="$SCRIPT_DIR/deploy-env.local.example.sh"
if [[ "$HAS_CLI_ARGS" -eq 0 && -f "$DEFAULT_ENV" ]]; then
  log "no CLI args — source defaults: $DEFAULT_ENV"
  # shellcheck disable=SC1090
  source "$DEFAULT_ENV"
fi

DEPLOY_ENV="$SCRIPT_DIR/deploy-env.local.sh"
if [[ -f "$DEPLOY_ENV" ]]; then
  log "source $DEPLOY_ENV"
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi

# ---- 网络拓扑：SVRIP（内网 API） + WEB_WAN_IP（外网 Web）----
SVRIP="${SVRIP:-${API_BACKEND_HOST:-127.0.0.1}}"
API_BACKEND_HOST="$SVRIP"
API_BACKEND_PORT="${API_BACKEND_PORT:-8080}"

WEB_WAN_IP="${WEB_WAN_IP:-${WAN_IP:-${CLOUD_PUBLIC_HOST:-}}}"
detect_lan_ip() {
  (hostname -I 2>/dev/null || true) | awk '{print $1}'
}
VERIFY_HOST="${VERIFY_HOST:-${WEB_WAN_IP:-$(detect_lan_ip)}}"
VERIFY_HOST="${VERIFY_HOST:-127.0.0.1}"
WEB_WAN_IP="${WEB_WAN_IP:-$VERIFY_HOST}"
CLOUD_PUBLIC_HOST="${CLOUD_PUBLIC_HOST:-$WEB_WAN_IP}"

GREEN_DEPLOY="${GREEN_DEPLOY:-1}"
ENABLE_HTTPS="${ENABLE_HTTPS:-0}"
FORCE_NPM_BUILD="${FORCE_NPM_BUILD:-0}"
SKIP_REGRESSION="${SKIP_REGRESSION:-0}"
NGINX_CONF_NAME="${NGINX_CONF_NAME:-starcrystal-idip.conf}"

# 非 root 无法绑定 80/443
if [[ $EUID -ne 0 ]]; then
  WEB_HTTP_PORT="${WEB_HTTP_PORT:-8088}"
  WEB_HTTPS_PORT="${WEB_HTTPS_PORT:-8443}"
else
  WEB_HTTP_PORT="${WEB_HTTP_PORT:-80}"
  WEB_HTTPS_PORT="${WEB_HTTPS_PORT:-443}"
fi

DEPLOY_ROOT="${DEPLOY_ROOT:-$REPO_ROOT/.deploy/idip-webclient}"
NGINX_PREFIX="${NGINX_PREFIX:-$DEPLOY_ROOT/nginx}"
if [[ "${GREEN_DEPLOY}" == "1" ]]; then
  WEB_ROOT="${WEB_ROOT:-$DEPLOY_ROOT/www}"
else
  WEB_ROOT="${WEB_ROOT:-/var/www/starcrystal-idip/dist}"
fi

IDIP_BASE_URL="${IDIP_BASE_URL:-http://${SVRIP}:${API_BACKEND_PORT}}"
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

STARCrystal_CONFIG="${STARCrystal_CONFIG:-$REPO_ROOT/release/configs/starcrystal.json}"
if [[ ! -f "$STARCrystal_CONFIG" && -f "$REPO_ROOT/configs/starcrystal.json" ]]; then
  STARCrystal_CONFIG="$REPO_ROOT/configs/starcrystal.json"
fi

read_json_idip() {
  local cfg="$1"
  [[ -f "$cfg" ]] || return 0
  local py=""
  command -v python3 >/dev/null && py=python3
  command -v python >/dev/null && [[ -z "$py" ]] && py=python
  [[ -n "$py" ]] || { log "WARN: no python — skip reading $cfg"; return 0; }
  "$py" - "$cfg" <<'PY'
import json, sys
path = sys.argv[1]
try:
    with open(path, encoding="utf-8") as f:
        j = json.load(f)
except Exception:
    sys.exit(0)
idip = j.get("idip") or {}
key = idip.get("key") or ""
ops = idip.get("operators") or []
user = ops[0].get("username", "") if ops else ""
pwd = ops[0].get("password", "") if ops else ""
print(key)
print(user)
print(pwd)
PY
}

if [[ -f "$STARCrystal_CONFIG" ]]; then
  mapfile -t _idip_cfg < <(read_json_idip "$STARCrystal_CONFIG")
  [[ -z "${IDIP_KEY:-}" || "${IDIP_KEY}" == "change-me-in-production" ]] && [[ -n "${_idip_cfg[0]:-}" ]] && IDIP_KEY="${_idip_cfg[0]}"
  [[ -z "${IDIP_USERNAME:-}" ]] && [[ -n "${_idip_cfg[1]:-}" ]] && IDIP_USERNAME="${_idip_cfg[1]}"
  [[ -z "${IDIP_PASSWORD:-}" ]] && [[ -n "${_idip_cfg[2]:-}" ]] && IDIP_PASSWORD="${_idip_cfg[2]}"
fi

IDIP_KEY="${IDIP_KEY:-change-me-in-production}"
IDIP_USERNAME="${IDIP_USERNAME:-holyjing}"
IDIP_PASSWORD="${IDIP_PASSWORD:-jgyjgyjgy}"

export IDIP_BASE_URL IDIP_WEBCLIENT_URL IDIP_KEY IDIP_USERNAME IDIP_PASSWORD
export API_BACKEND_HOST API_BACKEND_PORT VERIFY_HOST WEB_WAN_IP SVRIP ENABLE_HTTPS GREEN_DEPLOY

log "WEBCLIENT_DIR=$WEBCLIENT_DIR"
log "REPO_ROOT=$REPO_ROOT"
log "GREEN_DEPLOY=$GREEN_DEPLOY"
log "SVRIP=$SVRIP:${API_BACKEND_PORT} (nginx → API 内网)"
log "WEB_WAN_IP=$WEB_WAN_IP (外网/浏览器访问)"
log "IDIP_BASE_URL=$IDIP_BASE_URL (Vitest/脚本直连 API)"
log "IDIP_WEBCLIENT_URL=$IDIP_WEBCLIENT_URL"
log "WEB_ROOT=$WEB_ROOT ports=${WEB_HTTP_PORT}/${WEB_HTTPS_PORT} ENABLE_HTTPS=$ENABLE_HTTPS"

# ---- Node / npm ----
has_node_18() {
  command -v node >/dev/null 2>&1 || return 1
  local major
  major="$(node -p "process.versions.node.split('.')[0]" 2>/dev/null || echo 0)"
  [[ "$major" -ge 18 ]]
}

ensure_node() {
  if has_node_18; then
    log "node $(node -v)"
    return 0
  fi
  if command -v node >/dev/null 2>&1; then
    log "node $(node -v) too old, need >=18"
  fi
  if [[ "${GREEN_DEPLOY}" == "0" ]] && [[ $EUID -eq 0 ]]; then
    if command -v dnf >/dev/null 2>&1; then
      log "install nodejs via dnf ..."
      dnf install -y nodejs npm || true
    elif command -v apt-get >/dev/null 2>&1; then
      log "install nodejs via apt ..."
      apt-get update -qq && apt-get install -y nodejs npm || true
    fi
  fi
  if has_node_18; then
    log "node $(node -v)"
    return 0
  fi
  die "Node.js 18+ required (or pre-build dist/ on dev machine)"
}

build_webclient() {
  cd "$WEBCLIENT_DIR"
  if [[ "$FORCE_NPM_BUILD" != "1" && -f dist/index.html ]]; then
    log "skip npm build (dist/index.html exists, set FORCE_NPM_BUILD=1 to rebuild)"
    return 0
  fi
  ensure_node
  if [[ ! -d node_modules/vite ]]; then
    log "npm install ..."
    npm install
  fi
  log "npm run build ..."
  npm run build
  [[ -f dist/index.html ]] || die "build failed: dist/index.html missing"
}

fix_nginx_read_access() {
  [[ $EUID -eq 0 ]] || return 0
  local d="$WEB_ROOT"
  while [[ "$d" != "/" && -n "$d" ]]; do
    chmod a+x "$d" 2>/dev/null || true
    d="$(dirname "$d")"
  done
  chmod -R a+rX "$WEB_ROOT" 2>/dev/null || true
  if id nginx >/dev/null 2>&1; then
    chown -R nginx:nginx "$WEB_ROOT" 2>/dev/null || true
  elif id www-data >/dev/null 2>&1; then
    chown -R www-data:www-data "$WEB_ROOT" 2>/dev/null || true
  fi
  if command -v getenforce >/dev/null 2>&1 && [[ "$(getenforce 2>/dev/null)" == "Enforcing" ]]; then
    chcon -R -t httpd_sys_content_t "$WEB_ROOT" 2>/dev/null || true
  fi
}

install_static() {
  need_cmd mkdir
  need_cmd cp
  mkdir -p "$WEB_ROOT"
  rm -rf "${WEB_ROOT:?}"/*
  cp -a "$WEBCLIENT_DIR/dist/." "$WEB_ROOT/"
  fix_nginx_read_access
  log "static → $WEB_ROOT"
}

pick_nginx_template() {
  local mode="${1:-system}"
  if [[ "$mode" == "green" ]]; then
    if [[ "${ENABLE_HTTPS}" == "1" ]]; then
      echo "$REPO_ROOT/tools/nginx/starcrystal-idip-green-https.conf"
    else
      echo "$REPO_ROOT/tools/nginx/starcrystal-idip-green-http.conf"
    fi
    return
  fi
  local http_tpl="$REPO_ROOT/tools/nginx/starcrystal-idip-http.conf"
  local https_tpl="$REPO_ROOT/tools/nginx/starcrystal-idip-https.conf"
  if [[ "${ENABLE_HTTPS}" == "1" && -f "$https_tpl" ]]; then
    echo "$https_tpl"
  elif [[ -f "$http_tpl" ]]; then
    echo "$http_tpl"
  else
    die "nginx template not found under $REPO_ROOT/tools/nginx"
  fi
}

https_port_suffix() {
  if [[ "${WEB_HTTPS_PORT}" == "443" ]]; then
    echo ""
  else
    echo ":${WEB_HTTPS_PORT}"
  fi
}

ensure_ssl_cert() {
  [[ "${ENABLE_HTTPS}" == "1" ]] || return 0
  need_cmd openssl
  local ssl_dir="$1"
  mkdir -p "$ssl_dir"
  if [[ ! -f "$ssl_dir/starcrystal.crt" ]]; then
    log "generate self-signed TLS cert CN=$WEB_WAN_IP"
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
      -keyout "$ssl_dir/starcrystal.key" \
      -out "$ssl_dir/starcrystal.crt" \
      -subj "/CN=${WEB_WAN_IP}"
  fi
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | grep -q ":${port} "
  elif command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | grep -q ":${port} "
  else
    return 1
  fi
}

render_nginx_from_tpl() {
  local tpl="$1" out="$2" ssl_dir="$3"
  sed \
    -e "s|__NGINX_PREFIX__|${NGINX_PREFIX}|g" \
    -e "s|__API_BACKEND_HOST__|${API_BACKEND_HOST}|g" \
    -e "s|__API_BACKEND_PORT__|${API_BACKEND_PORT}|g" \
    -e "s|__WEB_ROOT__|${WEB_ROOT}|g" \
    -e "s|__HTTP_PORT__|${WEB_HTTP_PORT}|g" \
    -e "s|__HTTPS_PORT__|${WEB_HTTPS_PORT}|g" \
    -e "s|__HTTPS_PORT_SUFFIX__|$(https_port_suffix)|g" \
    -e "s|__SSL_CERT__|${ssl_dir}/starcrystal.crt|g" \
    -e "s|__SSL_KEY__|${ssl_dir}/starcrystal.key|g" \
    "$tpl" >"$out"
}

render_green_nginx() {
  local tpl="$1"
  local out="$NGINX_PREFIX/conf/nginx.conf"
  need_cmd sed
  mkdir -p "$NGINX_PREFIX"/{conf,logs,ssl}
  cp -f "$REPO_ROOT/tools/nginx/mime.types" "$NGINX_PREFIX/conf/mime.types"
  local ssl_dir="$NGINX_PREFIX/ssl"
  ensure_ssl_cert "$ssl_dir"
  render_nginx_from_tpl "$tpl" "$out" "$ssl_dir"
  log "green nginx conf → $out"
}

install_nginx_green_snippet_via_system() {
  [[ $EUID -eq 0 ]] || die "port ${WEB_HTTP_PORT} in use — need root to reload system nginx, or set WEB_HTTP_PORT=8088"
  local tpl ssl_dir="/etc/nginx/ssl"
  tpl="$(pick_nginx_template system)"
  ensure_ssl_cert "$ssl_dir"
  render_system_nginx_conf "$tpl"
  nginx -t || die "nginx -t failed"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl reload nginx 2>/dev/null || systemctl restart nginx
  else
    nginx -s reload
  fi
  log "reuse system nginx on :${WEB_HTTP_PORT} (static in $DEPLOY_ROOT)"
  write_deploy_info
  NGINX_MODE="system-snippet"
}

stop_green_nginx() {
  if [[ -f "$NGINX_PREFIX/logs/nginx.pid" ]]; then
    nginx -p "$NGINX_PREFIX" -s quit 2>/dev/null || true
    sleep 1
  fi
}

install_nginx_green() {
  command -v nginx >/dev/null 2>&1 || die "nginx required (install: dnf install nginx / apt install nginx)"
  NGINX_MODE="standalone"
  local tpl
  tpl="$(pick_nginx_template green)"
  [[ -f "$tpl" ]] || die "green template missing: $tpl"

  local listen_port="$WEB_HTTP_PORT"
  [[ "${ENABLE_HTTPS}" == "1" ]] && listen_port="$WEB_HTTPS_PORT"

  if port_in_use "$listen_port"; then
    log "port $listen_port already in use"
    if command -v systemctl >/dev/null 2>&1 && systemctl is-active nginx >/dev/null 2>&1; then
      log "reuse system nginx (static/ssl stay in $DEPLOY_ROOT)"
      install_nginx_green_snippet_via_system
      return 0
    fi
    if [[ $EUID -ne 0 && "$listen_port" == "80" ]]; then
      WEB_HTTP_PORT=8088
      WEB_HTTPS_PORT=8443
      log "fallback ports ${WEB_HTTP_PORT}/${WEB_HTTPS_PORT} (non-root or port busy)"
      # re-derive URL
      IDIP_WEBCLIENT_URL="http://${WEB_WAN_IP}:${WEB_HTTP_PORT}"
      export IDIP_WEBCLIENT_URL
    else
      die "port $listen_port busy and system nginx not available"
    fi
  fi

  render_green_nginx "$tpl"
  stop_green_nginx
  nginx -p "$NGINX_PREFIX" -t -c conf/nginx.conf || die "nginx -t failed"
  nginx -p "$NGINX_PREFIX" -c conf/nginx.conf
  log "green nginx started (prefix=$NGINX_PREFIX pid=$(cat "$NGINX_PREFIX/logs/nginx.pid" 2>/dev/null || echo '?'))"
  write_deploy_info
}

render_system_nginx_conf() {
  local tpl="$1"
  local out="/etc/nginx/conf.d/${NGINX_CONF_NAME}"
  need_cmd sed
  local bak_suffix
  bak_suffix=".bak.deploy-idip.$(date +%s)"
  for old in /etc/nginx/conf.d/starcrystal.conf /etc/nginx/conf.d/starcrystal-https.conf \
             /etc/nginx/conf.d/starcrystal-http.conf; do
    if [[ -f "$old" && "$old" != "$out" ]]; then
      mv "$old" "${old}${bak_suffix}"
      log "renamed conflicting $old → ${old}${bak_suffix}"
    fi
  done
  sed \
    -e "s|127\.0\.0\.1:8080|${API_BACKEND_HOST}:${API_BACKEND_PORT}|g" \
    -e "s|server 127\.0\.0\.1:8080|server ${API_BACKEND_HOST}:${API_BACKEND_PORT}|g" \
    -e "s|/var/www/starcrystal-idip/dist|${WEB_ROOT}|g" \
    "$tpl" >"$out"
  log "system nginx conf → $out"
}

install_nginx_system() {
  [[ $EUID -eq 0 ]] || die "system nginx mode requires root (sudo) or use GREEN_DEPLOY=1"
  if ! command -v nginx >/dev/null 2>&1; then
    if command -v dnf >/dev/null 2>&1; then
      dnf install -y nginx || true
    elif command -v apt-get >/dev/null 2>&1; then
      apt-get update -qq && apt-get install -y nginx || true
    fi
  fi
  command -v nginx >/dev/null 2>&1 || die "nginx not installed"
  ensure_ssl_cert "/etc/nginx/ssl"
  local tpl
  tpl="$(pick_nginx_template system)"
  render_system_nginx_conf "$tpl"
  nginx -t || die "nginx -t failed"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable nginx 2>/dev/null || true
    systemctl reload nginx 2>/dev/null || systemctl restart nginx
  else
    nginx -s reload 2>/dev/null || nginx
  fi
  log "system nginx reloaded"
}

write_deploy_info() {
  local info="$DEPLOY_ROOT/deploy-info.env"
  mkdir -p "$DEPLOY_ROOT"
  cat >"$info" <<EOF
# Generated by deploy-idip-webclient.sh — source for idip-webclient-ctl.sh
export GREEN_DEPLOY=1
export DEPLOY_ROOT='$DEPLOY_ROOT'
export NGINX_PREFIX='$NGINX_PREFIX'
export WEB_ROOT='$WEB_ROOT'
export SVRIP='$SVRIP'
export WEB_WAN_IP='$WEB_WAN_IP'
export IDIP_WEBCLIENT_URL='$IDIP_WEBCLIENT_URL'
export IDIP_BASE_URL='$IDIP_BASE_URL'
export ENABLE_HTTPS='$ENABLE_HTTPS'
export WEB_HTTP_PORT='$WEB_HTTP_PORT'
export WEB_HTTPS_PORT='$WEB_HTTPS_PORT'
EOF
  log "deploy info → $info"
}

preflight_api() {
  local url="${IDIP_BASE_URL%/}/healthz"
  if curl -fsS --max-time 8 "$url" | grep -q '"ok"'; then
    log "API healthz OK ($url)"
    return 0
  fi
  log "WARN: API healthz failed at $url"
  log "WARN: 确认 SVRIP=$SVRIP 可达且 starcrystalsvr 已启动 (release/startsvr.sh)"
}

post_deploy_smoke() {
  bash "$SCRIPT_DIR/verify-cloud.sh"
}

run_regression() {
  [[ "$SKIP_REGRESSION" == "1" ]] && { log "SKIP_REGRESSION=1"; return 0; }
  if ! has_node_18; then
    log "WARN: skip Vitest regression (Node.js 18+ not found; verify-cloud already passed)"
    log "WARN: 开发机可 npm run regression；或本机装 Node 18+ 后重跑 deploy"
    return 0
  fi
  bash "$SCRIPT_DIR/run-regression.sh"
}

print_access_hints() {
  log "OK"
  log "  外网 Web:  $IDIP_WEBCLIENT_URL/"
  log "  内网 API:  http://${SVRIP}:${API_BACKEND_PORT} (nginx upstream，勿对公网开放 8080)"
  log "  运营台 API Base 留空（浏览器同源 /idip /api）"
  if [[ "${GREEN_DEPLOY}" == "1" ]]; then
    log "  绿色部署:  $DEPLOY_ROOT (mode=${NGINX_MODE:-standalone})"
    if [[ "${NGINX_MODE:-standalone}" == "standalone" ]]; then
      log "  启停:      bash $SCRIPT_DIR/idip-webclient-ctl.sh {start|stop|status}"
    fi
  fi
  if [[ "$WEB_WAN_IP" != "127.0.0.1" ]]; then
    log "  防火墙:    放行 TCP ${WEB_HTTP_PORT}$([[ "$ENABLE_HTTPS" == "1" ]] && echo " ${WEB_HTTPS_PORT}") 到 $WEB_WAN_IP"
  fi
}

main() {
  need_cmd curl
  preflight_api
  build_webclient
  install_static
  if [[ "${GREEN_DEPLOY}" == "1" ]]; then
    install_nginx_green
  else
    install_nginx_system
  fi
  post_deploy_smoke
  run_regression
  print_access_hints
}

main "$@"
