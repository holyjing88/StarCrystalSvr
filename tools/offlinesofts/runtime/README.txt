Ubuntu/Debian 可将 MySQL 运行时依赖的 .deb 放在 deb/ 子目录。

推荐包：libaio1 libncurses6 libtinfo6 numactl

编译 Redis 还需 gcc、make（或设置 FETCH_BUILD_DEB=1 由 fetch 脚本尝试下载）。

install-linux-offline.sh 会尝试 sudo dpkg -i deb/*.deb
