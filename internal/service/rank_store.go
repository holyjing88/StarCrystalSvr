package service

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RankPlayRow 单条排行（score 为人气打开次数或活跃秒数）。
type RankPlayRow struct {
	GameID    string `json:"gameId,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	PlayCount int64  `json:"playCount"`
}

type rankStore interface {
	incrPopularity(ctx context.Context, gameID string, delta int64) (int64, error)
	topPopularity(ctx context.Context, limit int) ([]RankPlayRow, error)
	incrActivity(ctx context.Context, weekID, accountID string, delta int64) (int64, error)
	topActivity(ctx context.Context, weekID string, limit int) ([]RankPlayRow, error)
	activityMemberRank(ctx context.Context, weekID, accountID string) (rank int64, score int64, onBoard bool, err error)
	// 福利榜：全平台累计（不按 gameId 分桶）。
	incrWelfareGold(ctx context.Context, delta int64) (int64, error)
	welfareGoldTotal(ctx context.Context) (int64, error)
	incrWelfareToken(ctx context.Context, delta int64) (int64, error)
	welfareTokenTotal(ctx context.Context) (int64, error)
}

type rankCountPair struct {
	id string
	n  int64
}

func sortRankPairs(pairs []rankCountPair, limit int, forActivity bool) []RankPlayRow {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].n != pairs[j].n {
			return pairs[i].n > pairs[j].n
		}
		return strings.Compare(pairs[i].id, pairs[j].id) < 0
	})
	limit = ClampRankListLimit(limit)
	if limit > len(pairs) {
		limit = len(pairs)
	}
	out := make([]RankPlayRow, 0, limit)
	for i := 0; i < limit; i++ {
		row := RankPlayRow{PlayCount: pairs[i].n}
		if forActivity {
			row.AccountID = pairs[i].id
		} else {
			row.GameID = pairs[i].id
		}
		out = append(out, row)
	}
	return out
}

// --- 内存（dev / 单测）---

type memoryRankStore struct {
	mu            sync.Mutex
	pop           map[string]int64
	act           map[string]map[string]int64
	welGoldTotal  int64
	welTokenTotal int64
}

func newMemoryRankStore() *memoryRankStore {
	return &memoryRankStore{
		pop: make(map[string]int64),
		act: make(map[string]map[string]int64),
	}
}

func (s *memoryRankStore) incrPopularity(_ context.Context, gameID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pop[gameID] += delta
	return s.pop[gameID], nil
}

func (s *memoryRankStore) topPopularity(_ context.Context, limit int) ([]RankPlayRow, error) {
	s.mu.Lock()
	pairs := make([]rankCountPair, 0, len(s.pop))
	for id, n := range s.pop {
		pairs = append(pairs, rankCountPair{id: id, n: n})
	}
	s.mu.Unlock()
	return sortRankPairs(pairs, limit, false), nil
}

func (s *memoryRankStore) incrActivity(_ context.Context, weekID, accountID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wk := s.act[weekID]
	if wk == nil {
		wk = make(map[string]int64)
		s.act[weekID] = wk
	}
	wk[accountID] += delta
	return wk[accountID], nil
}

func (s *memoryRankStore) topActivity(_ context.Context, weekID string, limit int) ([]RankPlayRow, error) {
	s.mu.Lock()
	wk := s.act[weekID]
	pairs := make([]rankCountPair, 0, len(wk))
	for id, n := range wk {
		pairs = append(pairs, rankCountPair{id: id, n: n})
	}
	s.mu.Unlock()
	return sortRankPairs(pairs, limit, true), nil
}

func (s *memoryRankStore) activityMemberRank(_ context.Context, weekID, accountID string) (int64, int64, bool, error) {
	s.mu.Lock()
	wk := s.act[weekID]
	score, exists := wk[accountID]
	pairs := make([]rankCountPair, 0, len(wk))
	for id, n := range wk {
		pairs = append(pairs, rankCountPair{id: id, n: n})
	}
	s.mu.Unlock()
	if !exists || score <= 0 {
		return 0, 0, false, nil
	}
	sorted := sortRankPairs(pairs, len(pairs), true)
	for i, row := range sorted {
		if row.AccountID == accountID {
			return int64(i + 1), row.PlayCount, true, nil
		}
	}
	return 0, score, true, nil
}

func (s *memoryRankStore) incrWelfareGold(_ context.Context, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.welGoldTotal += delta
	return s.welGoldTotal, nil
}

func (s *memoryRankStore) welfareGoldTotal(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.welGoldTotal, nil
}

func (s *memoryRankStore) incrWelfareToken(_ context.Context, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.welTokenTotal += delta
	return s.welTokenTotal, nil
}

func (s *memoryRankStore) welfareTokenTotal(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.welTokenTotal, nil
}

// --- Redis ---

type redisRankStore struct {
	rdb         *redis.Client
	popKey      string
	actKeyPref  string
	welGoldKey  string
	welTokenKey string
}

func newRedisRankStore(addr, password string, db int) *redisRankStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &redisRankStore{
		rdb:         rdb,
		popKey:      "sr:rank:popularity",
		actKeyPref:  "sr:rank:activity",
		welGoldKey:  "sr:rank:welfare:gold:total",
		welTokenKey: "sr:rank:welfare:token:total",
	}
}

func (s *redisRankStore) incrPopularity(ctx context.Context, gameID string, delta int64) (int64, error) {
	v, err := s.rdb.ZIncrBy(ctx, s.popKey, float64(delta), gameID).Result()
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

func (s *redisRankStore) topPopularity(ctx context.Context, limit int) ([]RankPlayRow, error) {
	return s.topZSet(ctx, s.popKey, limit, false)
}

func (s *redisRankStore) activityKey(weekID string) string {
	return s.actKeyPref + ":" + strings.TrimSpace(weekID)
}

func (s *redisRankStore) incrActivity(ctx context.Context, weekID, accountID string, delta int64) (int64, error) {
	key := s.activityKey(weekID)
	v, err := s.rdb.ZIncrBy(ctx, key, float64(delta), accountID).Result()
	if err != nil {
		return 0, err
	}
	_ = s.rdb.Expire(ctx, key, 35*24*time.Hour).Err()
	return int64(v), nil
}

func (s *redisRankStore) topActivity(ctx context.Context, weekID string, limit int) ([]RankPlayRow, error) {
	return s.topZSet(ctx, s.activityKey(weekID), limit, true)
}

func (s *redisRankStore) activityMemberRank(ctx context.Context, weekID, accountID string) (int64, int64, bool, error) {
	key := s.activityKey(weekID)
	scoreF, err := s.rdb.ZScore(ctx, key, accountID).Result()
	if err == redis.Nil {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	score := int64(scoreF)
	if score <= 0 {
		return 0, 0, false, nil
	}
	rankZero, err := s.rdb.ZRevRank(ctx, key, accountID).Result()
	if err == redis.Nil {
		return 0, score, true, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return rankZero + 1, score, true, nil
}

func (s *redisRankStore) incrWelfareGold(ctx context.Context, delta int64) (int64, error) {
	return s.rdb.IncrBy(ctx, s.welGoldKey, delta).Result()
}

func (s *redisRankStore) welfareGoldTotal(ctx context.Context) (int64, error) {
	n, err := s.rdb.Get(ctx, s.welGoldKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

func (s *redisRankStore) incrWelfareToken(ctx context.Context, delta int64) (int64, error) {
	return s.rdb.IncrBy(ctx, s.welTokenKey, delta).Result()
}

func (s *redisRankStore) welfareTokenTotal(ctx context.Context) (int64, error) {
	n, err := s.rdb.Get(ctx, s.welTokenKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

func (s *redisRankStore) topZSet(ctx context.Context, key string, limit int, forActivity bool) ([]RankPlayRow, error) {
	limit = ClampRankListLimit(limit)
	if limit <= 0 {
		return nil, nil
	}
	zs, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]RankPlayRow, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		row := RankPlayRow{PlayCount: int64(z.Score)}
		if forActivity {
			row.AccountID = member
		} else {
			row.GameID = member
		}
		out = append(out, row)
	}
	return out, nil
}

// RankRedisConfig 来自 release/configs/starcrystal.json；线上可用环境变量覆盖（见 NewRankStoreFromSources）。
type RankRedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewRankStoreFromSources 合并环境变量与 JSON 配置后创建排行存储。
func NewRankStoreFromSources(file RankRedisConfig) rankStore {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		addr = strings.TrimSpace(file.Addr)
	}
	if addr == "" {
		return newMemoryRankStore()
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
	return newRedisRankStore(addr, password, db)
}

func pingRedisRankStore(ctx context.Context, st rankStore) error {
	if r, ok := st.(*redisRankStore); ok {
		return r.rdb.Ping(ctx).Err()
	}
	return nil
}
