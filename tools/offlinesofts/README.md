# Linux 离线安装包（offlinesofts）

路径：**`server-go/tools/offlinesofts/`**（与 `tools/scripts/` 同级）。

大文件默认**不入 Git**（见本目录 `.gitignore`）。在联网机器上先执行 `fetch`，打包整个 `offlinesofts/` 拷贝到离线机后执行 `install`。

## 目录结构

| 子目录 | 放入内容 |
|--------|----------|
| `mysql/` | `mysql-8.4.8-linux-glibc2.28-x86_64.tar.xz`、可选 `starcrystal-mysql-8.4.tar`（Docker） |
| `redis/` | `redis-7.2.6.tar.gz`、可选 `starcrystal-redis-7-alpine.tar`（Docker） |
| `go/` | `go1.24.0.linux-amd64.tar.gz`（可选） |
| `go-modules/` | `gomod-cache.tar.gz`（`go mod download` 生成，可选） |
| `runtime/deb/` | `libaio1` 等 `.deb`（Ubuntu/Debian，可选） |

版本与 URL 见 `manifest.json`。

## 1. 联网机：下载离线包

```bash
cd server-go
bash tools/offlinesofts/fetch-linux-offline-packages.sh
```

可选环境变量：

- `SKIP_GO=1` — 不下载 Go（仅运行已编译好的 `starcrystalsvr`）
- `SKIP_GOMOD=1` — 不打包模块缓存
- `SKIP_RUNTIME_DEB=1` — 不 `apt-get download` deb
- `FETCH_BUILD_DEB=1` — 额外下载 gcc/make 相关 deb（便于完全离线编译 Redis）

打包示例：

```bash
bash tools/offlinesofts/package-offlinesofts-bundle.sh
# 或
tar -czf starcrystal-offlinesofts-linux-amd64.tar.gz -C tools offlinesofts
```

## 2. 离线机：安装

```bash
cd server-go
tar -xzf /path/to/starcrystal-offlinesofts-linux-amd64.tar.gz -C tools
bash tools/offlinesofts/install-linux-offline.sh
```

安装结果：

- MySQL → `<repo>/../../mysql-portable-linux/mysql-8.4.8-linux-glibc2.28-x86_64`
- Redis → `server-go/redis/linux/redis-server`、`redis-cli`
- Go → `server-go/.go-toolchain/go`（可用 `STARCRYSTAL_GO_ROOT` 覆盖）

## 3. 一键安装（推荐）

离线包就位后，在 `release/` 目录执行：

```bash
cd server-go/release
bash ../tools/scripts/install-linux.sh
```

自动完成：安装 offlinesofts → 编译 `starcrystalsvr` → 启动 MySQL → 建库/账号 → 导入 schema → 启动 Redis 与 API。

联网机若尚未下载离线包：

```bash
ONLINE_FETCH=1 bash ../tools/scripts/install-linux.sh
```

仅安装不启动：`SKIP_START=1 bash ../tools/scripts/install-linux.sh`

## 4. 分步启服（可选）

```bash
cd server-go/release
bash ../tools/scripts/dbscripts/startdb.sh
bash ../tools/scripts/dbscripts/mysql/bootstrap-mysql-auth.sh
bash ../tools/scripts/dbscripts/mysql/rebuild-auth-mysql.sh
./startsvr.sh
```

## 与 Windows 便携环境对齐

| 组件 | Windows | Linux（本目录） |
|------|---------|----------------|
| MySQL | `mysql-portable/mysql-8.4.8-winx64` | `mysql-portable-linux/...` |
| Redis | `server-go/redis/*.exe` 或 WSL 构建 | `server-go/redis/linux/` |
| 启停库 | `dbscripts/startdb.ps1` / `stopdb.ps1` | `dbscripts/startdb.sh` / `stopdb.sh` |
| 启停 API | `startsvr.ps1` / `stopsvr.ps1` | `startsvr.sh` / `stopsvr.sh` |

## 架构说明

当前清单针对 **linux x86_64 (glibc 2.28+)**。ARM64 需更换 MySQL/Go 对应 tarball 并修改 `manifest.json` 中的文件名。
