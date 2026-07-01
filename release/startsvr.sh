#!/usr/bin/env bash
# Start starcrystalsvr in this directory (release/ or starcrystal-release/).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
cd "$ROOT"

BIN="$ROOT/starcrystalsvr"
if [[ ! -x "$BIN" ]]; then
  echo "[startsvr] FAIL — 缺少可执行文件 $BIN" >&2
  exit 1
fi

export GAMES_CONFIG="$ROOT/configs/games.json"
export CHANNEL_TEXTS_CONFIG="$ROOT/configs/channel_texts.json"
export STARCrystal_CONFIG="$ROOT/configs/starcrystal.json"
export AUTH_SMS_MOCK="${AUTH_SMS_MOCK:-1}"
# automation: do not disable ad rewards
export REG_ACCOUNTS_PER_IP_PER_DAY="${REG_ACCOUNTS_PER_IP_PER_DAY:-0}"
export REG_ACCOUNTS_PER_DEVICE_PER_DAY="${REG_ACCOUNTS_PER_DEVICE_PER_DAY:-0}"
unset API_ADDR
mkdir -p "$ROOT/log"

CONFIG_JSON="$ROOT/configs/starcrystal.json"
LISTEN_URL="http://0.0.0.0:8080"
if [[ -f "$CONFIG_JSON" ]]; then
  _cfg="$(python3 - "$CONFIG_JSON" <<'PY' 2>/dev/null || true
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    j = json.load(f)
host = (j.get("apiListenHost") or "").strip() or "0.0.0.0"
port = int(j.get("apiListenPort") or 8080) or 8080
scheme = "https" if j.get("useHttps") else "http"
dsn = (j.get("authMysqlDsn") or "").strip()
print(f"{scheme}://{host}:{port}")
print(dsn)
PY
)"
  if [[ -n "$_cfg" ]]; then
    LISTEN_URL="$(sed -n '1p' <<<"$_cfg")"
    [[ -z "$LISTEN_URL" ]] && LISTEN_URL="http://0.0.0.0:8080"
    if [[ -z "${AUTH_MYSQL_DSN:-}" ]]; then
      _dsn="$(sed -n '2p' <<<"$_cfg")"
      [[ -n "$_dsn" ]] && export AUTH_MYSQL_DSN="$_dsn"
    fi
  fi
fi

PID_FILE="${PID_FILE:-$ROOT/starcrystalsvr.pid}"
if [[ -f "$PID_FILE" ]]; then
  old="$(tr -d '\r\n' <"$PID_FILE")"
  if [[ -n "$old" ]] && kill -0 "$old" 2>/dev/null; then
    echo "[startsvr] OK — 已在运行 (PID=$old, URL=$LISTEN_URL)"
    exit 0
  fi
  rm -f "$PID_FILE"
fi

echo "[startsvr] 启动 $BIN (URL ~ $LISTEN_URL) ..."
nohup "$BIN" >>"$ROOT/log/starcrystalsvr.stdout.log" 2>>"$ROOT/log/starcrystalsvr.stderr.log" &
echo "$!" >"$PID_FILE"
echo "[startsvr] OK — PID=$(tr -d '\r\n' <"$PID_FILE"), URL=$LISTEN_URL, logs=log/"
