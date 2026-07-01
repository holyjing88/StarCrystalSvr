package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// GoldRedisStore monthServerGoldDeltaTotal, per-user month delta, daily produce caps.
type GoldRedisStore interface {
	IncrMonthServerDelta(ctx context.Context, yyyymm string, delta float64) error
	GetMonthServerDelta(ctx context.Context, yyyymm string) (float64, error)
	SetMonthServerDelta(ctx context.Context, yyyymm string, value float64) error
	IncrUserMonthDelta(ctx context.Context, yyyymm, accountID string, delta float64) error
	GetUserMonthDelta(ctx context.Context, yyyymm, accountID string) (float64, error)
	GetDayUsed(ctx context.Context, yyyymmdd, accountID string) (float64, error)
	IncrDayUsed(ctx context.Context, yyyymmdd, accountID string, delta float64) error
	GetDayBizUsed(ctx context.Context, yyyymmdd, accountID, bizType string) (float64, error)
	IncrDayBizUsed(ctx context.Context, yyyymmdd, accountID, bizType string, delta float64) error
	GetMonthTokenPool(ctx context.Context, yyyymm string) (float64, bool, error)
	SetMonthTokenPool(ctx context.Context, yyyymm string, pool float64) error
	IsSettlementDone(ctx context.Context, yyyymm string) (bool, error)
	MarkSettlementDone(ctx context.Context, yyyymm string) error
	ClearMonthUserGoldDeltas(ctx context.Context, yyyymm string) error
	TryRedeemGiftLock(ctx context.Context, accountID string) (bool, error)
}

func monthServerDeltaKey(yyyymm string) string {
	return "sr:gold:month:" + yyyymm + ":server_gold_delta_total"
}

func userMonthDeltaKey(yyyymm, accountID string) string {
	return "sr:gold:month:" + yyyymm + ":user:" + accountID + ":gold_delta"
}

func dayUsedKey(yyyymmdd, accountID string) string {
	return "sr:gold:day:" + yyyymmdd + ":user:" + accountID
}

func dayBizKey(yyyymmdd, accountID string) string {
	return "sr:gold:day:" + yyyymmdd + ":user:" + accountID + ":biz"
}

func monthTokenPoolKey(yyyymm string) string {
	return "sr:gold:month:" + yyyymm + ":token:pool"
}

func monthMetaKey(yyyymm string) string {
	return "sr:gold:month:" + yyyymm + ":meta"
}

const goldDayKeyTTL = 48 * time.Hour

// --- memory ---

type memoryGoldRedisStore struct {
	mu             sync.Mutex
	serverDelta    map[string]float64
	userDelta      map[string]float64
	dayUsed        map[string]float64
	dayBiz         map[string]map[string]float64
	tokenPool      map[string]float64
	settlementDone map[string]bool
}

func newMemoryGoldRedisStore() *memoryGoldRedisStore {
	return &memoryGoldRedisStore{
		serverDelta:    make(map[string]float64),
		userDelta:      make(map[string]float64),
		dayUsed:        make(map[string]float64),
		dayBiz:         make(map[string]map[string]float64),
		tokenPool:      make(map[string]float64),
		settlementDone: make(map[string]bool),
	}
}

func (s *memoryGoldRedisStore) IncrMonthServerDelta(_ context.Context, yyyymm string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverDelta[yyyymm] += delta
	return nil
}

func (s *memoryGoldRedisStore) GetMonthServerDelta(_ context.Context, yyyymm string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serverDelta[yyyymm], nil
}

func (s *memoryGoldRedisStore) SetMonthServerDelta(_ context.Context, yyyymm string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverDelta[yyyymm] = value
	return nil
}

func (s *memoryGoldRedisStore) IncrUserMonthDelta(_ context.Context, yyyymm, accountID string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := yyyymm + ":" + accountID
	s.userDelta[k] += delta
	return nil
}

func (s *memoryGoldRedisStore) GetUserMonthDelta(_ context.Context, yyyymm, accountID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.userDelta[yyyymm+":"+accountID], nil
}

func (s *memoryGoldRedisStore) GetDayUsed(_ context.Context, yyyymmdd, accountID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dayUsed[yyyymmdd+":"+accountID], nil
}

func (s *memoryGoldRedisStore) IncrDayUsed(_ context.Context, yyyymmdd, accountID string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayUsed[yyyymmdd+":"+accountID] += delta
	return nil
}

func (s *memoryGoldRedisStore) GetDayBizUsed(_ context.Context, yyyymmdd, accountID, bizType string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.dayBiz[yyyymmdd+":"+accountID]
	if m == nil {
		return 0, nil
	}
	return m[bizType], nil
}

func (s *memoryGoldRedisStore) IncrDayBizUsed(_ context.Context, yyyymmdd, accountID, bizType string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := yyyymmdd + ":" + accountID
	if s.dayBiz[k] == nil {
		s.dayBiz[k] = make(map[string]float64)
	}
	s.dayBiz[k][bizType] += delta
	return nil
}

func (s *memoryGoldRedisStore) GetMonthTokenPool(_ context.Context, yyyymm string) (float64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.tokenPool[yyyymm]
	return v, ok, nil
}

func (s *memoryGoldRedisStore) SetMonthTokenPool(_ context.Context, yyyymm string, pool float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenPool[yyyymm] = pool
	return nil
}

func (s *memoryGoldRedisStore) IsSettlementDone(_ context.Context, yyyymm string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settlementDone[yyyymm], nil
}

func (s *memoryGoldRedisStore) MarkSettlementDone(_ context.Context, yyyymm string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settlementDone[yyyymm] = true
	return nil
}

func (s *memoryGoldRedisStore) ClearMonthUserGoldDeltas(_ context.Context, yyyymm string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := yyyymm + ":"
	for k := range s.userDelta {
		if strings.HasPrefix(k, prefix) {
			delete(s.userDelta, k)
		}
	}
	return nil
}

func (s *memoryGoldRedisStore) TryRedeemGiftLock(_ context.Context, accountID string) (bool, error) {
	return true, nil
}

// --- redis ---

type redisGoldStore struct {
	rdb *redis.Client
}

func newRedisGoldStore(addr, password string, db int) *redisGoldStore {
	return &redisGoldStore{rdb: redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})}
}

func (s *redisGoldStore) IncrMonthServerDelta(ctx context.Context, yyyymm string, delta float64) error {
	return s.rdb.IncrByFloat(ctx, monthServerDeltaKey(yyyymm), delta).Err()
}

func (s *redisGoldStore) GetMonthServerDelta(ctx context.Context, yyyymm string) (float64, error) {
	v, err := s.rdb.Get(ctx, monthServerDeltaKey(yyyymm)).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (s *redisGoldStore) SetMonthServerDelta(ctx context.Context, yyyymm string, value float64) error {
	return s.rdb.Set(ctx, monthServerDeltaKey(yyyymm), value, 0).Err()
}

func (s *redisGoldStore) IncrUserMonthDelta(ctx context.Context, yyyymm, accountID string, delta float64) error {
	return s.rdb.IncrByFloat(ctx, userMonthDeltaKey(yyyymm, accountID), delta).Err()
}

func (s *redisGoldStore) GetUserMonthDelta(ctx context.Context, yyyymm, accountID string) (float64, error) {
	v, err := s.rdb.Get(ctx, userMonthDeltaKey(yyyymm, accountID)).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (s *redisGoldStore) GetDayUsed(ctx context.Context, yyyymmdd, accountID string) (float64, error) {
	v, err := s.rdb.Get(ctx, dayUsedKey(yyyymmdd, accountID)).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (s *redisGoldStore) IncrDayUsed(ctx context.Context, yyyymmdd, accountID string, delta float64) error {
	key := dayUsedKey(yyyymmdd, accountID)
	pipe := s.rdb.Pipeline()
	pipe.IncrByFloat(ctx, key, delta)
	pipe.Expire(ctx, key, goldDayKeyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisGoldStore) GetDayBizUsed(ctx context.Context, yyyymmdd, accountID, bizType string) (float64, error) {
	v, err := s.rdb.HGet(ctx, dayBizKey(yyyymmdd, accountID), bizType).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (s *redisGoldStore) IncrDayBizUsed(ctx context.Context, yyyymmdd, accountID, bizType string, delta float64) error {
	key := dayBizKey(yyyymmdd, accountID)
	pipe := s.rdb.Pipeline()
	pipe.HIncrByFloat(ctx, key, bizType, delta)
	pipe.Expire(ctx, key, goldDayKeyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisGoldStore) GetMonthTokenPool(ctx context.Context, yyyymm string) (float64, bool, error) {
	v, err := s.rdb.Get(ctx, monthTokenPoolKey(yyyymm)).Float64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (s *redisGoldStore) SetMonthTokenPool(ctx context.Context, yyyymm string, pool float64) error {
	return s.rdb.Set(ctx, monthTokenPoolKey(yyyymm), pool, 0).Err()
}

func (s *redisGoldStore) IsSettlementDone(ctx context.Context, yyyymm string) (bool, error) {
	v, err := s.rdb.HGet(ctx, monthMetaKey(yyyymm), "settlement_done").Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == "1", nil
}

func (s *redisGoldStore) MarkSettlementDone(ctx context.Context, yyyymm string) error {
	return s.rdb.HSet(ctx, monthMetaKey(yyyymm), "settlement_done", "1").Err()
}

func (s *redisGoldStore) ClearMonthUserGoldDeltas(ctx context.Context, yyyymm string) error {
	pattern := "sr:gold:month:" + yyyymm + ":user:*:gold_delta"
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pattern, 128).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func redeemGiftLockKey(accountID string) string {
	return "sr:gold:redeem_gift:lock:" + accountID
}

func (s *redisGoldStore) TryRedeemGiftLock(ctx context.Context, accountID string) (bool, error) {
	ok, err := s.rdb.SetNX(ctx, redeemGiftLockKey(accountID), "1", 60*time.Second).Result()
	return ok, err
}

// NewGoldRedisStore shares Redis connection settings with rank store.
func NewGoldRedisStore(file RankRedisConfig) GoldRedisStore {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		addr = strings.TrimSpace(file.Addr)
	}
	if addr == "" {
		return newMemoryGoldRedisStore()
	}
	password := file.Password
	if _, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		password = os.Getenv("REDIS_PASSWORD")
	}
	db := file.DB
	if db < 0 || db > 15 {
		db = 0
	}
	if d := strings.TrimSpace(os.Getenv("REDIS_DB")); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 0 && v < 16 {
			db = v
		}
	}
	return newRedisGoldStore(addr, password, db)
}
