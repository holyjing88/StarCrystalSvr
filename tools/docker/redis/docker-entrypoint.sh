#!/bin/sh
# StarCrystal Redis：在官方 entrypoint 之前启动容器内备份 crond（与 docker_redis.sh stop 中 pkill 配对）
# 配置见镜像内 /etc/redis/redis.conf；数据与备份在宿主挂载的 /data
set -e
mkdir -p /data/backups
if command -v crond >/dev/null 2>&1; then
  crond -b -l 2
  echo "[starcrystal-redis] backup crond started (daily 03:00 -> /data/backups)"
fi
exec /usr/local/bin/docker-entrypoint.sh "$@"
