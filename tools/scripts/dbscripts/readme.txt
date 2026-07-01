StarCrystal 数据库运维脚本（tools/scripts/dbscripts/）
=====================================================

本目录为**自包含**运维包：脚本、SQL、配置、运行时数据均在此目录内，
不依赖 repo/sql、release/、tools/starcrystal-config 等外部路径。

目录结构
--------
  dbscripts-config.sh / .ps1   本地配置入口（source / dot-source）
  local.env.example            环境变量模板（复制为 local.env）
  config/starcrystal.json.example
  sql/                         schema 与 Redis 键文档
  data/                        MySQL/Redis 默认数据与备份（运行时生成）
  mysql/  redis/               各组件脚本

快速开始
--------
  1. 使用已提供的 local.env / config/starcrystal.json（可按环境改密码与端口）
  2. Windows 便携 MySQL：在 local.env 中设置 MYSQL_PORTABLE_BASE

  Windows:
    .\tools\scripts\dbscripts\startdb.ps1
    .\tools\scripts\dbscripts\rebuilddb.ps1

  Linux:
    bash tools/scripts/dbscripts/startdb.sh
    bash tools/scripts/dbscripts/rebuilddb.sh

常用脚本
--------
  startdb / stopdb              MySQL + Redis 启停
  rebuilddb                     MySQL auth schema + Redis sr:* 重建

  mysql/mysql-start | mysql-stop
  mysql/bootstrap-mysql-auth.sh
  mysql/rebuild-auth-mysql

  redis/redis-start | redis-stop
  redis/rebuild-redis
  redis/install-redis-linux.sh  → 编译到 redis/linux/

Schema: sql/starcrystal_auth_mysql.sql, sql/starcrystal_redis_keys.md

独立发布包仍输出到 tools/0publish/dbscripts/（由 pack-publish 生成）。
