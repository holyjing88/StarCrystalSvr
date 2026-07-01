#!/usr/bin/env bash
# 在 Linux 宿主机安装系统 Redis（yum + epel），安装前强制释放 yum 锁。
# 用法: bash tools/scripts/_install_native_redis.sh
set -euo pipefail
export LANG="${LANG:-C.UTF-8}"

sc_release_yum_lock() {
  echo "==> force release yum lock"
  systemctl stop packagekit 2>/dev/null || true
  for p in yum yumBackend dnf PackageKit packagekitd; do
    pkill -9 "$p" 2>/dev/null || true
  done
  sleep 1
  for p in yum yumBackend dnf PackageKit packagekitd; do
    pkill -9 "$p" 2>/dev/null || true
  done
  rm -f /var/run/yum.pid /var/run/dnf.pid /var/run/PackageKit.pid
  rm -f /var/lib/rpm/.rpm.lock
  find /var/cache/yum -name cachecookie -delete 2>/dev/null || true
  if pgrep -a 'yum|PackageKit|packagekit' >/dev/null 2>&1; then
    echo "[sc_release_yum_lock] WARNING: yum/PackageKit still running:" >&2
    pgrep -a 'yum|PackageKit|packagekit' >&2 || true
    return 1
  fi
  echo "==> yum lock cleared"
}

sc_release_yum_lock

echo "==> install epel + redis"
yum install -y epel-release
yum install -y redis

echo "==> configure redis (127.0.0.1, systemd)"
conf=/etc/redis.conf
sed -i 's/^bind .*/bind 127.0.0.1/' "$conf"
if grep -q '^supervised ' "$conf"; then
  sed -i 's/^supervised .*/supervised systemd/' "$conf"
else
  echo 'supervised systemd' >> "$conf"
fi

systemctl enable redis
systemctl restart redis
systemctl is-active redis
redis-cli ping
redis-server --version
ss -lntp | grep 6379 || true
echo "==> native redis installed on 127.0.0.1:6379"
