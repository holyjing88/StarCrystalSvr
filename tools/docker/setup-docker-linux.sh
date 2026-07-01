#!/usr/bin/env bash
# Linux 宿主机：修复 Docker 拉取 Hub 时的 x509 证书问题，并可选配置 registry-mirrors
#
# 用法:
#   bash setup-docker-linux.sh              # 更新 CA + 配置镜像加速 + 重启 docker
#   bash setup-docker-linux.sh --ca-only    # 仅更新 CA 证书
#   bash setup-docker-linux.sh --mirror-only
#
# 环境变量:
#   DOCKER_REGISTRY_MIRRORS='https://docker.m.daocloud.io https://mirror.ccs.tencentyun.com'
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

mode="${1:-all}"
case "$mode" in
  all|--ca-only|--mirror-only) ;;
  -h|--help)
    echo "Usage: $0 [all|--ca-only|--mirror-only]"
    exit 0
    ;;
  *)
    echo "Usage: $0 [all|--ca-only|--mirror-only]" >&2
    exit 1
    ;;
esac

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请用 root 运行（或 sudo bash $0）" >&2
  exit 1
fi

install_ca() {
  echo "==> 更新系统 CA 证书"
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y ca-certificates curl
    update-ca-trust extract
  elif command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl
    update-ca-trust extract 2>/dev/null || update-ca-trust force-enable 2>/dev/null || true
  elif command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq
    apt-get install -y ca-certificates curl
    update-ca-certificates
  else
    echo "[warn] 未识别包管理器，请手动安装 ca-certificates" >&2
  fi
}

write_daemon_json() {
  local mirrors="${DOCKER_REGISTRY_MIRRORS:-https://docker.m.daocloud.io https://mirror.ccs.tencentyun.com}"
  local daemon_dir="/etc/docker"
  local daemon_file="$daemon_dir/daemon.json"
  mkdir -p "$daemon_dir"
  if [[ -f "$daemon_file" ]] && grep -q registry-mirrors "$daemon_file" 2>/dev/null; then
    echo "==> $daemon_file 已含 registry-mirrors，跳过写入"
    return 0
  fi
  echo "==> 写入 $daemon_file registry-mirrors"
  local m1 m2
  m1="${mirrors%% *}"
  m2="${mirrors#* }"
  if [[ "$m2" == "$mirrors" ]]; then m2=""; fi
  if [[ -n "$m2" ]]; then
    cat >"$daemon_file" <<EOF
{
  "registry-mirrors": [
    "$m1",
    "$m2"
  ]
}
EOF
  else
    cat >"$daemon_file" <<EOF
{
  "registry-mirrors": [
    "$m1"
  ]
}
EOF
  fi
  echo "registry-mirrors: $mirrors"
}

restart_docker() {
  echo "==> 重启 docker"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload 2>/dev/null || true
    systemctl restart docker
    systemctl is-active docker
  else
    service docker restart
  fi
}

verify_pull() {
  command -v docker >/dev/null 2>&1 || { echo "docker 未安装" >&2; return 1; }
  echo "==> 验证 docker pull redis:7-alpine"
  if docker pull redis:7-alpine; then
    echo "==> Docker Hub 拉取成功"
    return 0
  fi
  echo "==> 仍无法拉取。请使用离线方案: fetch-docker-base-images.sh load 或 docker_redis.sh load" >&2
  return 1
}

case "$mode" in
  all)
    install_ca
    write_daemon_json
    restart_docker
    verify_pull || true
    ;;
  --ca-only)
    install_ca
    restart_docker
    verify_pull || true
    ;;
  --mirror-only)
    write_daemon_json
    restart_docker
    verify_pull || true
    ;;
esac

echo ""
echo "setup-docker-linux: done"
