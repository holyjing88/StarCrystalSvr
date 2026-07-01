# API · 用例 · 服务端用途对照

| 方法 | 路径 | 用例 ID | Vitest | 服务端实现用途（handler → 底层） |
|------|------|---------|--------|----------------------------------|
| POST | `/idip/v1/gold/set-user` | IDP-003 | IDP-003 | **`handleIdipGoldSetUser`** → `GoldLedger.ApplyGold`（跳过日 cap）：改 DB **`curgold`**、Redis 月 **user/server gold_delta**、刷新 **`welfare_gold_cur`** 榜；运营补扣币，非玩家任务领奖 |
| GET | `/idip/v1/gold/month-user` | IDP-004 | IDP-004 | **`handleIdipGoldMonthUser`**：返回 Redis 键 `sr:gold:month:{yyyymm}:user:{id}:gold_delta` 说明（v1 未读数值）；用于对账玩家当月产出累计 |
| POST | `/idip/v1/gold/recalc-server-delta-total` | IDP-010 | IDP-010 | **`handleIdipRecalcServerDelta`**：未实现（501）；规划重算全服月 delta |
| GET | `/idip/v1/welfare/month-token-pool` | IDP-005 | IDP-005 | **`handleIdipGetMonthTokenPool`** → `GoldRedis.GetMonthTokenPool` 或配置默认：查当月 **Token 兑换池** |
| POST | `/idip/v1/welfare/set-month-token-pool` | IDP-005 | IDP-005 | **`handleIdipSetMonthTokenPool`** → `GoldRedis.SetMonthTokenPool`：运营设定月末分 **Token** 的池子大小 |
| POST | `/idip/v1/welfare/run-monthly-settlement` | IDP-006 | IDP-006 | **`handleIdipRunSettlement`** → `Settlement.RunSettlement`：月末批处理（`curgold` 兑入 `totalgold`/发 `curtoken`、清 `curgold`、更新福利四榜） |
| GET | `/idip/v1/tasks/definitions` | TSK-A-001 | TSK-A-001 | **`handleIdipTaskDefinitions`** → `AllTaskDefsForAdmin` + **`activeCount`**（`ListActiveTaskDefs`）：运营看全表与当前开放档位数 |
| POST | `/idip/v1/tasks/tier-policy` | TSK-S-006 | TSK-S-006 | **`handleIdipTaskTierPolicy`** → **`SetTaskTierPolicy`**：热开关 P0/P1/P2；影响玩家 **`GET /api/v1/tasks/welfare`** 可见任务数 |
| POST | `/idip/v1/tasks/definition/upsert` | TSK-A-002 | TSK-A-002 | **`handleIdipTaskDefinitionUpsert`** → **`UpsertTaskOverride`**：热改单任务奖励/目标/启用；玩家 **`POST /tasks/claim`** 用新 `rewardGold` |
| GET | `/idip/v1/tasks/user-progress` | — | TSK-IDP-PROGRESS | **`handleIdipTaskUserProgress`** → **`TaskService.GetWelfare(accountId)`**（同玩家 welfare  payload，无需 token） |

参数与错误码：[`server-go/doc/IDIP_API.md`](../../server-go/doc/IDIP_API.md)
