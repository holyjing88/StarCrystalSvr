#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${SCRIPT_DIR}/encrypt-idip-operator.sh"
PLAIN="${SCRIPT_DIR}/encrypt/idip-operator.plain.txt"
ENC="${SCRIPT_DIR}/encrypt/idip-operator.encrypted.json"
PASS=0
FAIL=0
TMP_CFG=""
ok() { echo "[PASS] $1"; PASS=$((PASS + 1)); }
bad() { echo "[FAIL] $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  rm -f "${TMP_CFG}" /tmp/idip_out.txt /tmp/idip_err.txt
  rm -f "${PLAIN}" "${ENC}"
}
trap cleanup EXIT

echo "=== acceptance: $(uname -a) ==="
echo "=== openssl: $(openssl version) ==="

TMP_CFG="$(mktemp)"
printf '%s\n' '{"keepMe":true,"idip":{"legacy":1}}' >"${TMP_CFG}"

if IDIP_CONFIG_PATH="${TMP_CFG}" bash "${SCRIPT}" >/tmp/idip_out.txt 2>/tmp/idip_err.txt; then
  ok "encrypt-idip-operator.sh exit 0"
else
  bad "encrypt-idip-operator.sh failed"
  cat /tmp/idip_err.txt || true
fi

if grep -q '"passwordEnc": "v1:' /tmp/idip_out.txt; then
  ok "stdout passwordEnc v1:"
else
  bad "stdout missing v1 passwordEnc"
  cat /tmp/idip_out.txt || true
fi

if [[ -f "${PLAIN}" && -f "${ENC}" ]]; then
  ok "local plain + encrypted files written"
else
  bad "missing idip-operator.plain.txt or idip-operator.encrypted.json"
fi

if [[ -f "${PLAIN}" ]] && grep -q '^IDIP_OPERATOR_PASSWORD=' "${PLAIN}"; then
  ok "plain file has password"
else
  bad "plain file missing password"
fi

if [[ -f "${ENC}" ]] && grep -q '"passwordEnc": "v1:' "${ENC}"; then
  ok "encrypted file has passwordEnc"
else
  bad "encrypted file invalid"
fi

if jq -e '.keepMe == true and .idip.legacy == 1 and .idip.operatorCipherKey != null and (.idip.operators | length) == 1' "${TMP_CFG}" >/dev/null 2>&1; then
  ok "config merge preserves other fields"
elif python3 - "${TMP_CFG}" <<'PY' >/dev/null 2>&1; then
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    d = json.load(f)
assert d.get("keepMe") is True
assert d.get("idip", {}).get("legacy") == 1
assert d.get("idip", {}).get("operatorCipherKey")
assert len(d.get("idip", {}).get("operators") or []) == 1
PY
  ok "config merge preserves other fields"
else
  bad "config merge failed or clobbered other fields"
  cat "${TMP_CFG}" || true
fi

if [[ -x "${SCRIPT_DIR}/bin/encrypt-idip-operator" ]]; then
  ok "bin extracted and executable"
else
  bad "bin missing or not executable"
fi

file "${SCRIPT_DIR}/bin/encrypt-idip-operator" || true
echo "========== RESULT: ${PASS} passed, ${FAIL} failed =========="
[[ "${FAIL}" -eq 0 ]]
