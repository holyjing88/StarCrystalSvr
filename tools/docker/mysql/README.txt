Dockerfile 用于 tools/docker/docker_mysql.sh build

构建后镜像标签: starcrystal/mysql:8.4

镜像内含（配置打入镜像，勿挂载覆盖）:
  - /etc/mysql/conf.d/starcrystal.cnf
  - /docker-entrypoint-initdb.d/01-starcrystal-auth.sql（仅首次初始化数据目录时执行）
  - /opt/starcrystal/mysql-scripts/mysql-backup.sh（build 时从 tools/scripts/mysql 同步）
  - 容器内 cronie，默认每天 03:00 备份

宿主磁盘目录（start 时挂载，可用环境变量覆盖）:
  DOCKER_MYSQL_DATA         默认 <repo>/.docker-data/mysql/       -> /var/lib/mysql
  DOCKER_MYSQL_BACKUP_DIR   默认 <repo>/.docker-data/mysql-backups/ -> /backups

生产建议将上述目录指到独立磁盘，例如:
  export DOCKER_DATA_DIR=/var/lib/starcrystal
  # 数据: /var/lib/starcrystal/mysql/
  # 备份: /var/lib/starcrystal/mysql-backups/

启停:
  bash tools/docker/docker_mysql.sh start   # MySQL + 容器内备份 crond
  bash tools/docker/docker_mysql.sh stop    # 先停 crond 再停 MySQL

手动备份:
  docker exec starcrystal-dev-mysql /opt/starcrystal/mysql-scripts/mysql-backup.sh

若容器在增加 /backups 卷之前已创建，需先 stop 并 rm 容器后重新 start。

导出 tar:
  bash tools/docker/docker_mysql.sh build   # 构建后自动写入 mirror_save
  bash tools/docker/docker_mysql.sh save    # 仅重新导出
  -> tools/docker/mirror_save/mysql/starcrystal-mysql-8.4.tar

导入: bash tools/docker/docker_mysql.sh load

分发给其他服务器: server-go/doc/DOCKER_IMAGE_SHARE.md
