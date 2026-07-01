# 运维脚本验收（tools/scripts + release 启停）

- **发布运行目录**：`server-go/release/`（`starcrystalsvr` 二进制、`configs/`、`assets/`）
- **运维脚本目录**：`server-go/tools/scripts/`
- **release 根目录仅保留**：`startsvr.ps1` / `stopsvr.ps1`、`startsvr.sh` / `stopsvr.sh`

用例 ID：**SCR-***，清单见 `tools/scripts/test/script-catalog.json`。

## 运行

```powershell
cd server-go
.\tools\scripts\test-scripts.ps1
.\tools\scripts\test-scripts.ps1 -Live
```

```bash
cd server-go
bash tools/scripts/test-scripts.sh
SCRIPTS_TEST_LIVE=1 bash tools/scripts/test-scripts.sh
```

## 日常命令（在 release/ 下）

```powershell
cd server-go\release
..\tools\scripts\dbscripts\startdb.ps1
.\startsvr.ps1
.\stopsvr.ps1
```

```bash
cd server-go/release
bash ../tools/scripts/dbscripts/startdb.sh
./startsvr.sh
./stopsvr.sh
```

环境变量 `SCRIPTS_TEST_LIVE`、`SCRIPTS_TEST_FULL`、`SCRIPTS_TEST_REDIS_PORT` 等同前文档。

## Docker 开发脚本（均在 tools/scripts/）

```bash
cd server-go
bash tools/docker/docker_mysql.sh build|save|load|paths
bash tools/docker/docker_redis.sh build|save|load|paths
bash tools/docker/docker_release.sh build|save|load|paths
bash tools/docker/docker_mirror_save.sh
bash tools/docker/install-docker.sh
bash tools/docker/docker_svrdev.sh
bash tools/docker/docker_stopdb.sh
```

| 组件 | 镜像标签 | 导出 tar（主 / 离线） |
|------|----------|----------------------|
| MySQL | `starcrystal/mysql:8.4` | `tools/docker/mirror_save/mysql/` |
| Redis | `starcrystal/redis:7-alpine` | `tools/docker/mirror_save/redis/` |
| Release | `starcrystal/release:bundle` | `tools/docker/mirror_save/release/`（无 exe，log 空目录） |

MySQL 容器数据：`server-go/.docker-data/mysql/`。分发说明见 `doc/DOCKER_IMAGE_SHARE.md`。

模拟 Linux 全量验收（Git Bash / WSL / Linux）：

```bash
bash tools/scripts/simulate-linux-start-test.sh
```
