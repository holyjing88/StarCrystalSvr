#!/usr/bin/env bash
# StarCrystal Linux 一键安装：离线包 → 编译 API → 初始化 MySQL → 启动 MySQL/Redis/服务
#
# 用法:
#   cd server-go/release && bash ../tools/scripts/install-linux.sh
#   或: bash install-linux.sh（release 根目录转发）
#
# 环境变量:
#   ONLINE_FETCH=1     缺少离线包时先联网下载（需 curl/wget）
#   SKIP_OFFLINE=1     跳过 offlinesofts 安装（已装过）
#   SKIP_BUILD=1       不编译 starcrystalsvr（已有二进制）
#   SKIP_SCHEMA=1      跳过 bootstrap + rebuild-auth-mysql
#   SKIP_START=1       安装后不启动服务
#   SKIP_GO=1 等       传给 install-linux-offline.sh（见 tools/offlinesofts/README.md）
#
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=starcrystal-config.sh
source "$SCRIPT_DIR/starcrystal-config.sh"
cd "$RELEASE_ROOT"

OFFLINE_INSTALL="$OFFLINE_SOFTS_ROOT/install-linux-offline.sh"
OFFLINE_FETCH="$OFFLINE_SOFTS_ROOT/fetch-linux-offline-packages.sh"
# shellcheck source=../offlinesofts/lib/offline-common.sh
source "$OFFLINE_SOFTS_ROOT/lib/offline-common.sh"

log() { echo ""; echo "==> $*"; }

need_linux() {
  case "$(uname -s 2>/dev/null || echo unknown)" in
    Linux) ;;
    *)
      echo "本脚本仅用于 Linux（当前: $(uname -s 2>/dev/null || echo unknown)）" >&2
      exit 1
      ;;
  esac
}

offline_archives_present() {
  offline_find_archive mysql "mysql-*.tar.xz" >/dev/null 2>&1 \
    || offline_find_archive redis "redis-*.tar.gz" >/dev/null 2>&1
}

setup_go_env() {
  sc_setup_go_env
}

step_offline() {
  [[ "${SKIP_OFFLINE:-0}" == "1" ]] && { log "跳过离线包安装 (SKIP_OFFLINE=1)"; return 0; }

  local mysql_ok=0 redis_ok=0
  [[ -x "$(sc_default_mysql_base)/bin/mysqld" ]] && mysql_ok=1
  [[ -x "$(offline_redis_linux_dest)/redis-server" ]] && redis_ok=1
  if [[ "$mysql_ok" == 1 && "$redis_ok" == 1 ]]; then
    log "便携 MySQL/Redis 已存在，跳过 offlinesofts 安装"
    return 0
  fi

  if ! offline_archives_present; then
    if [[ "${ONLINE_FETCH:-0}" == "1" ]]; then
      log "联网下载离线包"
      bash "$OFFLINE_FETCH"
    else
      echo "未找到 offlinesofts 安装包（mysql/*.tar.xz、redis/*.tar.gz）。" >&2
      echo "请将离线包放入 tools/offlinesofts/ 对应子目录，或设置 ONLINE_FETCH=1 后重试。" >&2
      exit 1
    fi
  fi

  log "安装离线包 (offlinesofts)"
  bash "$OFFLINE_INSTALL"
}

step_build() {
  [[ "${SKIP_BUILD:-0}" == "1" ]] && { log "跳过编译 (SKIP_BUILD=1)"; return 0; }
  if [[ -x "$RELEASE_ROOT/starcrystalsvr" ]]; then
    log "已存在 starcrystalsvr，跳过编译（强制重编请 rm starcrystalsvr 或 unset SKIP_BUILD）"
    return 0
  fi
  setup_go_env
  if ! command -v go >/dev/null 2>&1; then
    echo "未找到 go，无法编译。请放入 offlinesofts/go/ 后重跑，或 SKIP_BUILD=1 使用已有二进制。" >&2
    exit 1
  fi
  log "编译 starcrystalsvr"
  bash "$SCRIPT_DIR/build.sh"
}

step_mysql_schema() {
  [[ "${SKIP_SCHEMA:-0}" == "1" ]] && { log "跳过库表 (SKIP_SCHEMA=1)"; return 0; }
  if ! sc_mysql_available; then
    log "无可用 MySQL，跳过 schema（请自行配置 AUTH_MYSQL_DSN）"
    return 0
  fi
  log "启动 MySQL (mode=$(sc_mysql_mode))"
  bash "$SCRIPT_DIR/dbscripts/mysql/mysql-start.sh"
  log "初始化库与账号 (bootstrap-mysql-auth)"
  if [[ "$(sc_mysql_mode)" == portable ]]; then
    MYSQL_ROOT_PASSWORD= bash "$SCRIPT_DIR/dbscripts/mysql/bootstrap-mysql-auth.sh"
  else
    bash "$SCRIPT_DIR/dbscripts/mysql/bootstrap-mysql-auth.sh"
  fi
  log "重建 auth 表结构 (rebuild-auth-mysql)"
  bash "$SCRIPT_DIR/dbscripts/mysql/rebuild-auth-mysql.sh"
}

step_start_services() {
  [[ "${SKIP_START:-0}" == "1" ]] && { log "跳过启动 (SKIP_START=1)"; return 0; }
  log "启动 Redis + starcrystalsvr（含 MySQL 若已安装）"
  bash "$SCRIPT_DIR/startallsvr.sh"
}

main() {
  need_linux
  log "StarCrystal Linux 一键安装"
  echo "    RELEASE_ROOT=$RELEASE_ROOT"
  echo "    REPO_ROOT=$REPO_ROOT"

  step_offline
  step_build
  step_mysql_schema
  step_start_services

  log "完成"
  api="$(sc_api_base_url 2>/dev/null || echo 'http://0.0.0.0:8080')"
  echo "  API: $api"
  echo "  停止: ../tools/scripts/stopallsvr.sh"
  echo "  API:  ./stopsvr.sh"
  echo "  日志: $RELEASE_ROOT/log/"
}

main "$@"
