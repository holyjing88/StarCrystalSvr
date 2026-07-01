server-go/tools/
==================
  linux-host.env.example   Linux 宿主机 SSH 模板（复制为 linux-host.local.env，勿提交）
  linux-host.local.env     本地账号（已 gitignore，勿提交 SVN）
  scripts/       运维脚本（dbscripts/startdb、install-linux、build、ssh-linux.sh 等）
  docker/        Docker MySQL/Redis/Release；导出目录 mirror_save/（见 docker/readme.txt）
  offlinesofts/  Linux 离线安装包与 Docker 镜像 tar（见 offlinesofts/README.md）

Linux 远程执行:
  bash tools/scripts/ssh-linux.sh 'cd /home/holyjing/starcrystalsvr/tools/docker && bash docker_mirror_build.sh'
