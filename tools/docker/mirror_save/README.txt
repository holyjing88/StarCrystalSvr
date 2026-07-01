Docker 镜像离线导出目录（docker_mysql/redis/release save 均写入此处）

  mysql/   starcrystal-mysql-8.4.tar
           mysql-8.4-base.tar          （fetch-docker-base-images.sh save）
  redis/   starcrystal-redis-7-alpine.tar
           redis-7-alpine-base.tar     （fetch-docker-base-images.sh save）
  release/ starcrystal-release-bundle.tar

目标机: 对应 docker_*.sh load 后 start；或 fetch-docker-base-images.sh load 后 build

一键 rebuild 并导出: bash tools/docker/docker_mirror_build.sh

大文件不入 Git（见 tools/docker/.gitignore）
