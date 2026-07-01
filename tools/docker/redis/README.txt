Dockerfile 用于 tools/docker/docker_redis.sh build

构建后镜像标签: starcrystal/redis:7-alpine

镜像内含（配置打入镜像，勿挂载覆盖）:
  - /etc/redis/redis.conf
  - /opt/starcrystal/redis-scripts/redis-backup.sh（build 时从 tools/scripts/dbscripts/redis 同步）
  - 容器内 dcron，默认每天 03:00 备份到 /data/backups

宿主磁盘目录（start 时挂载，可用环境变量覆盖）:
  DOCKER_REDIS_DATA  默认 <repo>/.docker-data/redis/
    - dump.rdb / appendonly.aof（数据）
    - backups/redis-dump-*.rdb（备份）

生产建议:
  export DOCKER_REDIS_DATA=/var/lib/starcrystal/redis

启停:
  bash tools/docker/docker_redis.sh start   # Redis + 容器内备份 crond
  bash tools/docker/docker_redis.sh stop    # 先停 crond 再停 Redis

手动备份:
  docker exec starcrystal-dev-redis /opt/starcrystal/redis-scripts/redis-backup.sh

导出 tar:
  bash tools/docker/docker_redis.sh build   # 构建后自动写入 mirror_save
  bash tools/docker/docker_redis.sh save    # 仅重新导出
  -> tools/docker/mirror_save/redis/starcrystal-redis-7-alpine.tar

导入: bash tools/docker/docker_redis.sh load

分发给其他服务器: server-go/doc/DOCKER_IMAGE_SHARE.md
