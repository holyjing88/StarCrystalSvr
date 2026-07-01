#!/usr/bin/env python3
"""Patch Go sources on Linux repo (SMB may deny writes on internal/)."""
from pathlib import Path

import paramiko

env: dict[str, str] = {}
for line in Path(__file__).resolve().parent.parent.joinpath("linux-host.local.env").read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if line and not line.startswith("#") and "=" in line:
        k, v = line.split("=", 1)
        env[k.strip()] = v.strip()

ROOT = env.get("LINUX_REPO", "/home/holyjing/starcrystalsvr")
LOCAL = Path(__file__).resolve().parents[2]

patches = [
    (
        "internal/service/h5_upload.go",
        "\tif err := os.Rename(extracted, destDir); err != nil {",
        (
            "\tif !isUpdate {\n"
            "\t\tif st, statErr := os.Stat(destDir); statErr == nil && st.IsDir() {\n"
            "\t\t\t_ = os.RemoveAll(destDir)\n"
            "\t\t}\n"
            "\t}\n"
            "\tif err := os.Rename(extracted, destDir); err != nil {"
        ),
    ),
    (
        "internal/api/idip_auth_handlers.go",
        '\tw.Header().Set("X-IDIP-Session", res.SessionToken)',
        (
            '\tservice.DefaultAuditRecorder.Record(r.Context(), req.Username, "login", "", map[string]any{\n'
            '\t\t"clientIP": clientIP,\n'
            "\t})\n"
            '\tw.Header().Set("X-IDIP-Session", res.SessionToken)'
        ),
    ),
]

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
sftp = c.open_sftp()

for rel, needle, repl in patches:
    remote = f"{ROOT}/{rel}"
    local = LOCAL / rel
    data = local.read_text(encoding="utf-8")
    if repl in data:
        print(f"{rel}: already patched (local)")
        text = data
    elif needle not in data:
        raise SystemExit(f"{rel}: anchor missing")
    else:
        text = data.replace(needle, repl, 1)
        print(f"{rel}: patched")
    with sftp.file(remote, "w") as f:
        f.write(text)

sftp.close()
c.close()
print("PATCH_LINUX_OK")
