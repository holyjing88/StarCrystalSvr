#!/usr/bin/env bash
# 清空 StarCrystal Redis 运行时键（sr:*），等价于重建无表结构的 Redis 数据层。
# 键约定见 sql/starcrystal_redis_keys.md；配置读 dbscripts/config/starcrystal.json 或 local.env。
#
# 用法:
#   bash tools/scripts/dbscripts/redis/rebuild-redis.sh
#   bash tools/scripts/dbscripts/redis/rebuild-redis.sh -H 127.0.0.1 -p 6379 --db 0
#   REDIS_REBUILD_FLUSHDB=1 bash tools/scripts/dbscripts/redis/rebuild-redis.sh   # 整库 FLUSHDB（慎用）
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
# shellcheck source=../dbscripts-config.sh
source "$SCRIPT_DIR/../dbscripts-config.sh"

RedisHost="${REDIS_HOST:-127.0.0.1}"
RedisPort="${REDIS_PORT:-6379}"
RedisDb="${REDIS_DB:-0}"
RedisPassword="${REDIS_PASSWORD:-}"
Pattern="${REDIS_REBUILD_PATTERN:-sr:*}"
SchemaDoc="$SQL_DIR/starcrystal_redis_keys.md"

if [[ -f "$(sc_config_json)" ]]; then
  sc_py=""
  if command -v python3 >/dev/null 2>&1; then
    sc_py=python3
  elif command -v python >/dev/null 2>&1 && python -c 'import sys; sys.exit(0 if sys.version_info[0] >= 3 else 1)' 2>/dev/null; then
    sc_py=python
  fi
  if [[ -n "$sc_py" ]]; then
    eval "$("$sc_py" - "$(sc_config_json)" <<'PY'
import json, sys, shlex
with open(sys.argv[1], encoding="utf-8") as f:
    j = json.load(f)
addr = (j.get("redisAddr") or "127.0.0.1:6379").strip()
host, port = addr, "6379"
if ":" in addr:
    host, port = addr.rsplit(":", 1)
pw = (j.get("redisPassword") or "")
db = int(j.get("redisDb") or 0)
print(f"RedisHost={shlex.quote(host)}")
print(f"RedisPort={shlex.quote(str(port))}")
print(f"RedisPassword={shlex.quote(pw)}")
print(f"RedisDb={shlex.quote(str(db))}")
PY
)"
  fi
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    -H|--host) RedisHost="$2"; shift 2 ;;
    -p|--port) RedisPort="$2"; shift 2 ;;
    --db) RedisDb="$2"; shift 2 ;;
    --pattern) Pattern="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: $0 [-H host] [-p port] [--db n] [--pattern 'sr:*']" >&2
      exit 0
      ;;
    *) echo "Usage: $0 [-H host] [-p port] [--db n]" >&2; exit 1 ;;
  esac
done

redis_bin=""
if [[ -n "${REDIS_CLI:-}" && -x "${REDIS_CLI}" ]]; then
  redis_bin="$REDIS_CLI"
elif command -v redis-cli >/dev/null 2>&1; then
  redis_bin="$(command -v redis-cli)"
else
  HERE="$SCRIPT_DIR"
  # shellcheck source=redis-common.sh
  source "$SCRIPT_DIR/redis-common.sh"
  candidate="$(sc_redis_cli)"
  if command -v "$candidate" >/dev/null 2>&1; then
    redis_bin="$candidate"
  fi
fi
[[ -n "$redis_bin" ]] || { echo "[rebuild-redis] FAIL — 未找到 redis-cli" >&2; exit 1; }

redis_cli() {
  local args=(-h "$RedisHost" -p "$RedisPort" -n "$RedisDb")
  [[ -n "$RedisPassword" ]] && args+=(-a "$RedisPassword")
  "$redis_bin" "${args[@]}" "$@"
}

rebuild_ok() {
  echo "[rebuild-redis] OK — $*"
  exit 0
}

rebuild_fail() {
  echo "[rebuild-redis] FAIL — $*" >&2
  exit 1
}

if ! redis_cli ping 2>/dev/null | grep -q PONG; then
  rebuild_fail "无法连接 Redis ${RedisHost}:${RedisPort} db=${RedisDb}"
fi

count_keys() {
  redis_cli --scan --pattern "$Pattern" 2>/dev/null | wc -l | tr -d ' \r'
}

before="$(count_keys)"
echo "[rebuild-redis] 连接 ${RedisHost}:${RedisPort} db=${RedisDb} pattern=${Pattern}"
echo "[rebuild-redis] 重建前匹配键数量: ${before}"

if [[ "${REDIS_REBUILD_FLUSHDB:-0}" == "1" ]]; then
  echo "[rebuild-redis] REDIS_REBUILD_FLUSHDB=1：执行 FLUSHDB ..."
  redis_cli FLUSHDB >/dev/null
else
  deleted=0
  while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    redis_cli DEL "$key" >/dev/null
    deleted=$((deleted + 1))
  done < <(redis_cli --scan --pattern "$Pattern" 2>/dev/null || true)
  echo "[rebuild-redis] 已删除 ${deleted} 个键"
fi

after="$(count_keys)"
echo "[rebuild-redis] 重建后匹配键数量: ${after}"

if [[ "$after" -ne 0 ]]; then
  rebuild_fail "仍有 ${after} 个键匹配 ${Pattern}"
fi

if [[ -f "$SchemaDoc" ]]; then
  echo ""
  echo "[rebuild-redis] 键结构文档: $SchemaDoc"
  grep -E '^\| `sr:' "$SchemaDoc" 2>/dev/null | head -12 || true
fi

rebuild_ok "Redis 已重建 (db=${RedisDb}, 已清空 ${Pattern})"
