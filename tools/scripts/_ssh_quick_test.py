#!/usr/bin/env python3
import paramiko
import sys

ENV = r"Y:\holyjing\starcrystalsvr\tools\linux-host.local.env"
env = {}
for line in open(ENV, encoding="utf-8"):
    line = line.split("#", 1)[0].strip()
    if not line or "=" not in line:
        continue
    k, v = line.split("=", 1)
    env[k.strip()] = v.strip().strip('"').strip("'")

repo = env.get("LINUX_REPO", "/home/holyjing/starcrystalsvr")
publish = "/app/publish"

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(
    env["LINUX_HOST"],
    username=env["LINUX_USER"],
    password=env["LINUX_PASSWORD"],
    timeout=20,
    allow_agent=False,
    look_for_keys=False,
)

REMOTE_ISOLATED = f"""
set -euo pipefail
REPO='{repo}'
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
PR="$TMP/app/publish"
B="$PR/accept001"
mkdir -p "$PR/release" "$PR/dbscripts" "$PR/idip-webclient"
echo old >"$PR/release/marker.txt"
ST="$TMP/st"
mkdir -p "$ST/release" "$ST/dbscripts" "$ST/idip-webclient"
echo nr >"$ST/release/app.txt"
echo nd >"$ST/dbscripts/app.txt"
echo ni >"$ST/idip-webclient/app.txt"
mkdir -p "$B"
tar -czf "$B/release.tar.gz" -C "$ST" release
tar -czf "$B/dbscripts.tar.gz" -C "$ST" dbscripts
tar -czf "$B/idip-webclient.tar.gz" -C "$ST" idip-webclient
cp "$REPO/tools/0publish/scripts/unpack.sh" "$B/"
chmod +x "$B/unpack.sh"
cd "$B"
UNPACK_DEPLOY_ROOT="$PR" bash ./unpack.sh
test -f "$PR/release/app.txt"
test ! -f "$PR/release/marker.txt"
echo ISOLATED_OK
"""

EXTERNAL = f"""
set -e
P='{publish}'
if [ ! -d "$P" ]; then echo SKIP_NO_DIR; exit 0; fi
tag=$(ls -1t "$P" 2>/dev/null | grep -E '^[0-9]{{8}}-[0-9]{{6}}$' | head -1 || true)
if [ -z "$tag" ]; then echo SKIP_NO_TAG; exit 0; fi
cd "$P/$tag"
sed -i 's/\\r$//' unpack.sh
chmod +x unpack.sh
bash unpack.sh --dry-run
echo EXTERNAL_OK "$P/$tag"
"""


def go(name: str, cmd: str, timeout: int = 90) -> bool:
    print(f"--- {name} ---")
    try:
        _, stdout, stderr = c.exec_command(cmd, get_pty=True, timeout=timeout)
        out = stdout.read().decode("utf-8", "replace")
        err = stderr.read().decode("utf-8", "replace")
        code = stdout.channel.recv_exit_status()
    except Exception as e:
        print(f"TIMEOUT/ERR: {e}\n")
        return False
    if out.strip():
        print(out.rstrip())
    if err.strip():
        print(err.rstrip(), file=sys.stderr)
    ok = code == 0
    print(f"exit={code} {'OK' if ok else 'FAIL'}\n")
    return ok


print(f"==> {env['LINUX_USER']}@{env['LINUX_HOST']}")
results = []
results.append(
    go(
        "unpack-lf-syntax",
        f"bash -n {repo}/tools/0publish/scripts/unpack.sh; "
        f"wc -c < {repo}/tools/0publish/scripts/unpack.sh | tr -d ' '; "
        f"grep -U $'\\r' {repo}/tools/0publish/scripts/unpack.sh >/dev/null && exit 1 || echo NO_CR; "
        f"head -1 {repo}/tools/0publish/scripts/unpack.sh",
        30,
    )
)
results.append(go("isolated-unpack", REMOTE_ISOLATED, 90))
results.append(go("external-dry-run", EXTERNAL, 60))

USER_BUNDLE = f"""
set -e
REPO='{repo}'
B=$REPO/tools/0publish/20260531-073228
if [ ! -d "$B" ]; then echo SKIP_NO_BUNDLE; exit 0; fi
cr=$(tr -cd '\\r' < "$B/unpack.sh" | wc -c)
echo "CR_before=$cr"
cp "$REPO/tools/0publish/scripts/unpack.sh" "$B/unpack.sh"
chmod +x "$B/unpack.sh"
cr2=$(tr -cd '\\r' < "$B/unpack.sh" | wc -c)
echo "CR_after=$cr2"
bash -n "$B/unpack.sh"
cd "$B"
UNPACK_DEPLOY_ROOT=/app/publish bash ./unpack.sh --dry-run
echo USER_BUNDLE_OK
"""
results.append(go("user-bundle-20260531-073228", USER_BUNDLE, 60))

c.close()
if all(results):
    print("REMOTE: all checks passed")
    sys.exit(0)
print("REMOTE: some checks failed")
sys.exit(1)
