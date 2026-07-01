package service

import (
	"context"
	"testing"
	"time"
)

// TestRankRedisPing127 本机 127.0.0.1:6379 可连时通过；无 Redis 时 Skip（不拖 CI）。
func TestRankRedisPing127(t *testing.T) {
	st := NewRankStoreFromSources(RankRedisConfig{Addr: "127.0.0.1:6379", DB: 0})
	r, ok := st.(*redisRankStore)
	if !ok {
		t.Fatal("expected redis backend for non-empty addr")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis 127.0.0.1:6379 不可用（跳过）: %v", err)
	}
}
