#!/usr/bin/env python3
"""Restore UTF-8 shell scripts, sync to Linux, run acceptance."""
from __future__ import annotations

import os
import sys

import paramiko

HOST = os.environ.get("LINUX_TEST_HOST", "192.168.75.99")
USER = os.environ.get("STARCRYSTAL_LINUX_SSH_USER", "root")
PWD = os.environ.get("STARCRYSTAL_LINUX_SSH_PASSWORD", "")
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REMOTE = os.environ.get(
    "LINUX_IDIP_WEBCLIENT_SCRIPTS_ENCRYPT",
    "/home/holyjing/starcrystalsvr/tools/idip-webclient/scripts_encrypt",
)

ACCEPT_SH = r"""#!/usr/bin/env bash
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
"""


def write_utf8(name: str, content: str) -> str:
    path = os.path.join(SCRIPT_DIR, name)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)
    raw = open(path, "rb").read()
    if raw.startswith(b"\xef\xbb\xbf"):
        raw = raw[3:]
    if not raw.startswith(b"#!/"):
        raise RuntimeError(f"{name} not valid UTF-8 shell script")
    return path


def ensure_gz_binary() -> tuple[str, str]:
    import gzip

    bin_path = os.path.join(SCRIPT_DIR, "bin", "encrypt-idip-operator")
    gz_path = bin_path + ".gz"
    if not os.path.isfile(bin_path) or os.path.getsize(bin_path) < 1000:
        raise RuntimeError(f"missing or corrupt binary: {bin_path}")
    with open(bin_path, "rb") as src, gzip.open(gz_path, "wb", compresslevel=9) as dst:
        dst.write(src.read())
    if os.path.getsize(gz_path) < 1000:
        raise RuntimeError(f"failed to create {gz_path}")
    return bin_path, gz_path


def sftp_put(sftp: paramiko.SFTPClient, local: str, remote: str) -> None:
    with open(local, "rb") as f:
        data = f.read()
    with sftp.open(remote, "wb") as rf:
        rf.write(data)


def main() -> int:
    if not PWD:
        print("STARCRYSTAL_LINUX_SSH_PASSWORD not set", file=sys.stderr)
        return 2

    encrypt_path = os.path.join(SCRIPT_DIR, "encrypt-idip-operator.sh")
    if not os.path.isfile(encrypt_path):
        print(f"missing {encrypt_path}", file=sys.stderr)
        return 2
    with open(encrypt_path, "rb") as f:
        head = f.read(4)
        if head.startswith(b"\xef\xbb\xbf"):
            head = head[3:] + f.read(1)
        if not head.startswith(b"#!/"):
            print(f"{encrypt_path} is not a valid shell script", file=sys.stderr)
            return 2
    write_utf8("_accept_encrypt_idip_operator.sh", ACCEPT_SH)
    _, gz_path = ensure_gz_binary()
    bin_path = os.path.join(SCRIPT_DIR, "bin", "encrypt-idip-operator")
    print("Wrote UTF-8 scripts locally")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PWD, timeout=30, allow_agent=False, look_for_keys=False)

    sftp = client.open_sftp()
    try:
        try:
            sftp.stat(f"{REMOTE}/bin")
        except OSError:
            sftp.mkdir(f"{REMOTE}/bin")
        for rel in ("encrypt-idip-operator.sh", "_accept_encrypt_idip_operator.sh"):
            sftp_put(sftp, os.path.join(SCRIPT_DIR, rel), f"{REMOTE}/{rel}")
            print(f"UPLOAD {rel}")
        gz = gz_path
        if not os.path.isfile(gz):
            print(f"MISSING {gz}", file=sys.stderr)
            return 1
        sftp_put(sftp, gz, f"{REMOTE}/bin/encrypt-idip-operator.gz")
        print("UPLOAD bin/encrypt-idip-operator.gz")
        sftp_put(sftp, bin_path, f"{REMOTE}/bin/encrypt-idip-operator")
        print("UPLOAD bin/encrypt-idip-operator")
    finally:
        sftp.close()

    cmd = f"""
set -e
cd {REMOTE}
chmod +x encrypt-idip-operator.sh _accept_encrypt_idip_operator.sh bin/encrypt-idip-operator
bash _accept_encrypt_idip_operator.sh
echo REMOTE_ENCRYPT_ACCEPTANCE_OK
"""
    print(">>> remote acceptance")
    _, stdout, stderr = client.exec_command(cmd, timeout=120)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    code = stdout.channel.recv_exit_status()
    if out:
        print(out, end="")
    if err:
        print(err, file=sys.stderr, end="")
    client.close()

    if code != 0 or "REMOTE_ENCRYPT_ACCEPTANCE_OK" not in out:
        print(f"FAIL exit={code}", file=sys.stderr)
        return 1
    print("ALL_OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
