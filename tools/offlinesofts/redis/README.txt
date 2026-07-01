两种方式二选一（或并存）：

1) 便携源码 tar.gz（无 Docker）
  redis-7.2.6.tar.gz
  下载: bash ../fetch-linux-offline-packages.sh
  离线安装脚本会本地编译并安装到 server-go/redis/linux/

2) Docker 镜像 tar（有 Docker 时，推荐目录 tools/docker/mirror_save/redis/）
  starcrystal-redis-7-alpine.tar
  生成: cd server-go && bash tools/docker/docker_redis.sh build save
  导入: bash tools/docker/docker_redis.sh load
