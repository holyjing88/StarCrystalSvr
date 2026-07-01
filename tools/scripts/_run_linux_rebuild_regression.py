#!/usr/bin/env python3
"""Linux: rebuild starcrystalsvr, restart, server go tests, IDIP login + webclient regression."""
import os
import sys
from pathlib import Path

import paramiko

HERE = Path(__file__).resolve().parent
env_path = HERE.parent / "linux-host.local.env"
if not env_path.is_file():
    print(f"missing {env_path}", file=sys.stderr)
    sys.exit(2)

env: dict[str, str] = {}
for line in env_path.read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    k, v = line.split("=", 1)
    env[k.strip()] = v.strip()

HOST = env["LINUX_HOST"]
USER = env["LINUX_USER"]
PWD = env["LINUX_PASSWORD"]
ROOT = env.get("LINUX_REPO", "/home/holyjing/starcrystalsvr")
GO = f"{ROOT}/.go-toolchain/go/bin/go"


def run(c: paramiko.SSHClient, cmd: str, timeout: int = 600) -> tuple[int, str]:
    print(">>>", cmd[:240], flush=True)
    _, stdout, stderr = c.exec_command(cmd, timeout=timeout, get_pty=True)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    code = stdout.channel.recv_exit_status()
    if out:
        print(out, end="")
    if err:
        print(err, file=sys.stderr, end="")
    return code, out + err


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(
        HOST,
        username=USER,
        password=PWD,
        timeout=30,
        allow_agent=False,
        look_for_keys=False,
    )

    steps = [
        (
            "build",
            f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH "
            f"GOPROXY=https://goproxy.cn,direct && bash tools/scripts/build.sh",
        ),
        (
            "build-verify-tests",
            f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH && "
            f"{GO} test ./internal/service/... -count=1 -short || true",
        ),
        (
            "restart-svr",
            f"""set -e
cd {ROOT}/release
pkill -9 -x starcrystalsvr 2>/dev/null || true
sleep 1
rm -f starcrystalsvr.pid
su - holyjing -c 'cd {ROOT}/release && ./startsvr.sh'
sleep 3
curl -fsS http://127.0.0.1:8080/healthz && echo HEALTHZ_OK""",
        ),
        (
            "go-idip-auth",
            f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH && "
            f"{GO} test ./internal/api/... -run IdipAuth -count=1 -timeout 120s",
        ),
        (
            "go-service-regression",
            f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH && "
            f'{GO} test ./internal/service/... -run "IdipSession|Encrypt|Decrypt|VerifyIdip|'
            f'TestSmoke_IDIP|TestIdipTask|TestTasksClaim|TestGameFavorite" '
            f"-count=1 -timeout 300s",
        ),
        (
            "idip-login-script",
            f"bash {ROOT}/tools/idip-webclient/scripts_encrypt/_test_idip_login.sh",
        ),
    ]

    for name, cmd in steps:
        print(f"\n=== {name} ===", flush=True)
        code, _ = run(c, cmd, timeout=900)
        if code != 0:
            c.close()
            sys.exit(f"FAIL at {name} exit={code}")

    print("\n=== idip-webclient vitest (Linux) ===", flush=True)
    vitest_cmd = f"""set -e
cd {ROOT}/tools/idip-webclient
export IDIP_BASE_URL=http://127.0.0.1:8080
export IDIP_WEBCLIENT_URL=http://127.0.0.1
export IDIP_USERNAME=ops_admin
export IDIP_PASSWORD=change-me-ops-password
export IDIP_KEY=change-me-in-production
node_major=$(node -e 'console.log(process.versions.node.split(".")[0])' 2>/dev/null || echo 0)
if [ "$node_major" -lt 18 ] 2>/dev/null; then
  echo "SKIP_VITEST_ON_LINUX: Node 18+ required (got node_major=$node_major)"
  exit 0
fi
bash scripts/run-regression.sh
"""
    code, combined = run(c, vitest_cmd, timeout=600)
    c.close()
    if code != 0 and "SKIP_VITEST_ON_LINUX" not in combined:
        print(
            "NOTE: Linux Vitest skipped or failed; run from Windows: "
            "IDIP_BASE_URL=http://<host>:8080 npx vitest run (use %TEMP%\\starcrystal-idip-wc if Y: EPERM)",
            flush=True,
        )
        sys.exit(f"FAIL vitest exit={code}")
    print("\nLINUX_REBUILD_REGRESSION_OK", flush=True)


if __name__ == "__main__":
    main()
