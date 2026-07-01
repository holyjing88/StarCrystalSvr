#!/usr/bin/env python3
"""Linux IDIP 上线前验收：encrypt.sh、ELF 编译、startsvr、login、go test。"""
import json
import os
import re
import sys

import paramiko

HOST = os.environ.get("LINUX_TEST_HOST", "192.168.75.99")
USER = os.environ.get("STARCRYSTAL_LINUX_SSH_USER", "root")
PWD = os.environ.get("STARCRYSTAL_LINUX_SSH_PASSWORD", "")
ROOT = os.environ.get("LINUX_DEPLOY_DIR", "/home/holyjing/starcrystalsvr")
GO = f"{ROOT}/.go-toolchain/go/bin/go"
LOCAL_ROOT = os.environ.get(
    "STARCrystal_LOCAL_ROOT",
    r"Y:\holyjing\starcrystalsvr",
)


def sftp_sync_tree(sftp, local_dir, remote_dir):
    local_dir = os.path.normpath(local_dir)
    if not os.path.isdir(local_dir):
        return
    try:
        sftp.stat(remote_dir)
    except OSError:
        sftp.mkdir(remote_dir)
    for name in os.listdir(local_dir):
        lp = os.path.join(local_dir, name)
        rp = remote_dir + "/" + name
        if os.path.isdir(lp):
            sftp_sync_tree(sftp, lp, rp)
        else:
            sftp.put(lp, rp)


def sync_sources(c):
    """SVN update 失败时，从 Windows 工作副本同步 Go 源码到 Linux。"""
    if not os.path.isdir(LOCAL_ROOT):
        print(f"skip source sync: {LOCAL_ROOT} missing", flush=True)
        return
    rel_dirs = [
        "cmd/api",
        "cmd/encrypt-idip-operator",
        "internal/api",
        "internal/service",
        "internal/config",
    ]
    rel_files = ["go.mod", "go.sum"]
    sftp = c.open_sftp()
    try:
        for d in rel_dirs:
            sftp_sync_tree(
                sftp, os.path.join(LOCAL_ROOT, d), f"{ROOT}/{d.replace(os.sep, '/')}"
            )
        for f in rel_files:
            lp = os.path.join(LOCAL_ROOT, f)
            if os.path.isfile(lp):
                sftp.put(lp, f"{ROOT}/{f}")
    finally:
        sftp.close()
    print("SOURCE_SYNC_OK", flush=True)


def run(c, cmd, timeout=600, must_ok=True):
    print(">>>", cmd, flush=True)
    _, o, e = c.exec_command(cmd, timeout=timeout)
    out = o.read().decode("utf-8", "replace")
    err = e.read().decode("utf-8", "replace")
    code = o.channel.recv_exit_status()
    if out:
        print(out, end="")
    if err:
        print(err, file=sys.stderr, end="")
    if must_ok and code != 0:
        sys.exit(f"FAIL exit={code}: {cmd[:160]}")
    return code, out + err


def main():
    if not PWD:
        print("STARCRYSTAL_LINUX_SSH_PASSWORD not set", file=sys.stderr)
        sys.exit(2)

    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST, username=USER, password=PWD, timeout=30, allow_agent=False, look_for_keys=False)

    run(c, f"cd {ROOT} && svn update -q 2>&1 | tail -5", must_ok=False)

    cfg_path = f"{ROOT}/release/configs/starcrystal.json"
    cipher = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
    run(
        c,
        f"""set -e
python3 - <<'PY'
import json, subprocess
cfg_path = "{cfg_path}"
cipher = "{cipher}"
root = "{ROOT}"
out = subprocess.check_output([
    "go", "run", "./cmd/encrypt-idip-operator",
    "-username", "ops_admin",
    "-password", "change-me-ops-password",
    "-cipher-key-base64", cipher,
], cwd=root, universal_newlines=True)
print(out)
op = json.loads(out)
with open(cfg_path, encoding="utf-8-sig") as f:
    cfg = json.load(f)
idip = cfg.setdefault("idip", {{}})
idip["operatorCipherKey"] = cipher
idip["operators"] = [{{"username": op["username"], "passwordEnc": op["passwordEnc"]}}]
with open(cfg_path, "w", encoding="utf-8", newline="\\n") as f:
    json.dump(cfg, f, ensure_ascii=False, indent=2)
    f.write("\\n")
print("Updated", cfg_path)
PY
""",
    )

    _, cfg_txt = run(
        c,
        f"python3 -c \"import json; j=json.load(open('{cfg_path}', encoding='utf-8-sig')); "
        f"print(json.dumps(j.get('idip',{{}}).get('operators',[])))\"",
    )
    last_line = [ln for ln in cfg_txt.strip().splitlines() if ln.strip()][-1]
    ops = json.loads(last_line)
    if not ops or not str(ops[0].get("passwordEnc", "")).startswith("v1:"):
        sys.exit("FAIL: operators passwordEnc missing")

    run(c, f"cd {ROOT} && {GO} build -o release/starcrystalsvr ./cmd/api/")
    _, file_out = run(c, f"file {ROOT}/release/starcrystalsvr")
    if "ELF" not in file_out:
        sys.exit("FAIL: release/starcrystalsvr is not Linux ELF")

    run(
        c,
        f"""set -e
cd {ROOT}/release
pkill -9 -x starcrystalsvr 2>/dev/null || true
sleep 1
rm -f starcrystalsvr.pid
su - holyjing -c 'cd {ROOT}/release && ./startsvr.sh'
sleep 3
curl -fsS http://127.0.0.1:8080/healthz
""",
    )

    _, login = run(
        c,
        """curl -fsS -X POST http://127.0.0.1:8080/idip/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"ops_admin","password":"change-me-ops-password"}'""",
    )
    m = re.search(r"\{.*\}", login, re.S)
    if not m:
        sys.exit("FAIL login: no JSON")
    data = json.loads(m.group())
    if data.get("code") != 0:
        sys.exit(f"FAIL login code={data.get('code')} msg={data.get('message')}")
    tok = (data.get("data") or {}).get("sessionToken") or ""
    if not tok:
        sys.exit("FAIL: no sessionToken")

    run(
        c,
        f"curl -fsS -X POST http://127.0.0.1:8080/idip/v1/auth/heartbeat "
        f"-H 'X-IDIP-Session: {tok}' -H 'Content-Type: application/json' -d '{{}}'",
    )

    run(
        c,
        f"cd {ROOT} && {GO} test ./internal/service/... "
        f'-run "IdipSession|Encrypt|Decrypt|VerifyIdip" -count=1 -timeout 120s',
    )
    run(c, f"cd {ROOT} && {GO} test ./internal/api/... -run IdipAuth -count=1 -timeout 120s")
    run(c, f"cd {ROOT} && {GO} test ./cmd/encrypt-idip-operator/... -count=1 -timeout 120s")

    c.close()
    print("REMOTE_ACCEPTANCE_OK")


if __name__ == "__main__":
    main()
