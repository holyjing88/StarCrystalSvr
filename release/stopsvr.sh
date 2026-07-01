#!/usr/bin/env bash
# Stop starcrystalsvr in this directory (release/ or starcrystal-release/).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
PID_FILE="${PID_FILE:-$ROOT/starcrystalsvr.pid}"

stop_pid() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -0 "$pid" 2>/dev/null || return 0
  kill "$pid" 2>/dev/null || true
  local i
  for i in $(seq 1 10); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  kill -9 "$pid" 2>/dev/null || true
}

stopped=0
if [[ -f "$PID_FILE" ]]; then
  pid="$(tr -d '\r\n' <"$PID_FILE")"
  if [[ -n "$pid" ]]; then
    echo "[stopsvr] 停止 PID=$pid ..."
    stop_pid "$pid"
    stopped=1
  fi
  rm -f "$PID_FILE"
fi

while read -r pid; do
  [[ -z "$pid" ]] && continue
  echo "[stopsvr] 停止 starcrystalsvr PID=$pid ..."
  stop_pid "$pid"
  stopped=1
done < <(pgrep -x starcrystalsvr 2>/dev/null || true)

if [[ "$stopped" -eq 0 ]]; then
  echo "[stopsvr] OK — 未在运行"
  exit 0
fi

echo "[stopsvr] OK — 已停止"
