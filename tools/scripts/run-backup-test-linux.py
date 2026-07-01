#!/usr/bin/env python3
"""Sync and run dbscripts backup test on Linux host."""
import os
import paramiko
import sys

ENV_PATH = r"Y:\holyjing\starcrystalsvr\tools\linux-host.local.env"
DEV_DBSCRIPTS = r"d:\0_games\000StarCrystal\server\tools\scripts\dbscripts"
TEST_SCRIPT = "test-backup-linux.sh"


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


def main() -> int:
    env = load_env(ENV_PATH)
    repo = env["LINUX_REPO"]
    remote = f"{repo}/tools/scripts/dbscripts/{TEST_SCRIPT}"
    local = os.path.join(DEV_DBSCRIPTS, TEST_SCRIPT)

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        env["LINUX_HOST"],
        username=env["LINUX_USER"],
        password=env["LINUX_PASSWORD"],
        timeout=30,
        allow_agent=False,
        look_for_keys=False,
    )

    print(f"svn update {repo} ...")
    _, stdout, _ = client.exec_command(f"cd {repo} && svn update -q tools/scripts/dbscripts", timeout=120)
    stdout.channel.recv_exit_status()

    sftp = client.open_sftp()
    data = open(local, "rb").read().replace(b"\r\n", b"\n")
    tmp = local + ".lf"
    open(tmp, "wb").write(data)
    sftp.put(tmp, remote)
    sftp.close()
    os.remove(tmp)

    cmd = f"cd {repo} && export LANG=C LC_ALL=C && bash tools/scripts/dbscripts/{TEST_SCRIPT}"
    print(f"Run backup test on {env['LINUX_USER']}@{env['LINUX_HOST']} ...")
    _, stdout, stderr = client.exec_command(cmd, get_pty=True, timeout=300)
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
