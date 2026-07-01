#!/usr/bin/env python3
"""Run dbscripts Linux test via SSH (reads tools/linux-host.local.env on Y:)."""
import os
import sys

try:
    import paramiko
except ImportError:
    print("paramiko required", file=sys.stderr)
    sys.exit(2)

ENV_PATH = r"Y:\holyjing\starcrystalsvr\tools\linux-host.local.env"
DEV_DBSCRIPTS = r"d:\0_games\000StarCrystal\server\tools\scripts\dbscripts"


def load_env(path: str) -> dict:
    out = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.split("#", 1)[0].strip()
            if not line or "=" not in line:
                continue
            k, v = line.split("=", 1)
            out[k.strip()] = v.strip().strip('"').strip("'")
    return out


def sftp_sync_dir(sftp, local_root: str, remote_root: str) -> None:
    for root, _, files in os.walk(local_root):
        rel = os.path.relpath(root, local_root).replace("\\", "/")
        remote_dir = remote_root if rel == "." else f"{remote_root}/{rel}"
        try:
            sftp.stat(remote_dir)
        except OSError:
            parts = remote_dir.strip("/").split("/")
            cur = ""
            for p in parts:
                cur = f"{cur}/{p}" if cur else f"/{p}"
                try:
                    sftp.stat(cur)
                except OSError:
                    sftp.mkdir(cur)
        for name in files:
            lp = os.path.join(root, name)
            rp = f"{remote_dir}/{name}"
            sftp.put(lp, rp)


def main() -> int:
    env = load_env(ENV_PATH)
    host = env["LINUX_HOST"]
    user = env["LINUX_USER"]
    password = env["LINUX_PASSWORD"]
    repo = env.get("LINUX_REPO", "/home/holyjing/starcrystalsvr")
    redis_port = env.get("DOCKER_REDIS_PORT", "16379")

    remote_dbscripts = f"{repo}/tools/scripts/dbscripts"
    cmd = (
        f"set -e; cd {repo}; "
        f"export LANG=C.UTF-8; "
        f"echo '=== uname ==='; uname -a; "
        f"echo '=== static test ==='; "
        f"bash tools/scripts/dbscripts/test-dbscripts-linux.sh; "
        f"echo '=== live test (REDIS_PORT={redis_port}) ==='; "
        f"SCRIPTS_TEST_LIVE=1 REDIS_PORT={redis_port} "
        f"bash tools/scripts/dbscripts/test-dbscripts-linux.sh"
    )

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        host,
        username=user,
        password=password,
        timeout=30,
        allow_agent=False,
        look_for_keys=False,
    )

    print(f"Sync dbscripts -> {remote_dbscripts} ...")
    sftp = client.open_sftp()
    sftp_sync_dir(sftp, DEV_DBSCRIPTS, remote_dbscripts)
    sftp.close()

    print(f"Run tests on {user}@{host} ...")
    stdin, stdout, stderr = client.exec_command(cmd, get_pty=True, timeout=600)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    code = stdout.channel.recv_exit_status()
    if out:
        sys.stdout.write(out)
    if err:
        sys.stderr.write(err)
    client.close()
    return code


if __name__ == "__main__":
    sys.exit(main())
