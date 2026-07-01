StarCrystal 运维脚本（server-go/tools/scripts/）
============================================
运行时发布包目录: server-go/release/（starcrystalsvr 与 configs、assets）

在 release/ 目录执行（推荐）:
  Windows: ..\tools\scripts\dbscripts\startdb.ps1
  Linux:   bash ../tools/scripts/dbscripts/startdb.sh

API 启停仅在 release/ 根目录:
  .\startsvr.ps1 / .\stopsvr.ps1  |  ./startsvr.sh / ./stopsvr.sh

常用（tools/scripts/）:
  dbscripts/startdb / stopdb    MySQL + Redis
  startallsvr / stopallsvr      MySQL + Redis + API（API 调用 release/startsvr）
  dbscripts/                    MySQL + Redis 分项启停、bootstrap、rebuild、备份
  build.ps1 / build.sh
  install-linux.sh
  Docker 开发见 ../docker/readme.txt
  simulate-linux-start-test.sh   Linux 模拟验收

离线包: ../offlinesofts/（tools 根目录，与 scripts 同级）
验收:   test-scripts.ps1 | test-scripts.sh（见 doc/SCRIPTS_TESTING.md）
