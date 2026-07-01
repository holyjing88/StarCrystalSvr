#!/usr/bin/env python3
"""Deprecated: do not svn update on Linux. Sync via Windows WC (Y:\\holyjing\\starcrystalsvr) + SMB."""
import paramiko
from pathlib import Path

env = {}
for line in Path(__file__).resolve().parent.parent.joinpath("linux-host.local.env").read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if line and not line.startswith("#") and "=" in line:
        k, v = line.split("=", 1)
        env[k.strip()] = v.strip()

repo = env["LINUX_REPO"]

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(
    env["LINUX_HOST"],
    username=env["LINUX_USER"],
    password=env["LINUX_PASSWORD"],
    timeout=30,
    allow_agent=False,
    look_for_keys=False,
)
_, o, e = c.exec_command(
    f"test -d {repo} && ls -la {repo}/release/starcrystalsvr 2>/dev/null | head -1; "
    f"command -v svn >/dev/null && echo 'WARN: svn still installed' || echo 'OK: no svn on Linux'",
    timeout=60,
)
print((o.read() + e.read()).decode())
print("Use Windows: cd Y:\\holyjing\\starcrystalsvr && svn update")
c.close()
