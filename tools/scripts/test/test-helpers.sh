#!/usr/bin/env bash
# shellcheck disable=SC2034
# Source from test-scripts.sh

sc_test_tcp_open() {
  local host="${1:-127.0.0.1}" port="$2"
  if command -v nc >/dev/null 2>&1; then
    nc -z "$host" "$port" 2>/dev/null
    return $?
  fi
  timeout 1 bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null
}

sc_test_bash_syntax() {
  local f="$1"
  bash -n "$f"
}

sc_test_result_line() {
  local mark="$1" id="$2" msg="${3:-ok}" dur="${4:-0}"
  if [[ "$dur" -gt 0 ]]; then
    echo "[${mark}] ${id} (${dur}ms) — ${msg}"
  else
    echo "[${mark}] ${id} — ${msg}"
  fi
}

sc_test_report() {
  local pass=0 fail=0 skip=0
  # shellcheck disable=SC2206
  local lines=("$@")
  echo ""
  echo "=== Script acceptance ==="
  for line in "${lines[@]}"; do
    echo "$line"
    case "$line" in
      \[PASS\]*) pass=$((pass + 1)) ;;
      \[FAIL\]*) fail=$((fail + 1)) ;;
      \[SKIP\]*) skip=$((skip + 1)) ;;
    esac
  done
  echo ""
  echo "Total: ${pass} pass, ${fail} fail, ${skip} skip / $((pass + fail + skip))"
  [[ "$fail" -eq 0 ]]
}
