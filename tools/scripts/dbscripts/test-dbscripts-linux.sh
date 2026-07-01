#!/usr/bin/env bash
# dbscripts Linux 快速验收（静态 + 可选 live 启停）。
# 用法:
#   bash tools/scripts/dbscripts/test-dbscripts-linux.sh
#   SCRIPTS_TEST_LIVE=1 REDIS_PORT=16379 bash tools/scripts/dbscripts/test-dbscripts-linux.sh
set -uo pipefail
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
LIVE="${SCRIPTS_TEST_LIVE:-0}"
REDIS_PORT="${REDIS_PORT:-16379}"
MYSQL_PORT="${MYSQL_PORT:-3306}"

REPORT=()
FAIL=0

record() {
  REPORT+=("[$1] $2 — $3")
  [[ "$1" == FAIL ]] && FAIL=$((FAIL + 1))
}

echo "=== dbscripts Linux test (LIVE=$LIVE) ==="
echo "dir=$SCRIPT_DIR"

# 1. 语法
SC_TEST_ERR=""
if bash -n "$SCRIPT_DIR/dbscripts-config.sh" \
  && bash -n "$SCRIPT_DIR/startdb.sh" \
  && bash -n "$SCRIPT_DIR/stopdb.sh" \
  && bash -n "$SCRIPT_DIR/rebuilddb.sh" \
  && bash -n "$SCRIPT_DIR/mysql/rebuild-auth-mysql.sh" \
  && bash -n "$SCRIPT_DIR/redis/rebuild-redis.sh"; then
  record PASS SYNTAX "bash -n ok"
else
  record FAIL SYNTAX "bash -n failed"
fi

# 2. 自包含：config 可 source，SQL 在目录内
if (
  set -e
  # shellcheck source=dbscripts-config.sh
  source "$SCRIPT_DIR/dbscripts-config.sh"
  [[ -n "$DBSCRIPTS_ROOT" && -d "$SQL_DIR" ]]
  [[ -f "$SQL_DIR/starcrystal_auth_mysql.sql" ]]
  [[ -f "$SQL_DIR/starcrystal_redis_keys.md" ]]
  [[ -f "$DBSCRIPTS_ROOT/config/starcrystal.json.example" ]]
  ! grep -q 'starcrystal-config\.sh' "$SCRIPT_DIR/startdb.sh"
  ! grep -q 'StarcrystalConfig' "$SCRIPT_DIR/startdb.ps1"
); then
  record PASS SELFCONTAINED "dbscripts-config + sql/ ok"
else
  record FAIL SELFCONTAINED "missing local resources or external refs remain"
fi

# 3. redis 二进制探测
HERE="$SCRIPT_DIR/redis"
# shellcheck source=redis/redis-common.sh
source "$SCRIPT_DIR/redis/redis-common.sh"
REDIS_CLI_BIN="$(sc_redis_cli)"
REDIS_SVR_BIN="$(sc_redis_server)"
if command -v "$REDIS_CLI_BIN" >/dev/null 2>&1; then
  record PASS REDIS-BIN "cli=$REDIS_CLI_BIN"
else
  record SKIP REDIS-BIN "no redis-cli (install-redis-linux.sh or PATH)"
fi

# 4. mysql 便携探测
# shellcheck source=dbscripts-config.sh
source "$SCRIPT_DIR/dbscripts-config.sh"
MYSQL_BASE="$(sc_default_mysql_base)"
if [[ -x "$(sc_portable_mysqld)" ]] || command -v mysql >/dev/null 2>&1; then
  record PASS MYSQL-BIN "base=$MYSQL_BASE"
else
  record SKIP MYSQL-BIN "set MYSQL_PORTABLE_BASE in local.env"
fi

if [[ "$LIVE" == "1" ]]; then
  if command -v "$REDIS_CLI_BIN" >/dev/null 2>&1; then
    if REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/redis-stop.sh" || true \
     && REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/redis-start.sh" \
     && sleep 1 \
     && "$REDIS_CLI_BIN" -p "$REDIS_PORT" ping | grep -q PONG \
     && REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/rebuild-redis.sh" -p "$REDIS_PORT" \
     && REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/redis/redis-stop.sh"
  then
      record PASS LIVE-REDIS "start/rebuild/stop port=$REDIS_PORT"
    else
      record FAIL LIVE-REDIS "redis live test failed"
    fi
  else
    record SKIP LIVE-REDIS "no redis-cli"
  fi

  if [[ -x "$(sc_portable_mysqld)" ]] || command -v mysql >/dev/null 2>&1; then
    if MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/mysql-stop.sh" || true \
     && (MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/mysql-start.sh" || true) \
     && sleep 2 \
     && MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}" MYSQL_PORT="$MYSQL_PORT" \
       bash "$SCRIPT_DIR/mysql/bootstrap-mysql-auth.sh" \
     && MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/rebuild-auth-mysql.sh" -p "$MYSQL_PORT" \
     && MYSQL_PORT="$MYSQL_PORT" bash "$SCRIPT_DIR/mysql/mysql-stop.sh" || true
  then
      record PASS LIVE-MYSQL "bootstrap+rebuild port=$MYSQL_PORT"
    else
      record FAIL LIVE-MYSQL "mysql live test failed"
    fi
  else
    record SKIP LIVE-MYSQL "no mysql"
  fi

  if command -v "$REDIS_CLI_BIN" >/dev/null 2>&1; then
    if REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/stopdb.sh" || true \
     && REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/startdb.sh" \
     && sleep 2 \
     && "$REDIS_CLI_BIN" -p "$REDIS_PORT" ping | grep -q PONG \
     && REDIS_PORT="$REDIS_PORT" bash "$SCRIPT_DIR/stopdb.sh"
  then
      record PASS LIVE-STARTDB "startdb/stopdb port=$REDIS_PORT"
    else
      record FAIL LIVE-STARTDB "startdb live test failed"
    fi
  else
    record SKIP LIVE-STARTDB "no redis"
  fi
else
  record SKIP LIVE-REDIS "SCRIPTS_TEST_LIVE not set"
  record SKIP LIVE-MYSQL "SCRIPTS_TEST_LIVE not set"
  record SKIP LIVE-STARTDB "SCRIPTS_TEST_LIVE not set"
fi

echo ""
for line in "${REPORT[@]}"; do echo "$line"; done
echo ""
pass_n=0 skip_n=0
for line in "${REPORT[@]}"; do
  [[ "$line" == \[PASS\]* ]] && pass_n=$((pass_n + 1))
  [[ "$line" == \[SKIP\]* ]] && skip_n=$((skip_n + 1))
done
echo "Total: ${pass_n} pass, ${FAIL} fail, ${skip_n} skip / ${#REPORT[@]}"
[[ "$FAIL" -eq 0 ]]
