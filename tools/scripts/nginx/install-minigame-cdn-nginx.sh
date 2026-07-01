#!/usr/bin/env bash
# 安装联调 H5 CDN Nginx 配置（:9090 → /wwwroot/minigame.starlaneinfinite.com/h5/）
set -euo pipefail

log() { echo "[install-minigame-cdn-nginx] $*"; }
die() { echo "[install-minigame-cdn-nginx] FAIL: $*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TPL="$REPO_ROOT/tools/nginx/starcrystal-minigame-cdn-http.conf"
CONF_NAME="${NGINX_CONF_NAME:-starcrystal-minigame-cdn.conf}"
WWW_ROOT="${MINIGAME_WWW_ROOT:-/wwwroot/minigame.starlaneinfinite.com}"

[[ -f "$TPL" ]] || die "template missing: $TPL"
command -v nginx >/dev/null 2>&1 || die "nginx not installed"

if [[ $EUID -ne 0 ]]; then
  die "run as root (sudo)"
fi

mkdir -p "$WWW_ROOT/h5"
chmod -R a+rX "$WWW_ROOT" 2>/dev/null || true
if command -v chcon >/dev/null 2>&1 && command -v getenforce >/dev/null 2>&1; then
  if [[ "$(getenforce 2>/dev/null)" != "Disabled" ]]; then
    chcon -R -t httpd_sys_content_t "$WWW_ROOT" 2>/dev/null || true
    log "SELinux: httpd_sys_content_t on $WWW_ROOT"
  fi
fi

OUT="/etc/nginx/conf.d/$CONF_NAME"
cp -f "$TPL" "$OUT"
log "installed $OUT"

nginx -t || die "nginx -t failed"
if command -v systemctl >/dev/null 2>&1; then
  systemctl reload nginx 2>/dev/null || systemctl restart nginx
else
  nginx -s reload
fi

if command -v semanage >/dev/null 2>&1; then
  if semanage port -l 2>/dev/null | grep -qE '[[:space:]]9090[[:space:]]'; then
    semanage port -m -t http_port_t -p tcp 9090 2>/dev/null || true
  else
    semanage port -a -t http_port_t -p tcp 9090 2>/dev/null || true
  fi
  log "SELinux: port 9090 → http_port_t (nginx)"
fi

if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active firewalld >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port=9090/tcp >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
  log "firewalld: opened 9090/tcp (if firewalld active)"
fi

log "OK — probe: curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:9090/h5/"
log "wwwroot: $WWW_ROOT (sync from release_h5 before expecting 200)"
