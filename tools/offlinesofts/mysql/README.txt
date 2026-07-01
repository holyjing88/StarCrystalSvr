两种方式二选一（或并存）：

1) 便携 tar.xz（无 Docker）
  mysql-8.4.8-linux-glibc2.28-x86_64.tar.xz
  下载: bash ../fetch-linux-offline-packages.sh

2) Docker 镜像 tar（有 Docker 时，推荐目录 tools/docker/mirror_save/mysql/）
  starcrystal-mysql-8.4.tar
  生成: cd server-go && bash tools/docker/docker_mysql.sh build save
  导入: bash tools/docker/docker_mysql.sh load
