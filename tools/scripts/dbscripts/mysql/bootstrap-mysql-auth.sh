#!/usr/bin/env bash
# 便携 / 系统 MySQL：用 root 创建库与应用账号，幂等可重复执行。
# 便携 MySQL（无 root 密码）请: MYSQL_ROOT_PASSWORD= bash bootstrap-mysql-auth.sh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../dbscripts-config.sh
source "$SCRIPT_DIR/../dbscripts-config.sh"

SqlHost="${MYSQL_HOST:-127.0.0.1}"
Port="${MYSQL_PORT:-3306}"
Database="${MYSQL_AUTH_DB:-starcrystal_auth}"
User="${MYSQL_AUTH_USER:-star_auth}"
Password="${MYSQL_AUTH_PASSWORD:-jgyjgyjgy}"
RootUser="${MYSQL_ROOT_USER:-root}"
RootPassword="${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}"

BaseDir="${MYSQL_PORTABLE_BASE:-$(sc_default_mysql_base)}"
mysql_bin=""
if [[ -n "${MYSQL_CLIENT:-}" && -x "${MYSQL_CLIENT}" ]]; then
  mysql_bin="$MYSQL_CLIENT"
elif command -v mysql >/dev/null 2>&1; then
  mysql_bin="$(command -v mysql)"
elif [[ -x "$BaseDir/bin/mysql" ]]; then
  mysql_bin="$BaseDir/bin/mysql"
fi
[[ -n "$mysql_bin" ]] || { echo "mysql client not found" >&2; exit 1; }

mysql_root() {
  if [[ -n "$RootPassword" ]]; then
    "$mysql_bin" --protocol=TCP --host="$SqlHost" --port="$Port" \
      --user="$RootUser" --password="$RootPassword" "$@"
  else
    "$mysql_bin" --protocol=TCP --host="$SqlHost" --port="$Port" \
      --user="$RootUser" "$@"
  fi
}

declare -a _SC_MYSQL_PW_POLICY_KEYS=()
declare -a _SC_MYSQL_PW_POLICY_VALS=()

sc_mysql_validate_password_active() {
  mysql_root -Nse "SHOW VARIABLES LIKE 'validate_password%';" 2>/dev/null | grep -q .
}

sc_mysql_save_password_policy() {
  _SC_MYSQL_PW_POLICY_KEYS=()
  _SC_MYSQL_PW_POLICY_VALS=()
  sc_mysql_validate_password_active || return 0
  local key val
  while IFS=$'\t' read -r key val; do
    case "$key" in
      validate_password_policy|validate_password.policy|validate_password_length|validate_password.length|validate_password_mixed_case_count|validate_password.mixed_case_count|validate_password_number_count|validate_password.number_count|validate_password_special_char_count|validate_password.special_char_count|validate_password_check_user_name|validate_password.check_user_name)
        _SC_MYSQL_PW_POLICY_KEYS+=("$key")
        _SC_MYSQL_PW_POLICY_VALS+=("$val")
        ;;
    esac
  done < <(mysql_root -Nse "SHOW VARIABLES LIKE 'validate_password%';" 2>/dev/null || true)
}

sc_mysql_set_global_relaxed() {
  local key="$1" val="$2"
  local qval="$val"
  [[ "$val" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] && qval="'$val'"
  mysql_root -e "SET GLOBAL ${key}=${qval};" 2>/dev/null
}

sc_mysql_relax_password_policy() {
  sc_mysql_validate_password_active || return 0
  local key val changed=0
  while IFS=$'\t' read -r key val; do
    [[ -n "$key" ]] || continue
    case "$key" in
      validate_password_policy|validate_password.policy)
        sc_mysql_set_global_relaxed "$key" LOW && changed=1
        ;;
      validate_password_length|validate_password.length)
        sc_mysql_set_global_relaxed "$key" 4 && changed=1
        ;;
      validate_password_mixed_case_count|validate_password.mixed_case_count)
        sc_mysql_set_global_relaxed "$key" 0 && changed=1
        ;;
      validate_password_number_count|validate_password.number_count)
        sc_mysql_set_global_relaxed "$key" 0 && changed=1
        ;;
      validate_password_special_char_count|validate_password.special_char_count)
        sc_mysql_set_global_relaxed "$key" 0 && changed=1
        ;;
      validate_password_check_user_name|validate_password.check_user_name)
        sc_mysql_set_global_relaxed "$key" OFF && changed=1
        ;;
    esac
  done < <(mysql_root -Nse "SHOW VARIABLES LIKE 'validate_password%';" 2>/dev/null || true)
  if [[ "$changed" -eq 0 ]]; then
    echo "[bootstrap] WARN: 未发现可调整的 validate_password 变量，跳过策略放宽" >&2
    return 0
  fi
  echo "[bootstrap] 已临时降低 validate_password 策略（完成后恢复）"
}

sc_mysql_restore_password_policy() {
  [[ ${#_SC_MYSQL_PW_POLICY_KEYS[@]} -gt 0 ]] || return 0
  local i key val qval
  for i in "${!_SC_MYSQL_PW_POLICY_KEYS[@]}"; do
    key="${_SC_MYSQL_PW_POLICY_KEYS[$i]}"
    val="${_SC_MYSQL_PW_POLICY_VALS[$i]}"
    qval="$val"
    [[ "$val" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] && qval="'$val'"
    mysql_root -e "SET GLOBAL ${key}=${qval};" 2>/dev/null || true
  done
  echo "[bootstrap] 已恢复 validate_password 策略"
}

run_bootstrap_sql() {
  mysql_root -e "
CREATE DATABASE IF NOT EXISTS \`${Database}\`
  DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON \`${Database}\`.* TO '${User}'@'%';
GRANT ALL PRIVILEGES ON \`${Database}\`.* TO '${User}'@'localhost';
FLUSH PRIVILEGES;
"
}

sc_mysql_auth_user_exists() {
  mysql_root -Nse "
SELECT 1 FROM mysql.user
WHERE user='${User}' AND host IN ('%','localhost')
LIMIT 1;" 2>/dev/null | grep -q 1
}

run_bootstrap_create_users() {
  mysql_root -e "
CREATE USER IF NOT EXISTS '${User}'@'%' IDENTIFIED BY '${Password}';
CREATE USER IF NOT EXISTS '${User}'@'localhost' IDENTIFIED BY '${Password}';
"
}

echo "Bootstrap MySQL auth DB (${Database} / ${User}@${SqlHost}:${Port}) ..."

_BOOTSTRAP_ERR="$(mktemp)"
trap 'rm -f "$_BOOTSTRAP_ERR"' EXIT

if sc_mysql_auth_user_exists; then
  run_bootstrap_sql
  echo "Bootstrap OK."
  exit 0
fi

set +e
run_bootstrap_create_users 2>"$_BOOTSTRAP_ERR"
rc=$?
set -e

if [[ "$rc" -ne 0 ]] && grep -qE '1819|password.*policy|validate_password' "$_BOOTSTRAP_ERR"; then
  echo "[bootstrap] 应用密码不符合当前策略，临时降低 validate_password 后重试 ..."
  sc_mysql_save_password_policy
  sc_mysql_relax_password_policy
  set +e
  run_bootstrap_create_users 2>"$_BOOTSTRAP_ERR"
  rc=$?
  set -e
  sc_mysql_restore_password_policy
  if [[ "$rc" -ne 0 ]]; then
    echo "[bootstrap] 放宽策略后仍无法创建用户，请检查 MySQL 版本或手动创建 ${User}:" >&2
    cat "$_BOOTSTRAP_ERR" >&2
    exit "$rc"
  fi
elif [[ "$rc" -ne 0 ]]; then
  cat "$_BOOTSTRAP_ERR" >&2
  exit "$rc"
fi

run_bootstrap_sql
echo "Bootstrap OK."
