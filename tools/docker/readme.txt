StarCrystal Docker 开发（server-go/tools/docker/）
================================================
与 tools/scripts/、tools/offlinesofts/ 同级。

MySQL / Redis 配置已打入各自镜像；数据与备份目录默认在宿主:
  .docker-data/mysql/          MySQL 数据
  .docker-data/mysql-backups/  MySQL 备份
  .docker-data/redis/          Redis 数据 + backups/

可用 DOCKER_DATA_DIR / DOCKER_MYSQL_DATA / DOCKER_MYSQL_BACKUP_DIR / DOCKER_REDIS_DATA 指到生产磁盘。

x509 / 无法拉 Docker Hub:
  bash tools/docker/setup-docker-linux.sh          # root：CA + 镜像加速
  bash tools/docker/fetch-docker-base-images.sh    # 联网机 save / 离线机 load 基础镜像
  bash tools/docker/accept-docker.sh             # 验收

常用:
  bash tools/docker/install-docker.sh
  bash tools/docker/docker_startdb.sh
  bash tools/docker/docker_stopdb.sh
  bash tools/docker/docker_svrdev.sh
  bash tools/docker/docker_mysql.sh build|save|load|start
  bash tools/docker/docker_redis.sh build|save|load|start
  bash tools/docker/docker_release.sh build|save|load|start

离线镜像:
  bash tools/docker/docker_mirror_build.sh   # 强制 rebuild 三台并导出 tar
  bash tools/docker/docker_mirror_save.sh    # 已有镜像则仅 save
  -> tools/docker/mirror_save/mysql|redis|release/*.tar

release 镜像说明:
  - 含 configs、assets、startsvr.sh；不含 starcrystalsvr.exe
  - log/ 仅空目录；运行时 -v 挂载本机 starcrystalsvr 与日志卷

镜像分发: server-go/doc/DOCKER_IMAGE_SHARE.md
Dockerfile: mysql/、redis/、release/
