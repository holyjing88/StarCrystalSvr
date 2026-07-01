#!/usr/bin/env bash
# StarCrystal idip-webclient 卸载 — 撤销 deploy-idip-webclient.sh 对系统/绿色部署的影响
#
# 典型用法（与 deploy 相同，自动读 deploy-info.env / deploy-env.local.sh）：
#   bash tools/idip-webclient/scripts/undeploy-idip-webclient.sh
#
# 系统 nginx 模式（需 root 删除 conf.d 并 reload）：
#   sudo bash tools/idip-webclient/scripts/undeploy-idip-webclient.sh --system-nginx
#
# 删除绿色部署目录与静态文件（.deploy/idip-webclient、WEB_ROOT）：
#   bash tools/idip-webclient/scripts/undeploy-idip-webclient.sh --purge
#
# 环境变量（与 deploy 一致，deploy-info.env 优先）：
#   GREEN_DEPLOY / DEPLOY_ROOT / NGINX_PREFIX / WEB_ROOT / NGINX_CONF_NAME
#   RESTORE_NGINX_BACKUPS=1   恢复 deploy 备份的 starcrystal*.conf（默认 1）
#   PURGE=1                 同 --purge
#   SKIP_NGINX_RELOAD=1     不 reload 系统 nginx
#
set -euo pipefail

log() { echo "[undeploy-idip-webclient] $*"; }
warn() { echo "[undeploy-idip-webclient] WARN: $*" >&2; }
die() { echo "[undeploy-idip-webclient] FAIL: $*" >&2; exit 1; }

# ---- 路径探测（与 deploy 一致）----
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

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --green) GREEN_DEPLOY=1; shift ;;
      --system-nginx) GREEN_DEPLOY=0; shift ;;
      --purge) PURGE=1; shift ;;
      --no-restore-nginx) RESTORE_NGINX_BACKUPS=0; shift ;;
      --skip-reload) SKIP_NGINX_RELOAD=1; shift ;;
      -h|--help)
        sed -n '2,18p' "$0"
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

DEPLOY_ENV="$SCRIPT_DIR/deploy-env.local.sh"
if [[ -f "$DEPLOY_ENV" ]]; then
  log "source $DEPLOY_ENV"
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi

if [[ "$HAS_CLI_ARGS" -eq 0 ]]; then
  DEFAULT_ENV="$SCRIPT_DIR/deploy-env.local.example.sh"
  if [[ -f "$DEFAULT_ENV" ]]; then
    log "no CLI args — source defaults: $DEFAULT_ENV"
    # shellcheck disable=SC1090
    source "$DEFAULT_ENV"
  fi
fi

DEPLOY_INFO="${DEPLOY_ROOT:-$REPO_ROOT/.deploy/idip-webclient}/deploy-info.env"
if [[ -f "$DEPLOY_INFO" ]]; then
  log "source deploy state: $DEPLOY_INFO"
  # shellcheck disable=SC1090
  source "$DEPLOY_INFO"
fi

GREEN_DEPLOY="${GREEN_DEPLOY:-1}"
NGINX_CONF_NAME="${NGINX_CONF_NAME:-starcrystal-idip.conf}"
RESTORE_NGINX_BACKUPS="${RESTORE_NGINX_BACKUPS:-1}"
PURGE="${PURGE:-0}"
SKIP_NGINX_RELOAD="${SKIP_NGINX_RELOAD:-0}"

DEPLOY_ROOT="${DEPLOY_ROOT:-$REPO_ROOT/.deploy/idip-webclient}"
NGINX_PREFIX="${NGINX_PREFIX:-$DEPLOY_ROOT/nginx}"
if [[ "${GREEN_DEPLOY}" == "1" ]]; then
  WEB_ROOT="${WEB_ROOT:-$DEPLOY_ROOT/www}"
else
  WEB_ROOT="${WEB_ROOT:-/var/www/starcrystal-idip/dist}"
fi

SYSTEM_NGINX_CONF="/etc/nginx/conf.d/${NGINX_CONF_NAME}"

stop_green_nginx() {
  local pid_file="$NGINX_PREFIX/logs/nginx.pid"
  if [[ ! -f "$pid_file" ]]; then
    log "green nginx not running (no $pid_file)"
    return 0
  fi
  if command -v nginx >/dev/null 2>&1; then
    nginx -p "$NGINX_PREFIX" -s quit 2>/dev/null || true
    sleep 1
  fi
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      sleep 1
    fi
    rm -f "$pid_file"
  fi
  log "stopped green nginx (prefix=$NGINX_PREFIX)"
}

safe_remove_path() {
  local target="$1"
  local label="$2"
  [[ -n "$target" && "$target" != "/" ]] || die "refuse to remove empty or / ($label)"
  case "$target" in
    /|/etc|/etc/nginx|/var|/var/www|/usr|/home|/root) die "refuse to remove $target ($label)" ;;
  esac
  if [[ ! -e "$target" ]]; then
    log "skip missing $label: $target"
    return 0
  fi
  rm -rf "$target"
  log "removed $label: $target"
}

remove_idip_system_nginx_conf() {
  [[ $EUID -eq 0 ]] || {
    warn "not root — skip removing $SYSTEM_NGINX_CONF (sudo undeploy with --system-nginx)"
    return 1
  }
  if [[ -f "$SYSTEM_NGINX_CONF" ]]; then
    rm -f "$SYSTEM_NGINX_CONF"
    log "removed $SYSTEM_NGINX_CONF"
  else
    log "no system conf: $SYSTEM_NGINX_CONF"
  fi
  return 0
}

restore_nginx_backups() {
  [[ "${RESTORE_NGINX_BACKUPS}" == "1" ]] || { log "RESTORE_NGINX_BACKUPS=0 — skip restore"; return 0; }
  [[ $EUID -eq 0 ]] || {
    warn "not root — skip restoring starcrystal*.conf backups"
    return 1
  }
  local conf_d="/etc/nginx/conf.d"
  local base restored=0
  for base in starcrystal.conf starcrystal-https.conf starcrystal-http.conf; do
    if [[ -f "$conf_d/$base" ]]; then
      log "keep existing $conf_d/$base"
      continue
    fi
    local latest=""
    latest="$(ls -1t "$conf_d/${base}.bak.deploy-idip."* 2>/dev/null | head -1)" || true
    if [[ -z "$latest" ]]; then
      continue
    fi
    if [[ -n "$latest" && -f "$latest" ]]; then
      mv "$latest" "$conf_d/$base"
      log "restored $conf_d/$base ← $(basename "$latest")"
      restored=1
    fi
  done
  if [[ "$restored" -eq 0 ]]; then
    log "no starcrystal*.conf.bak.deploy-idip.* to restore"
  fi
}

reload_system_nginx() {
  [[ "${SKIP_NGINX_RELOAD}" == "1" ]] && { log "SKIP_NGINX_RELOAD=1"; return 0; }
  [[ $EUID -eq 0 ]] || return 0
  command -v nginx >/dev/null 2>&1 || { warn "nginx not installed — skip reload"; return 0; }
  if nginx -t 2>/dev/null; then
    if command -v systemctl >/dev/null 2>&1; then
      systemctl reload nginx 2>/dev/null || systemctl restart nginx 2>/dev/null || warn "systemctl reload nginx failed"
    else
      nginx -s reload 2>/dev/null || warn "nginx -s reload failed"
    fi
    log "system nginx reloaded"
  else
    warn "nginx -t failed — fix configs manually before reload"
  fi
}

purge_deploy_artifacts() {
  [[ "${PURGE}" == "1" ]] || return 0

  # 仅清理 deploy 常见路径，避免误删自定义 WEB_ROOT
  local purge_web=0
  case "$WEB_ROOT" in
    "$DEPLOY_ROOT/www"|/var/www/starcrystal-idip/dist)
      purge_web=1
      ;;
    *)
      warn "WEB_ROOT=$WEB_ROOT not in default list — skip deleting static files (set PURGE after moving files)"
      ;;
  esac

  if [[ "$purge_web" -eq 1 && -d "$WEB_ROOT" ]]; then
    safe_remove_path "$WEB_ROOT" "WEB_ROOT"
  fi

  if [[ -d "$DEPLOY_ROOT" ]]; then
    safe_remove_path "$DEPLOY_ROOT" "DEPLOY_ROOT"
  elif [[ -f "$DEPLOY_INFO" ]]; then
    rm -f "$DEPLOY_INFO"
    log "removed $DEPLOY_INFO"
  fi
}

remove_deploy_info() {
  local info="$DEPLOY_ROOT/deploy-info.env"
  if [[ -f "$info" ]]; then
    rm -f "$info"
    log "removed $info"
  fi
}

print_summary() {
  log "done"
  log "  green nginx: stopped (prefix=$NGINX_PREFIX)"
  if [[ $EUID -eq 0 ]]; then
    log "  system nginx: removed $NGINX_CONF_NAME; backups restored=$RESTORE_NGINX_BACKUPS"
  else
    log "  system nginx: run with sudo to remove $SYSTEM_NGINX_CONF and restore starcrystal*.conf"
  fi
  if [[ "${PURGE}" == "1" ]]; then
    log "  purge: removed deploy tree / static under default paths"
  else
    log "  files kept: $DEPLOY_ROOT (use --purge to delete)"
  fi
  log "  starcrystalsvr API (8080) was not stopped — unaffected"
}

main() {
  log "REPO_ROOT=$REPO_ROOT GREEN_DEPLOY=$GREEN_DEPLOY PURGE=$PURGE"
  log "DEPLOY_ROOT=$DEPLOY_ROOT NGINX_PREFIX=$NGINX_PREFIX WEB_ROOT=$WEB_ROOT"

  stop_green_nginx

  # 绿色部署也可能写过系统 nginx（端口占用时 install_nginx_green_snippet_via_system）
  if [[ $EUID -eq 0 ]]; then
    remove_idip_system_nginx_conf || true
    restore_nginx_backups || true
    reload_system_nginx
  elif [[ -f "$SYSTEM_NGINX_CONF" ]]; then
    warn "found $SYSTEM_NGINX_CONF but not root — run: sudo $0"
  fi

  purge_deploy_artifacts
  remove_deploy_info

  print_summary
}

main "$@"
