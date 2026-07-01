StarCrystal 运行目录（starcrystalsvr 发布包）

本目录仅保留 API 启停入口（与 starcrystalsvr 二进制同目录）：
  Windows: .\startsvr.ps1  |  .\stopsvr.ps1
  Linux:   ./startsvr.sh   |  ./stopsvr.sh

其它运维脚本在 server-go/tools/scripts/：
  启停库 dbscripts/startdb|stopdb、MySQL、Redis、build、install-linux；离线包见 tools/offlinesofts/

Linux 离线安装: bash ../tools/scripts/install-linux.sh
Docker 开发:   bash ../tools/docker/install-docker.sh
  离线镜像:   tools/docker/mirror_save/（release 镜像不含本目录 exe，log 仅空目录）
  release 容器: bash ../tools/docker/docker_release.sh build|save|load|start

配置: configs/starcrystal.json
日志: log/（运行时写入，不打进 release 镜像）

监听见 configs\starcrystal.json 的 apiListenHost / apiListenPort；useHttps=true 时仅接受 HTTPS（可配 tlsCertFile/tlsKeyFile 直连 TLS，或由反向代理传 X-Forwarded-Proto: https）
