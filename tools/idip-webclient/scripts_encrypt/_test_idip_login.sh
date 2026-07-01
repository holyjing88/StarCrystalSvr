#!/usr/bin/env bash
set -euo pipefail
ROOT="/home/holyjing/starcrystalsvr"
SCRIPTS="/home/holyjing/starcrystalsvr/tools/idip-webclient/scripts_encrypt"
CFG="/home/holyjing/starcrystalsvr/release/configs/starcrystal.json"
API="http://127.0.0.1:8080"
PLAIN="${SCRIPTS}/encrypt/idip-operator.plain.txt"
BACKUP="${CFG}.bak.login-test"
PASS=0
FAIL=0
ok() { echo "[PASS] $1"; PASS=$((PASS + 1)); }
bad() { echo "[FAIL] $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  if [[ -f "${BACKUP}" ]]; then
    mv -f "${BACKUP}" "${CFG}"
    echo "Restored ${CFG} from backup" >&2
  fi
  rm -f "${PLAIN}" "${SCRIPTS}/encrypt/idip-operator.encrypted.json"
}
trap cleanup EXIT

echo "=== IDIP login verification on $(hostname) ==="

cp "${CFG}" "${BACKUP}"

cd "${SCRIPTS}"
bash ./encrypt-idip-operator.sh >/tmp/idip_login_out.json 2>/tmp/idip_login_err.txt

if [[ ! -f "${PLAIN}" ]]; then
  echo "missing plain credentials file" >&2
  exit 1
fi
# shellcheck disable=SC1090
source <(grep -E '^IDIP_OPERATOR_(USERNAME|PASSWORD)=' "${PLAIN}")

if [[ -z "${IDIP_OPERATOR_USERNAME:-}" || -z "${IDIP_OPERATOR_PASSWORD:-}" ]]; then
  echo "failed to read credentials from ${PLAIN}" >&2
  cat "${PLAIN}" >&2
  exit 1
fi
echo "Generated operator: ${IDIP_OPERATOR_USERNAME}" >&2

cd "${ROOT}/release"
pkill -9 -x starcrystalsvr 2>/dev/null || true
sleep 1
rm -f starcrystalsvr.pid
su - holyjing -c 'cd /home/holyjing/starcrystalsvr/release && ./startsvr.sh'
sleep 3

if ! curl -fsS "${API}/healthz" >/dev/null; then
  bad "healthz failed"
  exit 1
fi
ok "server healthz"

# 正确密码
BODY_OK=$(python3 - "${IDIP_OPERATOR_USERNAME}" "${IDIP_OPERATOR_PASSWORD}" <<'PY'
import json, sys, urllib.request
u, p = sys.argv[1], sys.argv[2]
req = urllib.request.Request(
    "http://127.0.0.1:8080/idip/v1/auth/login",
    data=json.dumps({"username": u, "password": p}).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
try:
    with urllib.request.urlopen(req) as resp:
        print(resp.status)
        print(resp.read().decode())
except urllib.error.HTTPError as e:
    print(e.code)
    print(e.read().decode())
PY
)
HTTP_OK=$(echo "${BODY_OK}" | sed -n '1p')
JSON_OK=$(echo "${BODY_OK}" | sed '1d')
if [[ "${HTTP_OK}" == "200" ]] && echo "${JSON_OK}" | grep -q '"code":0'; then
  ok "login with correct password (HTTP 200, code=0)"
else
  bad "login with correct password (HTTP=${HTTP_OK} body=${JSON_OK})"
fi
TOKEN=$(echo "${JSON_OK}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("sessionToken",""))' 2>/dev/null || true)
if [[ -n "${TOKEN}" ]]; then
  ok "sessionToken returned"
else
  bad "missing sessionToken"
fi

# 错误密码
BODY_BAD=$(python3 - "${IDIP_OPERATOR_USERNAME}" <<'PY'
import json, sys, urllib.request
u = sys.argv[1]
req = urllib.request.Request(
    "http://127.0.0.1:8080/idip/v1/auth/login",
    data=json.dumps({"username": u, "password": "wrong-password-xyz"}).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
try:
    with urllib.request.urlopen(req) as resp:
        print(resp.status)
        print(resp.read().decode())
except urllib.error.HTTPError as e:
    print(e.code)
    print(e.read().decode())
PY
)
HTTP_BAD=$(echo "${BODY_BAD}" | sed -n '1p')
JSON_BAD=$(echo "${BODY_BAD}" | sed '1d')
if [[ "${HTTP_BAD}" == "401" ]] && echo "${JSON_BAD}" | grep -q '"code":1401'; then
  ok "login with wrong password (HTTP 401, code=1401)"
else
  bad "wrong password expected 401/1401 got HTTP=${HTTP_BAD} body=${JSON_BAD}"
fi

# 错误用户名
LOGIN_USER=$(curl -sS -w "\n%{http_code}" -X POST "${API}/idip/v1/auth/login"   -H 'Content-Type: application/json'   -d '{"username":"nonexistent_ops_user","password":"anything"}')
HTTP_USER=$(echo "${LOGIN_USER}" | tail -n1)
BODY_USER=$(echo "${LOGIN_USER}" | sed '$d')
if [[ "${HTTP_USER}" == "401" ]] && echo "${BODY_USER}" | grep -q '"code":1401'; then
  ok "login with unknown username (HTTP 401, code=1401)"
else
  bad "unknown username expected 401/1401 got HTTP=${HTTP_USER} body=${BODY_USER}"
fi

# 心跳
if [[ -n "${TOKEN}" ]]; then
  HB=$(curl -sS -w "\n%{http_code}" -X POST "${API}/idip/v1/auth/heartbeat"     -H "X-IDIP-Session: ${TOKEN}" -H 'Content-Type: application/json' -d '{}')
  HTTP_HB=$(echo "${HB}" | tail -n1)
  if [[ "${HTTP_HB}" == "200" ]]; then
    ok "heartbeat with valid session"
  else
    bad "heartbeat failed HTTP=${HTTP_HB}"
  fi
fi

echo "========== RESULT: ${PASS} passed, ${FAIL} failed =========="
[[ "${FAIL}" -eq 0 ]]
echo REMOTE_IDIP_LOGIN_TEST_OK
