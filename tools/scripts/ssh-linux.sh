#!/usr/bin/env bash
# 在 tools/linux-host.local.env 配置的 Linux 宿主机上执行命令
#
# 用法:
#   bash tools/scripts/ssh-linux.sh 'uname -a'
#   bash tools/scripts/ssh-linux.sh 'cd /home/holyjing/starcrystalsvr/tools/docker && ./docker_mirror_build.sh'
#
# 依赖: openssh-client；推荐 sshpass（yum install sshpass / apt install sshpass）
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
ENV_FILE="$HERE/../linux-host.local.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE" >&2
  echo "请复制: cp tools/linux-host.env.example tools/linux-host.local.env" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${LINUX_HOST:?LINUX_HOST 未设置}"
: "${LINUX_USER:?LINUX_USER 未设置}"
: "${LINUX_PASSWORD:?LINUX_PASSWORD 未设置}"

remote_cmd="${1:-}"
if [[ -z "$remote_cmd" ]]; then
  echo "Usage: $0 'remote command'" >&2
  exit 1
fi

ssh_opts=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)

if command -v sshpass >/dev/null 2>&1; then
  SSHPASS="$LINUX_PASSWORD" sshpass -e ssh "${ssh_opts[@]}" "${LINUX_USER}@${LINUX_HOST}" "$remote_cmd"
elif command -v python3 >/dev/null 2>&1 || command -v python >/dev/null 2>&1; then
  py=python3
  command -v python3 >/dev/null 2>&1 || py=python
  "$py" - "$LINUX_HOST" "$LINUX_USER" "$LINUX_PASSWORD" "$remote_cmd" <<'PY'
import paramiko, sys
host, user, pwd, cmd = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(host, username=user, password=pwd, timeout=30, allow_agent=False, look_for_keys=False)
stdin, stdout, stderr = c.exec_command(cmd, get_pty=True)
out = stdout.read().decode("utf-8", "replace")
err = stderr.read().decode("utf-8", "replace")
code = stdout.channel.recv_exit_status()
if out:
    sys.stdout.write(out)
if err:
    sys.stderr.write(err)
c.close()
sys.exit(code)
PY
else
  echo "需要 sshpass 或 python+paramiko 才能非交互 SSH" >&2
  exit 1
fi
