# IDIP API（网页客户端副本）

> **权威文档**（与 server-go 同步）：[`../../../doc/IDIP_API.md`](../../../doc/IDIP_API.md)  
> **设计**：[`H5游戏发布与运营登录设计.md`](./H5游戏发布与运营登录设计.md)

## 鉴权约定

| 场景 | 方式 |
|------|------|
| **运营台浏览器** | `POST /idip/v1/auth/login` → `X-IDIP-Session` + 30s 心跳 |
| **Vitest / 回归脚本** | 环境变量 `VITE_IDIP_KEY` 或 `X-IDIP-Key`，**无需** login |

## 快速索引（v1.3 已实现）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/idip/v1/auth/login` | 运营登录 |
| POST | `/idip/v1/auth/logout` | 注销 |
| POST | `/idip/v1/auth/heartbeat` | 心跳 |
| GET | `/idip/v1/games/list` | 游戏列表 + configVersion |
| POST | `/idip/v1/games/upsert` | 单条更新 |
| POST | `/idip/v1/games/batch-upsert` | 批量 ≤50 |
| POST | `/idip/v1/games/delete` | 删除 offline |
| POST | `/idip/v1/games/h5/upload` | H5 zip 上传 |
| GET | `/idip/v1/audit/logs` | 操作审计（内存） |
| POST | `/idip/v1/gold/set-user` | 改金币 |
| GET/POST | `/idip/v1/welfare/*` | 福利 |
| GET/POST | `/idip/v1/tasks/*` | 任务 |

规划未实现：`POST /idip/v1/publish/rsync-retry`、MySQL 审计落库。

## 运营密码加密

- Linux：`tools/idip-webclient/scripts_encrypt/encrypt-idip-operator.sh`（无参默认账号；自动写 `starcrystal.json`）
- Windows：`scripts_encrypt/encrypt-idip-operator.ps1`
- 详见 [`部署与验收.md`](./部署与验收.md) §3.3
