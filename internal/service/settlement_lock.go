package service

import (
	"context"
	"time"
)

const settlementLockTTL = 2 * time.Hour

func settlementLockKey(yyyymm string) string {
	return "sr:gold:lock:monthly_settlement:" + yyyymm
}

// TryAcquireSettlementLock 多实例月末批处理分布式锁（Redis SET NX）。
func TryAcquireSettlementLock(ctx context.Context, g GoldRedisStore, yyyymm string) (release func(), ok bool, err error) {
	rg, okStore := g.(*redisGoldStore)
	if !okStore {
		return func() {}, true, nil
	}
	key := settlementLockKey(yyyymm)
	acquired, err := rg.rdb.SetNX(ctx, key, "1", settlementLockTTL).Result()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	return func() { _ = rg.rdb.Del(context.Background(), key).Err() }, true, nil
}
