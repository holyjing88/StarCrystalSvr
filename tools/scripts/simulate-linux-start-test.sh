#!/usr/bin/env bash
# 模拟 Linux 验收：语法检查 + 可选 Docker 启停链（可在 Git Bash / WSL / Linux 上跑）
# 用法: cd server-go && bash tools/scripts/simulate-linux-start-test.sh
set -uo pipefail
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

PASS=0
FAIL=0
SKIP=0

ok() { echo "[PASS] $1 — $2"; PASS=$((PASS + 1)); }
bad() { echo "[FAIL] $1 — $2"; FAIL=$((FAIL + 1)); }
skip() { echo "[SKIP] $1 — $2"; SKIP=$((SKIP + 1)); }

echo "=== simulate-linux-start-test ==="
echo "  REPO=$REPO_ROOT"
echo "  uname=$(uname -s 2>/dev/null || echo unknown)"
echo ""

# --- 1) 静态：bash -n ---
STATIC_SH=(
  tools/scripts/starcrystal-config.sh
  tools/scripts/dbscripts/startdb.sh
  tools/scripts/dbscripts/stopdb.sh
  tools/scripts/install-linux.sh
  tools/scripts/simulate-linux-start-test.sh
  tools/docker/install-docker.sh
  tools/docker/docker_mysql.sh
  tools/docker/docker_redis.sh
  tools/docker/docker_startdb.sh
  tools/docker/docker_stopdb.sh
  tools/docker/docker_svrdev.sh
  tools/docker/docker_release.sh
  tools/docker/docker_mirror_save.sh
  tools/docker/docker_mirror_build.sh
  tools/docker/fetch-docker-base-images.sh
  tools/docker/setup-docker-linux.sh
  tools/docker/accept-docker.sh
  tools/docker/lib/docker-common.sh
  tools/docker/mysql/Dockerfile
  tools/docker/redis/Dockerfile
  tools/docker/release/Dockerfile
  release/startsvr.sh
  release/stopsvr.sh
)
for f in "${STATIC_SH[@]}"; do
  if [[ ! -f "$f" ]]; then
    bad "SCR-SIM-missing" "$f"
    continue
  fi
  if bash -n "$f" 2>/dev/null; then
    ok "SCR-SIM-syntax" "$f"
  else
    bad "SCR-SIM-syntax" "$f"
  fi
done

# --- 2) offlinesofts 在 tools 根（非 scripts 下）---
if [[ -d tools/offlinesofts/lib ]]; then
  ok "SCR-SIM-offlinesofts" "tools/offlinesofts/"
else
  bad "SCR-SIM-offlinesofts" "缺少 tools/offlinesofts/"
fi
if [[ -d tools/scripts/offlinesofts ]]; then
  bad "SCR-SIM-offlinesofts-moved" "不应仍存在 tools/scripts/offlinesofts"
else
  ok "SCR-SIM-offlinesofts-moved" "无 tools/scripts/offlinesofts"
fi

# --- 3) docker 在 tools 根（非 scripts 下）---
if [[ -f tools/docker/docker_startdb.sh ]]; then
  ok "SCR-SIM-docker" "tools/docker/"
else
  bad "SCR-SIM-docker" "缺少 tools/docker/"
fi
for legacy in tools/scripts/docker_mysql.sh tools/scripts/install-docker.sh tools/scripts/lib/docker-common.sh; do
  if [[ -e "$legacy" ]]; then
    bad "SCR-SIM-docker-moved" "不应仍存在 $legacy"
  else
    ok "SCR-SIM-docker-moved" "无 $legacy"
  fi
done

# --- 4) release 根目录不应有 install-linux / docker_* ---
FORBIDDEN=(
  release/install-linux.sh
  release/docker_mysql.sh
  release/docker_redis.sh
  release/docker_startdb.sh
  release/docker_stopdb.sh
  release/docker_svrdev.sh
  release/install-docker.sh
  release/simulate-linux-start-test.sh
)
for f in "${FORBIDDEN[@]}"; do
  if [[ -f "$f" ]]; then
    bad "SCR-SIM-release-clean" "不应存在 $f"
  else
    ok "SCR-SIM-release-clean" "无 $f"
  fi
done

# --- 5) config 加载 ---
if bash -c 'source tools/scripts/starcrystal-config.sh && [[ -n "$RELEASE_ROOT" && -f "$RELEASE_ROOT/configs/starcrystal.json" ]]' 2>/dev/null; then
  ok "SCR-SIM-config" "RELEASE_ROOT 解析"
else
  bad "SCR-SIM-config" "starcrystal-config.sh"
fi

# --- 6) 内置 test-scripts 静态段 ---
TS_LOG="${TMPDIR:-/tmp}/sc-sim-test-scripts.log"
if bash tools/scripts/test-scripts.sh >"$TS_LOG" 2>&1; then
  ok "SCR-SIM-test-scripts" "test-scripts.sh 静态通过"
else
  bad "SCR-SIM-test-scripts" "test-scripts.sh 失败（见 $TS_LOG）"
fi

# --- 7) Docker 启停链（可选）---
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  echo ""
  echo "==> Docker live chain"
  export DOCKER_MYSQL_PORT="${DOCKER_MYSQL_PORT:-3306}"
  export DOCKER_REDIS_PORT="${DOCKER_REDIS_PORT:-6379}"
  if bash tools/docker/docker_stopdb.sh 2>/dev/null; then ok "SCR-SIM-docker-stop" "preclean"; else skip "SCR-SIM-docker-stop" "preclean"; fi
  if bash tools/docker/docker_startdb.sh; then
    ok "SCR-SIM-docker-startdb" "mysql+redis up"
    if timeout 2 bash -c 'echo >/dev/tcp/127.0.0.1/3306' 2>/dev/null; then
      ok "SCR-SIM-mysql-port" "3306"
    else
      bad "SCR-SIM-mysql-port" "3306"
    fi
    if timeout 2 bash -c 'echo >/dev/tcp/127.0.0.1/6379' 2>/dev/null; then
      ok "SCR-SIM-redis-port" "6379"
    else
      bad "SCR-SIM-redis-port" "6379"
    fi
    bash tools/docker/docker_stopdb.sh && ok "SCR-SIM-docker-stopdb" "down"
  else
    bad "SCR-SIM-docker-startdb" "failed"
  fi
else
  skip "SCR-SIM-docker" "docker 不可用，跳过 live"
fi

echo ""
echo "=== Summary: $PASS pass, $FAIL fail, $SKIP skip ==="
[[ "$FAIL" -eq 0 ]]
