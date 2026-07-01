release 镜像：打包 server-go/release 的 configs、assets、startsvr.sh（不含 starcrystalsvr.exe，log/ 仅空目录）。

构建上下文由 docker_release.sh 生成到 ../.build-release/

  bash tools/docker/docker_release.sh build
  bash tools/docker/docker_release.sh save   # -> tools/docker/mirror_save/release/

目标机需自备 Linux 二进制并挂载:
  -v /path/to/starcrystalsvr:/app/starcrystalsvr:ro
