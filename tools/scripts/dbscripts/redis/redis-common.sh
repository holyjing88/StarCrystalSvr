#!/usr/bin/env bash
# StarCrystal release/scripts/redis — 供 redis-start|stop|backup.sh source，勿单独执行。
# UTF-8：本目录 *.sh 请保存为 UTF-8；未设置 LANG 时使用 C.UTF-8，与常见 Linux/MySQL 运维脚本约定一致。
export LANG="${LANG:-C.UTF-8}"
export LC_ALL="${LC_ALL:-$LANG}"

# 与 Windows PowerShell 脚本对齐的环境变量：
#   REDIS_CONF       redis.conf 路径；未设时使用 <脚本目录>/redis.conf
#   PORT / REDIS_PORT  监听端口（任设其一即可；未设则从 redis.conf 的 port 读取，否则 6379）
#   REDIS_SERVER_EXE redis-server 可执行文件（默认 redis-server）
#   REDIS_CLI        redis-cli 可执行文件（默认 redis-cli）
#   REDIS_DATA_DIR   数据目录（默认 <dbscripts>/redis/data）
#   REDIS_BACKUP_DIR 备份目录（默认 <dbscripts>/redis/data_backup）
#   BACKUP_ROOT      同 REDIS_BACKUP_DIR（redis-backup.sh 兼容别名）
#   REDIS_DIR        备份时强制指定数据目录（覆盖自动探测）
#   FORCE_KILL       redis-stop：设为 1 时在 SHUTDOWN 后仍尝试 pkill redis-server（慎用，多实例会全杀）

: "${HERE:?redis-common.sh: 请先在调用方设置 HERE 为脚本目录后再 source}"

sc_redis_load_linux_host_env() {
  local f="$HERE/../local.env"
  [[ -f "$f" ]] || return 0
  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" ]] && continue
    [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    val="${BASH_REMATCH[2]}"
    val="${val%\"}"; val="${val#\"}"
    val="${val%\'}"; val="${val#\'}"
    case "$key" in
      REDIS_DATA_DIR)
        [[ -z "${REDIS_DATA_DIR:-}" ]] && export REDIS_DATA_DIR="$val"
        ;;
      REDIS_BACKUP_DIR)
        [[ -z "${REDIS_BACKUP_DIR:-}" ]] && export REDIS_BACKUP_DIR="$val"
        ;;
      DOCKER_REDIS_DATA)
        [[ -z "${REDIS_DATA_DIR:-}" ]] && export REDIS_DATA_DIR="$val"
        ;;
      DOCKER_REDIS_BACKUP_DIR)
        [[ -z "${REDIS_BACKUP_DIR:-}" ]] && export REDIS_BACKUP_DIR="$val"
        ;;
    esac
  done <"$f"
}

sc_redis_load_linux_host_env

sc_redis_default_data_dir() {
  echo "${REDIS_DATA_DIR:-$HERE/data}"
}

sc_redis_default_backup_dir() {
  echo "${REDIS_BACKUP_DIR:-${BACKUP_ROOT:-$HERE/data_backup}}"
}

sc_redis_ensure_dirs() {
  local data backup
  data="$(sc_redis_resolve_data_dir "$(sc_redis_conf_default)")"
  backup="$(sc_redis_default_backup_dir)"
  mkdir -p "$data" "$backup" "$HERE/logs"
}

sc_redis_conf_default() {
  if [[ -n "${REDIS_CONF:-}" ]]; then echo "$REDIS_CONF"; return; fi
  if [[ -f "$HERE/redis.conf" ]]; then echo "$HERE/redis.conf"; return; fi
  echo "$HERE/redis.conf"
}

# 端口优先级：PORT 或 REDIS_PORT 环境变量 > redis.conf 中 port > 6379
sc_redis_port() {
  local conf="${1:-}"
  local penv
  if [[ -n "${PORT:-}" ]]; then echo "$PORT"; return; fi
  penv="${REDIS_PORT:-}"
  if [[ -n "$penv" && "$penv" =~ ^[0-9]+$ ]]; then echo "$penv"; return; fi
  if [[ -z "$conf" ]]; then conf="$(sc_redis_conf_default)"; fi
  if [[ -f "$conf" ]]; then
    local p
    p=$(grep -E '^[[:space:]]*port[[:space:]]+' "$conf" 2>/dev/null | head -1 | awk '{print $2}' | tr -d '\r' || true)
    if [[ -n "$p" && "$p" =~ ^[0-9]+$ ]]; then echo "$p"; return; fi
  fi
  echo "6379"
}

sc_redis_cli() {
  if [[ -n "${REDIS_CLI:-}" ]]; then echo "$REDIS_CLI"; return; fi
  if [[ -f "$HERE/linux/redis-cli" ]]; then echo "$HERE/linux/redis-cli"; return; fi
  if [[ -f "$HERE/bin/redis-cli" ]]; then echo "$HERE/bin/redis-cli"; return; fi
  if [[ -f "$HERE/bin/redis-cli.exe" ]]; then echo "$HERE/bin/redis-cli.exe"; return; fi
  echo "redis-cli"
}

sc_redis_server() {
  if [[ -n "${REDIS_SERVER_EXE:-}" ]]; then echo "$REDIS_SERVER_EXE"; return; fi
  if [[ -f "$HERE/linux/redis-server" ]]; then echo "$HERE/linux/redis-server"; return; fi
  if [[ -f "$HERE/bin/redis-server" ]]; then echo "$HERE/bin/redis-server"; return; fi
  if [[ -f "$HERE/bin/redis-server.exe" ]]; then echo "$HERE/bin/redis-server.exe"; return; fi
  echo "redis-server"
}

# 数据目录：REDIS_DIR > REDIS_DATA_DIR > redis.conf 中 dir > <dbscripts>/redis/data
sc_redis_parse_conf_dir() {
  local conf="$1"
  local raw d conf_dir
  [[ -f "$conf" ]] || return 1
  raw=$(grep -E '^[[:space:]]*dir[[:space:]]+' "$conf" 2>/dev/null | head -1 || true)
  [[ -n "$raw" ]] || return 1
  d=$(printf '%s\n' "$raw" | sed -E 's/^[[:space:]]*dir[[:space:]]+//;s/#.*$//;s/[[:space:]]+$//')
  d="${d//\"/}"
  d="${d//\'/}"
  [[ -n "$d" ]] || return 1
  if [[ "$d" != /* ]]; then
    d="${d#./}"
    conf_dir="$(cd "$(dirname "$conf")" && pwd)"
    d="$conf_dir/$d"
  fi
  echo "$d"
}

sc_redis_resolve_data_dir() {
  local conf="${1:-$(sc_redis_conf_default)}"
  local d=""

  if [[ -n "${REDIS_DIR:-}" ]]; then
    d="$REDIS_DIR"
  elif [[ -n "${REDIS_DATA_DIR:-}" ]]; then
    d="$REDIS_DATA_DIR"
  else
    d="$(sc_redis_parse_conf_dir "$conf" 2>/dev/null || true)"
    [[ -n "$d" ]] || d="$(sc_redis_default_data_dir)"
  fi
  mkdir -p "$d"
  echo "$d"
}

sc_redis_backup_cron_register() {
  [[ "${SKIP_BACKUP_CRON:-0}" == "1" ]] && return 0
  bash "$HERE/redis-backup-register-cron.sh"
}

sc_redis_backup_cron_unregister() {
  [[ "${SKIP_BACKUP_CRON:-0}" == "1" ]] && return 0
  bash "$HERE/redis-backup-unregister-cron.sh" 2>/dev/null || true
}

sc_redis_ping() {
  local port="${1:-$(sc_redis_port "$(sc_redis_conf_default)")}"
  local cli
  cli="$(sc_redis_cli)"
  command -v "$cli" >/dev/null 2>&1 || return 1
  "$cli" -p "$port" ping 2>/dev/null | grep -q PONG
}

sc_redis_wait_stopped() {
  local port="${1:-$(sc_redis_port "$(sc_redis_conf_default)")}"
  local i
  for i in $(seq 1 15); do
    if ! sc_redis_ping "$port"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

sc_redis_system_unit() {
  if command -v systemctl >/dev/null 2>&1; then
    if systemctl list-unit-files redis.service 2>/dev/null | awk '{print $1}' | grep -qx redis.service; then
      echo redis
      return 0
    fi
  fi
  [[ -f /usr/lib/systemd/system/redis.service || -f /etc/systemd/system/redis.service ]] && { echo redis; return 0; }
  return 1
}
