#!/usr/bin/env python3
import sys
from pathlib import Path

import paramiko

env: dict[str, str] = {}
for line in Path(__file__).resolve().parent.parent.joinpath("linux-host.local.env").read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if line and not line.startswith("#") and "=" in line:
        k, v = line.split("=", 1)
        env[k.strip()] = v.strip()

ROOT = env.get("LINUX_REPO", "/home/holyjing/starcrystalsvr")
GO = f"{ROOT}/.go-toolchain/go/bin/go"
HOST = env["LINUX_HOST"]


def run(c: paramiko.SSHClient, cmd: str, timeout: int = 300) -> int:
    print(">>>", cmd[:120], flush=True)
    _, stdout, stderr = c.exec_command(cmd, timeout=timeout, get_pty=True)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    code = stdout.channel.recv_exit_status()
    if out:
        print(out, end="")
    if err:
        print(err, file=sys.stderr, end="")
    return code


def main() -> None:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(
        HOST,
        username=env["LINUX_USER"],
        password=env["LINUX_PASSWORD"],
        timeout=30,
        allow_agent=False,
        look_for_keys=False,
    )

    pack = f"""
set -e
H5={ROOT}/release_h5
python3 - <<'PY'
import hashlib, json, os, subprocess, tarfile, tempfile
from pathlib import Path
root = Path("{ROOT}")
h5 = root / "release_h5"
cfg_path = root / "release/configs/games.json"
cfg = json.loads(cfg_path.read_text(encoding="utf-8-sig"))
changed = False
for g in cfg.get("list", []):
    if str(g.get("entryType", "")).lower() != "h5":
        continue
    url = (g.get("downloadUrl") or "").replace("\\\\", "/")
    if not url:
        continue
    tar_name = url.split("/")[-1]
    tar_path = h5 / tar_name
    entry = (g.get("entryUrl") or "").replace("\\\\", "/")
    parts = entry.split("/")
    try:
        i = parts.index("h5")
        game_dir = parts[i + 1]
    except (ValueError, IndexError):
        continue
    src = h5 / game_dir
    if not src.is_dir():
        print("skip no dir", game_dir)
        continue
    if not tar_path.is_file():
        ver = "1.0.0.0"
        if "?v=" in entry:
            ver = entry.split("?v=")[-1].split("&")[0]
        top = f"{{game_dir}}_v{{ver}}"
        stage = Path(tempfile.mkdtemp())
        dest = stage / top
        dest.mkdir(parents=True)
        for child in src.iterdir():
            if child.name.startswith("."):
                continue
            subprocess.run(["cp", "-a", str(child), str(dest / child.name)], check=True)
        with tarfile.open(tar_path, "w:gz") as tf:
            tf.add(dest, arcname=top)
        subprocess.run(["rm", "-rf", str(stage)], check=True)
        print("packed", tar_path)
        changed = True
    raw = tar_path.read_bytes()
    g["packageBytes"] = len(raw)
    g["downloadSha256"] = hashlib.sha256(raw).hexdigest()
    changed = True
before = len(cfg.get("list", []))
cfg["list"] = [
    g for g in cfg.get("list", [])
    if not str(g.get("gameId", "")).startswith("vitest-")
]
if len(cfg["list"]) != before:
    changed = True
    print("removed vitest games from config")
if changed:
    cfg_path.write_text(json.dumps(cfg, ensure_ascii=False, indent=2) + "\\n", encoding="utf-8")
    print("games.json metadata synced")
PY
rm -rf "$H5/vitest-game1" "$H5"/.upload-vitest-game1-* 2>/dev/null || true
"""
    if run(c, pack) != 0:
        sys.exit(1)

    tests = [
        f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH GOPROXY=https://goproxy.cn,direct && bash tools/scripts/build.sh",
        f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH && {GO} test ./internal/service/... -count=1 -short",
        f"cd {ROOT} && export PATH={ROOT}/.go-toolchain/go/bin:$PATH && {GO} test ./internal/api/... -run IdipAuth -count=1 -timeout 120s",
        f"""set -e
cd {ROOT}/release
pkill -9 -x starcrystalsvr 2>/dev/null || true
sleep 1
rm -f starcrystalsvr.pid
su - holyjing -c 'cd {ROOT}/release && ./startsvr.sh'
sleep 3
curl -fsS http://127.0.0.1:8080/healthz""",
        f"bash {ROOT}/tools/idip-webclient/scripts_encrypt/_test_idip_login.sh",
    ]
    for cmd in tests:
        if run(c, cmd, timeout=600) != 0:
            c.close()
            sys.exit(1)

    c.close()
    print("LINUX_VERIFY_OK", flush=True)


if __name__ == "__main__":
    main()
