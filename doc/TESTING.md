# StarCrystal server-go 测试体系

本文档描述 API 契约/冒烟测试、内部反外挂单元测试、以及与 MySQL 配套的集成测试如何组织与运行。

---

## 1. 快速运行

在项目根目录 `server-go`：

```bash
cd server-go
go test ./...
```

默认 `go test ./...` **需要可用的 MySQL**：由环境变量 `AUTH_MYSQL_DSN` 或仓库内 `release/configs/starcrystal.json` 的 `authMysqlDsn` 提供（与正式服务一致）；否则 `NewAuthService` 在测试进程内即失败。纯包测试（如 `internal/antifraud`、`internal/regpolicy`）不连库；API 冒烟见 `internal/api/smoke_api_test.go`。

---

## 2. 套件分层

| 层级 | 目录 / 构建标签 | 用途 |
|------|-----------------|------|
| API 冒烟与契约 | `internal/api/*_test.go` | 对 **所有已注册 HTTP 路由**做状态码与业务 `code` 校验 |
| 广告频控（进程内） | `internal/antifraud/*_test.go` | `AdsGate`：slot 白名单、按账号/IP 的分钟计数、环路 IP 特例 |
| 注册风控判定（纯函数） | `internal/regpolicy/*_test.go` | 「下一笔注册是否应关激励奖励」的组合表测试 |
| 客户端 IP 解析 | `internal/httpx/*_test.go` | `X-Forwarded-For` / `X-Real-Ip`、`RemoteAddr` |
| MySQL 集成 | `internal/integration/` + **`-tags=integration`** | 同日设备维度与 `Decide`/`INSERT` 等与真实库一致的行为 |

---

## 3. API 层覆盖范围（`internal/api`）

测试使用 `httptest.Server` + `NewServer(service.RankRedisConfig{}).Handler()`，**不启动独立进程**。

### 3.1 已覆盖的公开路由（与 `internal/api/server.go` 对齐）

- **GET** `/`、`/healthz`、`/favicon.ico`
- **GET** `/api/v1/games`：缺 `appVersion`/`platform` → `400` + `1400`；可设 `GAMES_CONFIG` 指向临时 JSON → `200` + `code 0`
- **OPTIONS** 任意路径 → `204` + CORS 响应头
- **GET** `/api/v1/wallet/balance`、`/api/v1/wallet/ledger`
- **GET** `/api/v1/welfare/redeem-gift/{redeemId}`（占位成功）
- **POST** `/api/v1/welfare/redeem-token-gift`（兑换礼品；原 `wallet/withdraw/apply` 已废弃）
- **POST** `/api/v1/ads/callback/{network}`（占位）
- **POST** `/api/v1/ads/start`、`/api/v1/ads/complete`：无 Bearer → `401` + `1406`；非法 token → `401` + `1407`
- **游戏收藏**：`POST/GET/DELETE /api/v1/games/favorite*` → `TestGameFavorite_AddListRemove`（游客 Bearer 增删查）
- **鉴权链**：`POST /api/v1/auth/guest` 签发 token 后调 `ads/start`（需 MySQL；见 `TestSmoke_Ads_Start_ValidGuestToken_WithMysql`）
- **Auth（错误路径）**：多条 `POST` 非法 JSON → `400` + `1400`；OAuth 非法 `provider`；缺 token 的 `/auth/me`、`/auth/my/team`、`/auth/gm/metrics`；短密码注册、错误登录、`/auth/sendverificationcode` 空体（缺账号）、`/auth/register` 短密码等

### 3.2 路由与中间件边角

- 对「仅 GET」的路径发 **POST** → `405 Method Not Allowed`
- **POST** `/api/v1/auth/sendverificationcode`：注册发码（`purpose=register`）、找回密码发码（`purpose=password_reset`）；Unity 客户端会根据账号自动填 `channel=phone|email`；服务端若收到 `channel` 则校验与账号一致；遗留 JSON 仅含 `email` 无 `account` 时视为找回密码

### 3.3 刻意未做「成功链路」断言的接口

以下依赖外部环境，默认套件**只做失败/占位类断言**，避免 CI 耦合：

- 真实 SMTP / 邮箱验证码、`AUTH_SMS_MOCK` 短信
- Google / Facebook **有效** OAuth
- MySQL 上的注册、登录、`ads/start`→`watchId` 全链路、`1430` 封号等 → 见 **§5 集成测试**

---

## 4. 内部反外挂模块测试

### 4.1 `internal/antifraud`（`AdsGate`）

覆盖点包括：

- 环境变量 **`AD_SLOT_ALLOWLIST`**：`CheckSlotValid` 行为
- **`AD_START_PER_MIN_ACCOUNT`、`AD_START_PER_MIN_IP`**：同一自然分钟内 burst 超限 → `ErrRateLimitStart`
- **`127.0.0.1` / `::1`**：`AllowStart` 不累计 IP 维（与设计一致）
- **`AD_COMPLETE_PER_MIN_ACCOUNT`**：`AllowComplete` 侧 burst；可将 **`AD_COMPLETE_PER_MIN_IP`** 设为 `0` 关闭 IP 维

### 4.2 `internal/regpolicy`

对 `RegistrationShouldDisableAdRewards(...)` 的输入组合做表驱动测试（设备/IP 阈值、维度关闭等）。

### 4.3 `internal/httpx`

对 `ClientIP` 在有 `X-Forwarded-For` / `X-Real-Ip` / 仅 `RemoteAddr` 时的优先级做断言。

### 4.4 `internal/service`

业务逻辑测试若构造 `AuthService`，须具备合法 DSN（或与集成测试相同环境）。

**例外（无需 MySQL）**：`game_favorite_test.go` 的 `TestGameFavoriteService_*` 使用内存 `GameFavoriteStore`；`rank_activity_test.go` 等活跃榜/任务单元测试同理。

---

## 5. MySQL 集成测试（`-tags=integration`）

源码：`internal/integration/registration_mysql_test.go`（**仅**在带 `integration` 构建标签时编译）。

前提：

1. 环境变量 **`STARCRYSTAL_INTEGRATION_MYSQL`** 设为可写的测试库 DSN（与 `AUTH_MYSQL_DSN` 形式类似）。
2. 库表已包含 `auth_accounts.registration_ip`、`auth_accounts.ad_rewards_disabled` 等（见 `tools/scripts/dbscripts/sql/starcrystal_auth_mysql.sql` 全量脚本）。

运行示例（PowerShell）：

```powershell
cd server-go
$env:STARCRYSTAL_INTEGRATION_MYSQL = 'USER:PWD@tcp(127.0.0.1:3306)/starcrystal_auth?parseTime=true&loc=Local'
go test ./internal/integration -tags=integration -count=1
```

用例会向库中插入两行带相同 `device_id` 的当日注册记录，再断言「第三笔逻辑注册」在策略下会被判定应关闭广告奖励。

---

## 6. v7.1 经济与福利榜测试用例（落地文档）

完整用例表、执行顺序、IDIP curl 模板与跟进登记见：

- **`../StarCrystal2022/doc/后台测试用例-v7.1.md`**
- **IDIP 接口说明（v1 + v1.2 经济查询已实现）**：**`doc/IDIP_API.md`**
- **玩家福利任务 API**：**`doc/PLAYER_TASK_API.md`**
- **网页回归**：**`../idip-webclient/`**（`npm run regression`）

当前自动化覆盖仍以 §3 冒烟 + `gold_ledger_test.go` 日上限单元测试为主；**IDIP v1.2**（`day-user`、月榜、用户画像）可由客户端 `tools/run-idip-economy-live-tests.ps1` + `idip_economy_handlers_test.go` 覆盖；**月末结算、四福利榜 INT** 仍有多数 ⬜，按后台测试用例 §6 逐步补齐。

---

## 7. 维护须知

- 在 **`server.go` 增加或删除路由** 时，请同步在 **`internal/api/smoke_api_test.go`**（或分拆的新测试文件）中增加对应契约用例。
- **`internal/antifraud`** 与环境变量耦合：单测中用 **`t.Setenv`**，避免并行用例争抢全局 `os.Getenv`（当前用例默认未启用 `t.Parallel()`）。
- 与 Unity / H5 联调相关的字段与错误码，可与 `../StarCrystal2022/doc/ADS_INTEGRATION.md` 交叉对照。

---

## 8. 运维脚本验收（SCR-*）

`tools/scripts` 与 `release/` 启停脚本验收见 **`doc/SCRIPTS_TESTING.md`**：

```powershell
cd server-go
.\tools\scripts\test-scripts.ps1 -Live
```

```bash
cd server-go
SCRIPTS_TEST_LIVE=1 bash tools/scripts/test-scripts.sh
```

---

## 9. README 摘要

顶层 `README.md` 中有运行命令的快速入口；完整说明以 **本文档** 为准。
