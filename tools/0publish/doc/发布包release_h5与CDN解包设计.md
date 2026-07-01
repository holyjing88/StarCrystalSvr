# 发布包 `release_h5` 与 CDN `/h5` 解包设计

> **状态**：已实现（`pack-publish.sh` / `pack-publish.ps1` / `unpack.sh` / `pack-publish-verify.sh`）。  
> **关联**：[`doc/H5小游戏联调CDN-Nginx部署.md`](../../../doc/H5小游戏联调CDN-Nginx部署.md)（客户端副本路径可能为 `StarCrystal2022/doc/...`）、`internal/service/h5_paths.go`、`publish.rsync.cdnH5Dir`。

---

## 1. 目标

外网/联调机执行 **`unpack.sh`** 部署 API 发布包时，除原有 `release` / `dbscripts` / `idip-webclient` 外：

1. **打包**：`pack-publish.sh` 将仓库根 **`release_h5/`** 打入发布子目录 **`release_h5.tar.gz`**（与三个业务包并列）。
2. **解包**：在部署上述三目录之后，对 **CDN 静态根下的 `h5/`** 做「备份 → 清空 → 用发布包内 `release_h5` 全量覆盖」。

使 **IDIP 日常上传写盘**（`release_h5` + API `rsync`）与 **整包外网发布**（`unpack.sh` 直写 wwwroot）两条链路最终落到 **同一 CDN 路径**，客户端 `gameBaseCDNUrl` + `games.json` 的 `h5/...` URL 不变。

---

## 2. 路径约定

| 角色 | 路径 | 说明 |
|------|------|------|
| 仓库 H5 写盘（IDIP / API） | `{repoRoot}/release_h5/` | `publish.h5AssetsDir` 默认；整包 `.tar.gz` 与游戏目录同级 |
| H5 备份（IDIP 删包前） | `{repoRoot}/release_h5_backup/` | 运营删包/覆盖前按游戏备份 zip，与发布解包无关 |
| API 部署根（外网 unpack） | `/app/publish/` | 默认 `UNPACK_DEPLOY_ROOT`；下有 `release/`、`dbscripts/`、`idip-webclient/` |
| CDN 站点根 | `/wwwroot/minigame.starlaneinfinite.com/` | Nginx `root`；生产/联调同约定 |
| **CDN H5 对外目录** | `/wwwroot/minigame.starlaneinfinite.com/h5/` | URL `/h5/game2/index.html` 映射到此 |
| CDN H5 发布前备份目录 | `/wwwroot/minigame.starlaneinfinite.com/_h5-predeploy-backup/` | **仅 `unpack.sh` 写入**；与 IDIP `release_h5_backup` 分离 |

联调示例：

```text
/home/holyjing/starcrystalsvr/release_h5/     ← 开发机 IDIP 写盘
/wwwroot/minigame.starlaneinfinite.com/h5/    ← Nginx :9090 静态（unpack 全量覆盖目标）
```

---

## 3. `pack-publish.sh` 变更（已实现）

### 3.1 新增源与产物

```text
源目录:  $REPO_ROOT/release_h5
产物:    $OUTPUT_DIR/release_h5.tar.gz
staging: $STAGING_ROOT/release_h5/   # tar 包内顶层目录名固定为 release_h5
```

### 3.2 打包规则

- 与 `dbscripts` 类似：`copy_tree` + `write_tar release_h5`。
- **排除**（建议，避免打进巨大运行时垃圾）：
  - `release_h5_backup/`（若误放在 `release_h5` 下）
  - `.upload-*` 临时目录（IDIP 上传中间态）
  - 可选：体积极大的本地测试包（由 `--exclude` 与 manifest 说明）
- **包含**：各 `game*/index.html`、同目录 `*_v*.tar.gz` 整包、与当前 `rsync` 到 CDN 一致的文件集。
- `release_h5` **不存在或为空**时：`WARN` 并仍生成空包，或 **失败退出**（实现时二选一，建议 **WARN + 空目录 tar**，外网 unpack 可跳过 CDN 步）。

### 3.3 发布子目录内容（Linux）

```text
tools/0publish/yyyyMMdd-HHmmss/
  release.tar.gz
  dbscripts.tar.gz
  idip-webclient.tar.gz
  release_h5.tar.gz          ← 新增
  pack-manifest.txt
  unpack.sh
```

`pack-manifest.txt` 与 `readme.txt` 同步增加第四项说明。

### 3.4 Windows `pack-publish.ps1`

与 Linux 对齐：子目录内增加 **`release_h5.zip`**（或 `.tar.gz`，以实现为准），总包 zip 一并纳入。

---

## 4. `unpack.sh` 变更（已实现）

### 4.1 总体流程

在现有 **[1/3]～[3/3]**（备份并替换 `/app/publish` 下三目录）**成功之后**，增加 **CDN H5 段**（可编号 `[4/4]`）：

```text
[4/4] CDN H5（可选，见 §4.5）
  (a) 将 CDN_H5_DIR 整目录打 tar.gz 备份到 CDN_H5_BACKUP_ROOT
  (b) 保留该目录下最多 10 份备份，删除更旧的
  (c) 清空 CDN_H5_DIR 下全部内容（不删除 h5 目录本身）
  (d) 解压当前发布子目录内 release_h5.tar.gz 到临时 staging
  (e) 将 staging/release_h5/ 内所有条目同步到 CDN_H5_DIR/（等价 rsync -a）
```

**注意**：解压目标是 **`.../h5/` 目录内部**，不是把 `release_h5` 文件夹再套一层（即最终为 `.../h5/game2/index.html`，而非 `.../h5/release_h5/game2/...`）。

### 4.2 CDN H5 备份（最多 10 份）

| 项 | 约定 |
|----|------|
| 备份根目录 | `CDN_H5_BACKUP_ROOT` 默认 `/wwwroot/minigame.starlaneinfinite.com/_h5-predeploy-backup` |
| 单次备份文件 | `{CDN_H5_BACKUP_ROOT}/h5-{publishSubdir}-{yyyyMMdd-HHmmss}.tar.gz` |
| 备份方式 | `tar -czf ... -C "$(dirname CDN_H5_DIR)" "$(basename CDN_H5_DIR)"`（打包整个 `h5` 目录快照） |
| 保留份数 | **10**（常量 `CDN_H5_BACKUP_MAX=10`，与 `games.json.bak` / `h5_backup` 运营策略一致） |
|  prune | 按文件名或 mtime 排序，删除超出 10 份的最旧文件 |

与 `/app/publish/_predeploy-backup/`（备份 release 等三目录）**独立**，互不影响。

### 4.3 清空与覆盖

```bash
# 伪代码
find "$CDN_H5_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
# 或: rsync --delete from empty + extract；实现须保证 h5 目录仍存在且权限正确
tar -xzf "$PUBLISH_SUBDIR/release_h5.tar.gz" -C "$STAGING"
rsync -a --delete "$STAGING/release_h5/" "$CDN_H5_DIR/"
```

- 清空范围：**仅** `CDN_H5_DIR` 下一层（`game1/`、`game2/`、`*.tar.gz` 等），不删除 `minigame.starlaneinfinite.com` 下其它未来目录。
- 覆盖后建议：`chown`/`chmod` 与 Nginx 运行用户一致（联调常见 `nginx:nginx` 或 `holyjing` + 全局读）；SELinux 环境可文档引用 `chcon -R -t httpd_sys_content_t`。

### 4.4 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `UNPACK_CDN_H5_DIR` | `/wwwroot/minigame.starlaneinfinite.com/h5` | 覆盖目标 |
| `UNPACK_CDN_H5_BACKUP_ROOT` | `/wwwroot/minigame.starlaneinfinite.com/_h5-predeploy-backup` | 备份存放 |
| `UNPACK_CDN_H5_BACKUP_MAX` | `10` | 最多保留备份数 |
| `UNPACK_SKIP_CDN_H5` | 未设 | 设为 `1` 时跳过 [4/4]（仅更新 API 包、不动 CDN） |

`--dry-run` 时：打印将备份/清空/解压的路径与 tar 名，不执行 `rm`/`tar`/`rsync`。

### 4.5 可选与失败策略

| 场景 | 行为（建议） |
|------|----------------|
| 发布子目录 **无** `release_h5.tar.gz` | 打印 `WARN` 并 **跳过** [4/4]（兼容旧包） |
| `CDN_H5_DIR` 不存在 | `mkdir -p` 后继续 |
| `CDN_H5_DIR` 不可写 | **失败退出**，不部分清空（先完成备份再清空） |
| 解压后 staging 无 `release_h5/` 顶层 | **失败退出**，并提示检查打包脚本 |

### 4.6 与 API `rsync` 的关系

| 时机 | 机制 |
|------|------|
| 日常 IDIP 上传/删包 | API 写 `release_h5` → `publish_cdn_sync.go` **增量 rsync** 到 `cdnH5Dir` |
| 外网整包 `unpack.sh` | **全量**备份 + 清空 + 用 `release_h5.tar.gz` 覆盖 `cdnH5Dir` |

二者互补：联调/生产日常走 IDIP；**割接/新环境/灾难恢复**走发布包 unpack，无需单独再跑 `rsync release_h5/`。

---

## 5. 验收

### 5.1 打包

```bash
cd /home/holyjing/starcrystalsvr
bash tools/0publish/scripts/pack-publish.sh --skip-build
ls -lh tools/0publish/$(ls -1t tools/0publish | head -1)/release_h5.tar.gz
tar -tzf .../release_h5.tar.gz | head
# 应见 release_h5/game2/index.html 等
```

### 5.2 解包（联调）

```bash
cd /app/publish/yyyyMMdd-HHmmss
UNPACK_DEPLOY_ROOT=/app/publish bash unpack.sh --dry-run
bash unpack.sh
ls /wwwroot/minigame.starlaneinfinite.com/_h5-predeploy-backup/   # 新增备份 ≤10
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9090/h5/game2/index.html
# 期望 200
```

### 5.3 回滚

从 `_h5-predeploy-backup/h5-*.tar.gz` 任选一份：

```bash
CDN=/wwwroot/minigame.starlaneinfinite.com
rm -rf "$CDN/h5"/*
tar -xzf "$CDN/_h5-predeploy-backup/h5-....tar.gz" -C "$CDN"
# 备份包内顶层为 h5/ 时调整 -C 与 strip 路径
```

（实现时 `unpack.sh` 备份格式固定后，可另附 `rollback-cdn-h5.sh` 或文档化一条 tar 命令。）

---

## 6. 实现清单

- [x] `pack-publish.sh`：`copy_release_h5`、`release_h5.tar.gz`、`pack-manifest.txt`
- [x] `pack-publish.ps1` / `pack-publish-verify.sh`：第四包校验
- [x] `unpack.sh`：[4/4] CDN 备份/清空/解压/保留 10 份
- [x] `tools/0publish/scripts/readme.txt`：四包说明与环境变量
- [ ] （可选）`doc/H5小游戏联调CDN-Nginx部署.md` §5 增加「外网 unpack 全量覆盖」一句

---

## 7. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-06-05 | 初稿：`release_h5` 入发布包；`unpack.sh` CDN `h5` 备份 10 份 + 全量覆盖 |
