Linux 便携 Redis（本目录）
----------------------------
二进制 redis-server / redis-cli 由源码编译生成，体积较大，已加入 .gitignore，不随仓库提交。

在 Linux 或 WSL 中安装（需 gcc、make、curl）：
  cd <server-go 根目录>
  bash tools/scripts/dbscripts/redis/install-redis-linux.sh

在 Windows 上若已安装 WSL：
  .\tools\scripts\dbscripts\redis\install-redis-linux.ps1

安装完成后，tools/scripts/dbscripts/redis/redis-common.sh 会优先使用 <repo>/redis/linux/ 下的 redis-server / redis-cli。
