#!/usr/bin/env bash
# StarCrystal MySQL 脚本公共变量（备份 / 路径）
export LANG="${LANG:-C.UTF-8}"

: "${HERE:?mysql-common.sh: 请先在调用方设置 HERE 为脚本目录后再 source}"

sc_mysql_load_linux_host_env() {
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
      MYSQL_DATA_DIR)
        [[ -z "${MYSQL_DATA_DIR:-}" ]] && export MYSQL_DATA_DIR="$val"
        ;;
      MYSQL_BACKUP_DIR)
        [[ -z "${MYSQL_BACKUP_DIR:-}" ]] && export MYSQL_BACKUP_DIR="$val"
        ;;
      DOCKER_MYSQL_DATA)
        [[ -z "${MYSQL_DATA_DIR:-}" ]] && export MYSQL_DATA_DIR="$val"
        ;;
      DOCKER_MYSQL_BACKUP_DIR)
        [[ -z "${MYSQL_BACKUP_DIR:-}" ]] && export MYSQL_BACKUP_DIR="$val"
        ;;
      MYSQL_ROOT_PASSWORD)
        [[ -z "${MYSQL_ROOT_PASSWORD:-}" ]] && export MYSQL_ROOT_PASSWORD="$val"
        ;;
    esac
  done <"$f"
}

sc_mysql_load_linux_host_env

sc_mysql_default_data_dir() {
  echo "${MYSQL_DATA_DIR:-$HERE/../data/mysql}"
}

sc_mysql_default_backup_dir() {
  echo "${MYSQL_BACKUP_DIR:-$HERE/../data/mysql_backup}"
}

sc_mysql_ensure_dirs() {
  local data backup
  data="$(sc_mysql_default_data_dir)"
  backup="$(sc_mysql_default_backup_dir)"
  mkdir -p "$data" "$backup"
}

sc_mysql_backup_defaults() {
  MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
  MYSQL_PORT="${MYSQL_PORT:-3306}"
  MYSQL_USER="${MYSQL_USER:-root}"
  MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}}"
  MYSQL_DATABASE="${MYSQL_DATABASE:-starcrystal_auth}"
  MYSQL_UNIX_SOCKET="${MYSQL_UNIX_SOCKET:-}"
  BACKUP_ROOT="${BACKUP_ROOT:-$(sc_mysql_default_backup_dir)}"
  KEEP="${KEEP:-14}"
}

sc_mysql_backup_cron_register() {
  [[ "${SKIP_BACKUP_CRON:-0}" == "1" ]] && return 0
  bash "$HERE/mysql-backup-register-cron.sh"
}

sc_mysql_backup_cron_unregister() {
  [[ "${SKIP_BACKUP_CRON:-0}" == "1" ]] && return 0
  bash "$HERE/mysql-backup-unregister-cron.sh" 2>/dev/null || true
}
