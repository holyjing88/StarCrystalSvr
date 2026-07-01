# StarCrystal Redis 键结构（无 DDL，运行时写入）

Redis 不使用表结构；服务端按约定 **key 前缀 `sr:`** 写入。重建 = 清空 `sr:*` 键（见 `tools/scripts/rebuild-redis.sh`）。

配置来源：`release/configs/starcrystal.json` → `redisAddr` / `redisPassword` / `redisDb`。

## Gold（`internal/service/gold_redis_store.go`）

| 类型 | Key 模式 | 说明 |
|------|----------|------|
| string | `sr:gold:month:{yyyymm}:server_gold_delta_total` | 全服当月金币增量 |
| string | `sr:gold:month:{yyyymm}:user:{accountID}:gold_delta` | 用户当月金币增量 |
| string | `sr:gold:day:{yyyymmdd}:user:{accountID}` | 用户日金币产出（TTL 48h） |
| hash | `sr:gold:day:{yyyymmdd}:user:{accountID}:biz` | 分业务日产出（TTL 48h） |
| string | `sr:gold:month:{yyyymm}:token:pool` | 当月 token 池 |
| hash | `sr:gold:month:{yyyymm}:meta` | 月元数据（如 `settlement_done`） |
| string | `sr:gold:lock:monthly_settlement:{yyyymm}` | 月结算锁 |
| string | `sr:gold:redeem_gift:lock:{accountID}` | 兑换礼包锁（TTL 60s） |

## Rank（`internal/service/rank_store.go`）

| 类型 | Key | 说明 |
|------|-----|------|
| zset | `sr:rank:popularity` | 游戏人气榜 |
| zset | `sr:rank:activity:{weekID}` | 周活跃榜 |
| string | `sr:rank:welfare:gold:total` | 福利金币累计（旧） |
| string | `sr:rank:welfare:token:total` | 福利 token 累计（旧） |

## Welfare rank（`internal/service/welfare_rank_store.go`）

| 类型 | Key | 说明 |
|------|-----|------|
| zset | `sr:rank:welfare:gold:cur` | 福利金币当前榜 |
| zset | `sr:rank:welfare:gold:total` | 福利金币累计榜 |
| zset | `sr:rank:welfare:token:cur` | 福利 token 当前榜 |
| zset | `sr:rank:welfare:token:total` | 福利 token 累计榜 |

## 说明

- Task 进度当前仅内存实现（`task_store.go`），无 Redis 键。
- 首次业务写入时自动创建键；**rebuild 后无需 seed 初始数据**。
