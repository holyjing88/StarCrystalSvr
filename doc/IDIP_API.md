# IDIP 运营接口说明（v1 已实现 · v1.2 经济查询已实现 · v1.1 部分规划）

> **IDIP**（In-game Data Interface Platform）：仅供**内网**运营/工具调用的 HTTP JSON API，路径前缀 `/idip/v1/`。  
> **实现**：`internal/api/idip_handlers.go`、`idip_economy_handlers.go`、`idip_task_handlers.go`  
> **网页客户端**：`idip-webclient/`（`src/api/idipClient.ts`）  
> **关联玩家 API**：`doc/PLAYER_TASK_API.md`（福利任务领奖，需 Bearer，非 IDIP）

---

## 0. 通用约定

### 0.1 鉴权与网络

| 项 | 说明 |
|----|------|
| **来源 IP** | 仅允许回环 / 私网（`127.0.0.1`、`10/8`、`172.16/12`、`192.168/16`）。公网来源 → HTTP **403**，`code=1403`，`message=idip forbidden`（用例 IDP-001）。 |
| **Key（脚本）** | 请求头 **`X-IDIP-Key`** 等于 `starcrystal.json` → `idip.key`（默认 `change-me-in-production`）。缺失或错误 → **403**，`code=1403`（用例 IDP-002）。 |
| **Bearer** | IDIP **不使用** 玩家 `Authorization`；改金、查进度均通过 body/query 的 `accountId`。 |

### 0.2 响应信封

与玩家 API 相同：

```json
{
  "code": 0,
  "message": "success",
  "data": { }
}
```

| code | 含义 | 典型 HTTP |
|------|------|-----------|
| 0 | 成功 | 200 |
| 1001 | 功能未实现（P2） | 501 |
| 1400 | 参数/业务错误 | 400 |
| 1403 | IDIP 禁止（IP/Key） | 403 |
| 1405 | 方法不允许 | 405 |
| 1420 | 任务不存在（upsert） | 400 |
| 1501 | 经济/任务服务未配置 | 503 |
| 2002 | 服务端内部错误 | 500 |

### 0.3 时区

金币自然日、任务日重置、月末结算：`Asia/Shanghai`（与 `GoldLedgerService`、任务引擎一致）。

---

## 0.4 运营登录 `/idip/v1/auth/*`（已实现）

| 方法 | 路径 |
|------|------|
| POST | `/idip/v1/auth/login` |
| POST | `/idip/v1/auth/logout` |
| POST | `/idip/v1/auth/heartbeat` |

运营台：login 后 `X-IDIP-Session` + 30s 心跳。Vitest/脚本：仅 `X-IDIP-Key`。  
密码：`idip.operators[].passwordEnc`（AES-GCM），密钥 `operatorCipherKey` 或 env `IDIP_OPERATOR_CIPHER_KEY`。  
生成（Windows 需传参）：`tools/idip-webclient/scripts/encrypt-idip-operator.ps1`。  
Linux 无参默认：`tools/idip-webclient/scripts/encrypt-idip-operator.sh`（`ops_admin` / `change-me-ops-password`；`MERGE=1` 写入 `release/configs/starcrystal.json`）。

---

## 0.5 游戏与 H5 `/idip/v1/games/*`（已实现）

`games/list|upsert|batch-upsert|delete|h5/upload`、`audit/logs`。  
玩家 `GET /api/v1/games`：SHA256 `configVersion` + download 三字段；缺 v= 跳过并打日志。  
备份：`games.json.bak.*` 与 `h5_backup` 各保留 10 份。

---

---

## 1. 金币 · `/idip/v1/gold/*`

### 1.1 `POST /idip/v1/gold/set-user`（IDP-003 · 已实现）

**用途**：运营对指定账号调整 **`curgold`**（当前金币）。走 `GoldLedgerService.ApplyGold`，**跳过日产出上限**（`SkipDailyCap=true`），仍会更新 Redis 月 delta 与福利当月金币榜（若未封号）。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `accountId` | string | 是 | 玩家账号 ID（`auth_accounts.account_id`） |
| `op` | string | 是 | `add` 增加 · `deduct` 扣减 · `set` 设为绝对值（空串同 `set`） |
| `amount` | number | 是 | 与 `op` 配合的数值（≥0） |
| `bizType` | string | 否 | 流水业务类型，默认空；建议 `idip_manual` |

**成功 `data`**（`GoldApplyResult`）

| 字段 | 说明 |
|------|------|
| `before` / `after` | 含 `curgold`、`totalgold`、`curtoken`、`totaltoken` |
| `requestedDelta` | 请求变动量 |
| `grantedDelta` | 实际变动（扣减失败时可能为 0） |
| `dailyCapRemaining` | 日 cap 剩余（IDIP Add 通常不扣 cap，仅展示） |

**错误**：`accountId` 空 → 1400；余额不足扣减 → 1400 message；economy 未配置 → 1501。

**示例**

```bash
curl -sS -X POST "http://127.0.0.1:8080/idip/v1/gold/set-user" \
  -H "Content-Type: application/json" -H "X-IDIP-Key: $KEY" \
  -d '{"accountId":"acc_test","op":"add","amount":100,"bizType":"idip_manual"}'
```

---

### 1.2 `GET /idip/v1/gold/month-user`（IDP-004 · 已实现 v1.2）

**用途**：查询用户在某自然月内通过 **Add** 累计的 granted 金币（Redis 月键）。

**Query**

| 参数 | 必填 | 说明 |
|------|------|------|
| `accountId` | 是 | 账号 ID |
| `yyyymm` | 否 | 如 `202605`；默认当前上海月 |

**成功 `data`**

```json
{
  "yyyymm": "202605",
  "accountId": "acc_test",
  "userGoldDelta": 1234.5,
  "redisKey": "sr:gold:month:202605:user:acc_test:gold_delta"
}
```

---

### 1.3 `POST /idip/v1/gold/recalc-server-delta-total`（IDP-010 · 未实现）

**用途（规划）**：按流水重算全服月 `server_gold_delta_total`，用于运维对账。  
**现状**：HTTP **501**，`code=1001`。

**别名**：`POST /idip/v1/gold/recalc-total`（同 handler）。

---

### 1.4 `GET /idip/v1/gold/day-user`（IDP-007 · 已实现 v1.2）

**用途**：查询用户当日产出计数（`sr:gold:day:{YYYYMMDD}:user:*`），用于日 cap 排查。

**Query**

| 参数 | 必填 | 说明 |
|------|------|------|
| `accountId` | 是 | 玩家账号 |
| `yyyymmdd` | 否 | 默认当日（`gold.timezone`） |

**成功 `data`**：`yyyymmdd`、`accountId`、`dayUsed`、`dailyCap`、`remaining`、`byBiz`（各 biz 已用额度）。

**自动化**：`tools/run-idip-economy-live-tests.ps1`（客户端仓库）。

---

## 2. 福利结算 · `/idip/v1/welfare/*`

### 2.1 `GET /idip/v1/welfare/month-token-pool`（IDP-005 · 已实现）

**用途**：读取指定月份 **Token 兑换池**总量（月末按池子比例给玩家发 `curtoken`）。

**Query**：`yyyymm` 可选，默认当前月。

**成功 `data`**

| 字段 | 说明 |
|------|------|
| `yyyymm` | 月份 |
| `monthTokenPool` | 池大小 |
| `from` | 可选；`redis` 命中则无此字段；未命中时 `json` 表示来自配置文件默认值 |

---

### 2.2 `POST /idip/v1/welfare/set-month-token-pool`（IDP-005 · 已实现）

**用途**：写入 Redis 中该月 Token 池（覆盖配置默认值）。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pool` | number | 是 | 池总量 |
| `yyyymm` | string | 否 | 默认当前月 |

---

### 2.3 `POST /idip/v1/welfare/run-monthly-settlement`（IDP-006 · 已实现）

**用途**：触发**月末批处理**：按当月 `curgold` 与全服分母折算 `curtoken`，`curgold` 清零，`totalgold` 增加等（详见经济文档 ECO-D）。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `yyyymm` | string | 否 | 指定结算月；空为当前策略月 |
| `force` | bool | 否 | `true` 时可重复跑（测试用）；生产慎用 |

**注意**：需 Redis + MySQL + `Settlement` 已配置；多实例并发仅一单获得锁（ECO-D-006）。

---

### 2.4 `GET /idip/v1/welfare/settlement-status`（规划 v1.1）

**用途**：查询上次结算时间、处理用户数、是否锁占用。  
**现状**：未实现。

---

### 2.5 `GET /idip/v1/welfare/monthly-token-leaderboard`（IDP-011 · 已实现 v1.2）

**用途**：查询指定**历史月份**月末批处理中，按 **`token_delta` 降序** 的玩家排名（Top **1000**，分页）。

**Query**

| 参数 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `yyyymm` | 是 | — | `YYYYMM` |
| `page` | 否 | 1 | ≥1 |
| `pageSize` | 否 | 50 | 1–100 |
| `sort` | 否 | `token_delta_desc` | v1.2 仅支持此值 |

**数据源**：`welfare_exchange_log`（`yyyymm` + `token_delta` 索引）。

**成功 `data.items[]`**：`rank`、`accountId`、`displayName`、`inviteCode`、`goldSpent`、`tokenDelta`、`rate`、`settledAt`。

**用例**：IDP-011（见客户端 `doc/玩家金币Token商业化测试方案-v7.1.md`）。

---

### 2.6 `GET /idip/v1/economy/user-profile`（IDP-012 · 已实现 v1.2）

**用途**：按 **`accountId`** 或 **`inviteCode`** 查询用户经济画像（金币/Token 四字段、日产出、月 Redis、历史结算、可选四榜）。

**Query**：`accountId` 与 `inviteCode` 二选一；可选 `settlementMonths`（默认 6）、`includeRanks`（默认 false）。

**成功 `data` 分组**：`account`、`economy`、`dailyProduce`、`monthActivity`、`monthlySettlements[]`、`lifetimeSummary`、`ranks`（可选）、`inviteStats`。

**用例**：IDP-012（见客户端 `doc/玩家金币Token商业化测试方案-v7.1.md`）。

---

## 3. 福利任务 · `/idip/v1/tasks/*`

配置持久化：`release/configs/tasks_welfare.json`；运行时覆盖：`UpsertTaskOverride`、内存 `tierPolicy`。

### 3.1 `GET /idip/v1/tasks/definitions`（TSK-A-001 · 已实现）

**用途**：运营查看**全量任务目录**（含 P1/P2 未开放档位）及当前 **tier 开关**、**activeCount**（对玩家可见任务数）。

**成功 `data`**

| 字段 | 说明 |
|------|------|
| `tierPolicy` | `{ p0Enabled, p1Enabled, p2Enabled }` |
| `tasks[]` | `taskId, tier, category, enabled, target, rewardGold, metric` |
| `activeCount` | `ListActiveTaskDefs()` 数量（受 tier 与 enabled 影响） |

---

### 3.2 `POST /idip/v1/tasks/tier-policy`（TSK-S-006 · 已实现）

**用途**：热更新 **P0/P1/P2 档位开关**（进程内全局，重启后以 JSON 文件为准 unless 再次 POST）。

**请求体**：`{ "p0Enabled": true, "p1Enabled": false, "p2Enabled": false }`

**成功 `data`**：写入后的 `tierPolicy`。

**影响**：玩家 `GET /api/v1/tasks/welfare` 返回的 `tasks[]` 条数随 `activeCount` 变化。

---

### 3.3 `POST /idip/v1/tasks/definition/upsert`（TSK-A-002 · 已实现）

**用途**：覆盖单任务 `enabled` / `rewardGold` / `target`（内存覆盖，非全量落库）。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `taskId` | string | 是 |  catalog 中已存在的 ID |
| `enabled` | bool | 否 | 指针语义：仅传则更新 |
| `rewardGold` | number | 否 | 同上 |
| `target` | number | 否 | 同上 |

**错误**：未知 `taskId` → 1420。

---

### 3.4 `GET /idip/v1/tasks/user-progress`（运营 · 已实现）

**用途**：按 `accountId` 查看与玩家 welfare 相同的任务进度（**无需 Bearer**）。  
**实现**：内部调用 `TaskService.GetWelfare`（与 `GET /api/v1/tasks/welfare` 同结构）。

**Query**：`accountId` 必填。

---

### 3.5 `GET /idip/v1/tasks/tier-policy`（规划 v1.1）

**用途**：只读当前 tier，避免误 POST。  
**现状**：未实现；可用 `GET definitions` 的 `tierPolicy` 字段代替。

---

### 3.6 `POST /idip/v1/tasks/reset-user-day`（规划 v1.1 · 高危）

**用途**：清除指定账号当日任务领取标记 / 进度（客服慎用）。  
**现状**：未实现。

---

## 4. 运维 · 规划 v1.1

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/idip/v1/health` | 返回 `code=0`、服务版本、economy/task 是否就绪（无需 body） |
| POST | `/idip/v1/config/reload-tasks` | 热加载 `tasks_welfare.json` |

---

## 5. 与玩家 API 对照

| 能力 | IDIP | 玩家 API |
|------|------|----------|
| 查任务列表 | `GET .../tasks/user-progress?accountId=` | `GET /api/v1/tasks/welfare` + Bearer |
| 领任务奖 | —（禁止，防作弊） | `POST /api/v1/tasks/claim` |
| 改金币 | `POST .../gold/set-user` | 仅游戏内行为间接变更 |
| 改 tier/奖励表 | tier-policy / upsert | 不可 |

详见 **`PLAYER_TASK_API.md`**。

---

## 6. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0-doc | 2026-05-19 | 初版：已实现接口注释 + v1.1 扩展规划 |
| v1.0-impl | — | 与 server-go 当前代码一致 |
| v1.2-plan | 2026-05-29 | 规划 `monthly-token-leaderboard`、`economy/user-profile`（商业化测试专文） |
| v1.2-impl | 2026-05-29 | 实现 IDP-007/011/012；`gold/month-user` 读 Redis `userGoldDelta` |
