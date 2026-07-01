# 玩家福利任务 API（与 IDIP 对照）

> **路径前缀**：`/api/v1/tasks/*`  
> **鉴权**：`Authorization: Bearer <accessToken>`（游客/手机号登录均可）  
> **实现**：`internal/api/task_handlers.go`、`internal/service/task_service.go`  
> **策划**：《福利页任务系统策划.md》  
> **运营查进度**：优先用 IDIP `GET /idip/v1/tasks/user-progress?accountId=`（无需玩家 token）

---

## 通用响应

```json
{ "code": 0, "message": "success", "data": { } }
```

任务专用错误码见策划 §9：`1406` 未登录 · `1421` 不可领 · `1422` 已领 · `1425` 广告凭证无效。

---

## 1. `GET /api/v1/tasks/welfare`

**用途**：福利页拉取任务列表、七日签到状态、档位策略（客户端 `WelfareTaskController` / `SevenDaySignInPopup`）。

**Query**

| 参数 | 必填 | 说明 |
|------|------|------|
| `lang` | 建议 | 文案语言，如 `zh`；影响部分 title 解析 |

**成功 `data`（`WelfareTasksData`）**

| 字段 | 说明 |
|------|------|
| `todayYmd` | 上海自然日 `YYYYMMDD` |
| `tierPolicy` | 当前 P0/P1/P2 开关（与 IDIP 一致） |
| `signin7d` | 七日链：`chain, canClaim, dayRewards[], nextDayIndex`… |
| `tasks[]` | 每条：`taskId, status, progress, target, rewardGold, adBonusGold, category`… |
| `byCategory` | 按 `daily/play/ad/limited` 分组 |

**`status` 枚举**：`locked` · `in_progress` · `claimable` · `claimed`

**进度来源（服务端钩子，非本接口写入）**

- `POST /api/v1/rank/activity` → 活跃秒、连玩、对局数  
- `POST /api/v1/ads/complete` → 广告次数 + adBonus 凭证  
- 注册带 `inviteCode` → 邀请人 `daily_invite`  
- `POST /api/v1/tasks/report` → 页面浏览等  

---

## 2. `POST /api/v1/tasks/claim`

**用途**：领取指定任务奖励（**唯一**合法发币入口，禁止客户端本地改 `curgold`）。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `taskId` | string | 是 | 如 `signin_7d`、`daily_free_claim`、`play_daily_60s` |
| `adBonus` | bool | 否 | 签到等支持广告加成的任务；需 5 分钟内有过 `ads/complete` |

**成功 `data`（`TaskClaimResult`）**

| 字段 | 说明 |
|------|------|
| `taskId` | 任务 ID |
| `grantedGold` | 本次发放金币 |
| `curgold` | 发放后余额 |
| `dailyCapRemaining` | 日产出 cap 剩余 |
| `signin7d` | 签到类任务会带回更新后的七日状态 |

---

## 3. `POST /api/v1/tasks/report`

**用途**：客户端上报可信任度较高的事件，推进任务 metric。

**请求体**

| 字段 | 说明 |
|------|------|
| `event` | `page_view` · `share_success` |
| `page` | `page_view` 时必填，如 `welfare` |

**支持事件**

| event | 行为 |
|-------|------|
| `page_view` | 记录 welfare 页访问（P1 `daily_visit_welfare` 等） |
| `share_success` | 分享成功日标记 |

---

## 4. 关联接口（非 tasks 路径）

| 方法 | 路径 | 与任务关系 |
|------|------|------------|
| POST | `/api/v1/rank/activity` | `play_daily_60s`、`play_milestone_300s`、`daily_play_sessions` |
| POST | `/api/v1/ads/start` | 广告会话开始 |
| POST | `/api/v1/ads/complete` | `ad_daily_*`、`ad_first_today`；写 adBonus 凭证 |
| POST | `/api/v1/auth/register-by-code` | body `inviteCode` → 邀请人 `daily_invite` |

---

## 5. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-05-19 | 初版，与 IDIP_API.md 交叉引用 |
