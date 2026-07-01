#!/usr/bin/env python3
"""Linux: encrypt-idip-operator.sh + starcrystalsvr login 账号密码验证。"""
from __future__ import annotations

import json
import os
import re
import sys

import paramiko

HOST = os.environ.get("LINUX_TEST_HOST", "192.168.75.99")
USER = os.environ.get("STARCRYSTAL_LINUX_SSH_USER", "root")
PWD = os.environ.get("STARCRYSTAL_LINUX_SSH_PASSWORD", "")
ROOT = os.environ.get("LINUX_DEPLOY_DIR", "/home/holyjing/starcrystalsvr")
SCRIPTS = f"{ROOT}/tools/idip-webclient/scripts_encrypt"
CFG = f"{ROOT}/release/configs/starcrystal.json"
API = "http://127.0.0.1:8080"


def run(client: paramiko.SSHClient, cmd: str, timeout: int = 180) -> tuple[int, str]:
    print(f">>> {cmd[:200]}{'...' if len(cmd) > 200 else ''}", flush=True)
    _, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    code = stdout.channel.recv_exit_status()
    combined = out + err
    if combined.strip():
        print(combined, end="" if combined.endswith("\n") else "\n", flush=True)
    return code, combined


def parse_json_response(text: str) -> dict:
    m = re.search(r"\{.*\}", text, re.S)
    if not m:
        raise ValueError(f"no JSON in response: {text[:500]!r}")
    return json.loads(m.group())


def main() -> int:
    if not PWD:
        print("STARCRYSTAL_LINUX_SSH_PASSWORD not set", file=sys.stderr)
        return 2

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PWD, timeout=30, allow_agent=False, look_for_keys=False)

    test_sh = """#!/usr/bin/env bash
set -euo pipefail
ROOT="__ROOT__"
SCRIPTS="__SCRIPTS__"
CFG="__CFG__"
API="__API__"
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
su - holyjing -c 'cd __ROOT__/release && ./startsvr.sh'
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
LOGIN_USER=$(curl -sS -w "\\n%{http_code}" -X POST "${API}/idip/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"nonexistent_ops_user","password":"anything"}')
HTTP_USER=$(echo "${LOGIN_USER}" | tail -n1)
BODY_USER=$(echo "${LOGIN_USER}" | sed '$d')
if [[ "${HTTP_USER}" == "401" ]] && echo "${BODY_USER}" | grep -q '"code":1401'; then
  ok "login with unknown username (HTTP 401, code=1401)"
else
  bad "unknown username expected 401/1401 got HTTP=${HTTP_USER} body=${BODY_USER}"
fi

# 心跳
if [[ -n "${TOKEN}" ]]; then
  HB=$(curl -sS -w "\\n%{http_code}" -X POST "${API}/idip/v1/auth/heartbeat" \
    -H "X-IDIP-Session: ${TOKEN}" -H 'Content-Type: application/json' -d '{}')
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
"""
    test_sh = (
        test_sh.replace("__ROOT__", ROOT)
        .replace("__SCRIPTS__", SCRIPTS)
        .replace("__CFG__", CFG)
        .replace("__API__", API)
    )

    sftp = client.open_sftp()
    remote_test = f"{SCRIPTS}/_test_idip_login.sh"
    with sftp.open(remote_test, "w") as f:
        f.write(test_sh)
    sftp.close()

    code, out = run(client, f"chmod +x {remote_test} && bash {remote_test}", timeout=300)
    client.close()

    if code != 0 or "REMOTE_IDIP_LOGIN_TEST_OK" not in out:
        print(f"FAIL exit={code}", file=sys.stderr)
        return 1
    print("ALL_OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
