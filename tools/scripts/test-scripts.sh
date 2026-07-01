#!/usr/bin/env bash
# release/scripts 运维脚本验收（Linux）。静态默认；SCRIPTS_TEST_LIVE=1 启停实测。
# 用法: cd server-go && bash tools/scripts/test-scripts.sh
set -uo pipefail
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=starcrystal-config.sh
source "$SCRIPT_DIR/starcrystal-config.sh"

LIVE="${SCRIPTS_TEST_LIVE:-0}"
FULL="${SCRIPTS_TEST_FULL:-0}"
BUILD="${SCRIPTS_TEST_BUILD:-0}"
REDIS_PORT="${SCRIPTS_TEST_REDIS_PORT:-16379}"
MYSQL_PORT="${MYSQL_PORT:-3306}"

REPORT=()
FAIL_COUNT=0

record() {
  local mark="$1" id="$2" msg="$3"
  REPORT+=("[${mark}] ${id} — ${msg}")
  [[ "$mark" == FAIL ]] && FAIL_COUNT=$((FAIL_COUNT + 1))
}

run_static() {
  local id="$1"
  shift
  if "$@"; then record PASS "$id" ok
  else record FAIL "$id" "${SC_TEST_ERR:-failed}"
  fi
}

run_optional() {
  local id="$1" skip_reason="$2"
  shift 2
  if [[ -n "$skip_reason" ]]; then
    record SKIP "$id" "$skip_reason"
    return 0
  fi
  if "$@"; then record PASS "$id" ok
  else record FAIL "$id" "${SC_TEST_ERR:-failed}"
  fi
}

linux_entries=(
  dbscripts/startdb.sh dbscripts/stopdb.sh
  dbscripts/mysql/mysql-start.sh dbscripts/mysql/mysql-stop.sh
  startallsvr.sh stopallsvr.sh
  build.sh install-linux.sh
  simulate-linux-start-test.sh
  dbscripts/mysql/bootstrap-mysql-auth.sh dbscripts/mysql/rebuild-auth-mysql.sh
  dbscripts/redis/redis-start.sh dbscripts/redis/redis-stop.sh dbscripts/redis/redis-common.sh
)

DOCKER_DIR="$(cd "$SCRIPT_DIR/../docker" && pwd)"
docker_entries=(
  install-docker.sh
  docker_mysql.sh docker_redis.sh docker_release.sh docker_mirror_save.sh
  docker_startdb.sh docker_stopdb.sh docker_svrdev.sh
  lib/docker-common.sh
)

# SCR-S-CONFIG-L
SC_TEST_ERR=""
run_static SCR-S-CONFIG-L bash -c "
  set -e
  source \"$SCRIPT_DIR/starcrystal-config.sh\"
  [[ -n \"\$RELEASE_ROOT\" && -n \"\$REPO_ROOT\" ]]
  [[ -f \"\$RELEASE_ROOT/configs/starcrystal.json\" ]]
"

# SCR-S-ENTRY-L
SC_TEST_ERR=""
run_static SCR-S-ENTRY-L bash -c "
  set -e
  for name in ${linux_entries[*]}; do
    f=\"$SCRIPT_DIR/\$name\"
    [[ -f \"\$f\" ]] || { echo \"missing tools/scripts/\$name\"; exit 1; }
    bash -n \"\$f\"
  done
  for name in startsvr.sh stopsvr.sh; do
    f=\"$RELEASE_ROOT/\$name\"
    [[ -f \"\$f\" ]] || { echo \"missing release/\$name\"; exit 1; }
    bash -n \"\$f\"
  done
  for name in ${docker_entries[*]}; do
    f=\"$DOCKER_DIR/\$name\"
    [[ -f \"\$f\" ]] || { echo \"missing tools/docker/\$name\"; exit 1; }
    bash -n \"\$f\"
  done
"

has_redis() {
  local HERE="$SCRIPT_DIR/dbscripts/redis"
  # shellcheck source=dbscripts/redis/redis-common.sh
  source "$SCRIPT_DIR/dbscripts/redis/redis-common.sh"
  local exe
  exe="$(sc_redis_server)"
  [[ -x "$exe" ]]
}

has_mysql() { [[ -x "$(sc_default_mysql_base)/bin/mysqld" ]]; }
has_svr() { [[ -x "$RELEASE_ROOT/starcrystalsvr" ]]; }

if [[ "$LIVE" == "1" ]]; then
  if has_redis; then
    SC_TEST_ERR=""
    run_static SCR-L-REDIS bash -c "
      set -e
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/redis/redis-stop.sh\" || true
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/redis/redis-start.sh\"
      sleep 1
      timeout 1 bash -c 'echo >/dev/tcp/127.0.0.1/$REDIS_PORT' 2>/dev/null
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/redis/redis-stop.sh\"
    "
  else
    record SKIP SCR-L-REDIS "no redis binary"
  fi

  if has_mysql; then
    SC_TEST_ERR=""
    run_static SCR-L-MYSQL bash -c "
      set -e
      MYSQL_PORT=$MYSQL_PORT bash \"$SCRIPT_DIR/dbscripts/mysql/mysql-stop.sh\" || true
      MYSQL_PORT=$MYSQL_PORT bash \"$SCRIPT_DIR/dbscripts/mysql/mysql-start.sh\"
      sleep 2
      timeout 1 bash -c 'echo >/dev/tcp/127.0.0.1/$MYSQL_PORT' 2>/dev/null
      MYSQL_PORT=$MYSQL_PORT bash \"$SCRIPT_DIR/dbscripts/mysql/mysql-stop.sh\"
    "
  else
    record SKIP SCR-L-MYSQL "no portable mysqld"
  fi

  if has_redis; then
    SC_TEST_ERR=""
    run_static SCR-L-STARTDB bash -c "
      set -e
      REDIS_PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/stopdb.sh\" || true
      REDIS_PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/startdb.sh\"
      sleep 2
      timeout 1 bash -c 'echo >/dev/tcp/127.0.0.1/$REDIS_PORT' 2>/dev/null
      REDIS_PORT=$REDIS_PORT bash \"$SCRIPT_DIR/dbscripts/stopdb.sh\"
    "
  else
    record SKIP SCR-L-STARTDB "no redis"
  fi

  if has_svr; then
    SC_TEST_ERR=""
    run_static SCR-L-STARTSVR bash -c "
      set -e
      bash \"$RELEASE_ROOT/stopsvr.sh\" || true
      if [[ -x \"$(sc_default_mysql_base)/bin/mysqld\" ]]; then
        bash \"$SCRIPT_DIR/dbscripts/mysql/mysql-start.sh\" || true
        sleep 2
      fi
      bash \"$RELEASE_ROOT/startsvr.sh\"
      sleep 2
      curl -fsS \"\$(sc_api_base_url)/healthz\" >/dev/null
      bash \"$SCRIPT_DIR/stopsvr.sh\"
      bash \"$SCRIPT_DIR/dbscripts/mysql/mysql-stop.sh\" 2>/dev/null || true
    "
  else
    record SKIP SCR-L-STARTSVR "no starcrystalsvr"
  fi

  if [[ "$FULL" == "1" ]] && has_svr && has_redis; then
    SC_TEST_ERR=""
    run_static SCR-L-STARTALL bash -c "
      set -e
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/stopallsvr.sh\" || true
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/startallsvr.sh\"
      sleep 3
      curl -fsS \"\$(sc_api_base_url)/healthz\" >/dev/null
      PORT=$REDIS_PORT bash \"$SCRIPT_DIR/stopallsvr.sh\"
    "
  else
    record SKIP SCR-L-STARTALL "SCRIPTS_TEST_FULL=1 and svr+redis required"
  fi
else
  record SKIP SCR-L-REDIS "SCRIPTS_TEST_LIVE not set"
  record SKIP SCR-L-MYSQL "SCRIPTS_TEST_LIVE not set"
  record SKIP SCR-L-STARTDB "SCRIPTS_TEST_LIVE not set"
  record SKIP SCR-L-STARTSVR "SCRIPTS_TEST_LIVE not set"
  record SKIP SCR-L-STARTALL "SCRIPTS_TEST_LIVE not set"
fi

if [[ "$BUILD" == "1" ]]; then
  if command -v go >/dev/null 2>&1; then
    SC_TEST_ERR=""
    run_static SCR-S-BUILD bash "$SCRIPT_DIR/build.sh"
  else
    record SKIP SCR-S-BUILD "no go"
  fi
else
  record SKIP SCR-S-BUILD "SCRIPTS_TEST_BUILD not set"
fi

echo ""
echo "=== Script acceptance ==="
for line in "${REPORT[@]}"; do echo "$line"; done
pass_n=0 skip_n=0
for line in "${REPORT[@]}"; do
  [[ "$line" == \[PASS\]* ]] && pass_n=$((pass_n + 1))
  [[ "$line" == \[SKIP\]* ]] && skip_n=$((skip_n + 1))
done
echo ""
echo "Total: ${pass_n} pass, ${FAIL_COUNT} fail, ${skip_n} skip / ${#REPORT[@]}"
[[ "$FAIL_COUNT" -eq 0 ]]
