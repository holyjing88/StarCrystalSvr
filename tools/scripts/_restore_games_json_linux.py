#!/usr/bin/env python3
import json
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
cfg = f"{ROOT}/release/configs/games.json"
bak = f"{ROOT}/release/configs/games.json.bak.1780315408"

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

cmds = [
    f"cp -a {bak} {cfg}",
    f"""python3 - <<'PY'
import json
from pathlib import Path
p = Path("{cfg}")
cfg = json.loads(p.read_text(encoding="utf-8-sig"))
for g in cfg.get("list", []):
    if g.get("gameId") == "g001":
        g["channels"] = ["ChannelType_GooglePlay"]
        break
p.write_text(json.dumps(cfg, ensure_ascii=False, indent=2) + "\\n", encoding="utf-8")
print("g001 channels patched, games=", len(cfg.get("list", [])))
PY""",
]
for cmd in cmds:
    _, o, e = c.exec_command(cmd, timeout=60)
    out = o.read().decode()
    err = e.read().decode()
    code = o.channel.recv_exit_status()
    print(out or err)
    if code != 0:
        c.close()
        sys.exit(code)

# g002 tar metadata
_, o, _ = c.exec_command(
    f"""python3 - <<'PY'
import hashlib, json
from pathlib import Path
root = Path("{ROOT}")
tar = root / "release_h5/game2_v1.0.0.0.tar.gz"
cfg_path = root / "release/configs/games.json"
raw = tar.read_bytes()
sha = hashlib.sha256(raw).hexdigest()
size = len(raw)
cfg = json.loads(cfg_path.read_text(encoding="utf-8-sig"))
for g in cfg.get("list", []):
    if g.get("gameId") == "g002":
        g["packageBytes"] = size
        g["downloadSha256"] = sha
        break
cfg_path.write_text(json.dumps(cfg, ensure_ascii=False, indent=2) + "\\n", encoding="utf-8")
print("g002", size, sha[:16])
PY"""
)
print(o.read().decode())
c.close()
print("RESTORE_OK")
