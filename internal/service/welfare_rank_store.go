package service

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	BoardWelfareGoldCur          = "welfare_gold_cur"
	BoardWelfareGoldTotal        = "welfare_gold_total"
	BoardWelfareDownContribCur   = "welfare_down_contrib_cur"
	BoardWelfareDownContribTotal = "welfare_down_contrib_total"
	BoardWelfareUpContribCur     = "welfare_up_contrib_cur"
	BoardWelfareUpContribTotal   = "welfare_up_contrib_total"
	BoardWelfareTokenCur         = "welfare_token_cur"
	BoardWelfareTokenTotal       = "welfare_token_total"
)

type WelfareRankRow struct {
	AccountID string
	Score     float64
}

type welfareBoardScoreCmd struct {
	board     string
	accountID string
	score     float64
}

type welfareRankStore interface {
	setScore(ctx context.Context, board, accountID string, score float64) error
	batchSetScores(ctx context.Context, cmds []welfareBoardScoreCmd) error
	remove(ctx context.Context, board, accountID string) error
	top(ctx context.Context, board string, limit int) ([]WelfareRankRow, error)
	memberRank(ctx context.Context, board, accountID string) (rank int64, score float64, onBoard bool, err error)
}

func welfareZKey(board string) string {
	switch board {
	case BoardWelfareGoldCur:
		return "sr:rank:welfare:gold:cur"
	case BoardWelfareGoldTotal:
		return "sr:rank:welfare:gold:total"
	case BoardWelfareDownContribCur:
		return "sr:rank:welfare:down:cur"
	case BoardWelfareDownContribTotal:
		return "sr:rank:welfare:down:total"
	case BoardWelfareUpContribCur:
		return "sr:rank:welfare:up:cur"
	case BoardWelfareUpContribTotal:
		return "sr:rank:welfare:up:total"
	case BoardWelfareTokenCur:
		return "sr:rank:welfare:token:cur"
	case BoardWelfareTokenTotal:
		return "sr:rank:welfare:token:total"
	default:
		return "sr:rank:welfare:unknown:" + board
	}
}

func IsWelfareBoard(board string) bool {
	switch strings.TrimSpace(strings.ToLower(board)) {
	case BoardWelfareGoldCur, BoardWelfareGoldTotal,
		BoardWelfareDownContribCur, BoardWelfareDownContribTotal,
		BoardWelfareUpContribCur, BoardWelfareUpContribTotal,
		BoardWelfareTokenCur, BoardWelfareTokenTotal,
		"welfare_gold", "welfare_token":
		return true
	default:
		return false
	}
}

func isWelfareCurBoard(board string) bool {
	switch board {
	case BoardWelfareGoldCur, BoardWelfareTokenCur,
		BoardWelfareDownContribCur, BoardWelfareUpContribCur:
		return true
	default:
		return false
	}
}

func welfareScoreUpdate(board string, score float64) (remove bool, add bool) {
	if score > 0 {
		return false, true
	}
	if isWelfareCurBoard(board) {
		return true, false
	}
	return false, false
}

func NormalizeWelfareBoard(board string) string {
	b := strings.TrimSpace(strings.ToLower(board))
	switch b {
	case "welfare_gold":
		return BoardWelfareGoldCur
	case "welfare_token":
		return BoardWelfareTokenCur
	default:
		return b
	}
}

type memoryWelfareRankStore struct {
	mu     sync.Mutex
	boards map[string]map[string]float64
}

func newMemoryWelfareRankStore() *memoryWelfareRankStore {
	return &memoryWelfareRankStore{boards: make(map[string]map[string]float64)}
}

func (s *memoryWelfareRankStore) setScore(_ context.Context, board, accountID string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.boards[board] == nil {
		s.boards[board] = make(map[string]float64)
	}
	rm, add := welfareScoreUpdate(board, score)
	if add {
		s.boards[board][accountID] = score
	} else if rm {
		delete(s.boards[board], accountID)
	}
	return nil
}

func (s *memoryWelfareRankStore) batchSetScores(_ context.Context, cmds []welfareBoardScoreCmd) error {
	for _, c := range cmds {
		if err := s.setScore(context.Background(), c.board, c.accountID, c.score); err != nil {
			return err
		}
	}
	return nil
}

func (s *memoryWelfareRankStore) remove(_ context.Context, board, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.boards[board]; m != nil {
		delete(m, accountID)
	}
	return nil
}

func (s *memoryWelfareRankStore) top(_ context.Context, board string, limit int) ([]WelfareRankRow, error) {
	s.mu.Lock()
	m := s.boards[board]
	pairs := make([]WelfareRankRow, 0, len(m))
	for id, sc := range m {
		if sc > 0 {
			pairs = append(pairs, WelfareRankRow{AccountID: id, Score: sc})
		}
	}
	s.mu.Unlock()
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Score != pairs[j].Score {
			return pairs[i].Score > pairs[j].Score
		}
		return pairs[i].AccountID < pairs[j].AccountID
	})
	limit = ClampRankListLimit(limit)
	if limit > len(pairs) {
		limit = len(pairs)
	}
	return pairs[:limit], nil
}

func (s *memoryWelfareRankStore) memberRank(_ context.Context, board, accountID string) (int64, float64, bool, error) {
	rows, err := s.top(context.Background(), board, ClampRankListLimit(500))
	if err != nil {
		return 0, 0, false, err
	}
	for i, r := range rows {
		if r.AccountID == accountID {
			return int64(i + 1), r.Score, true, nil
		}
	}
	return 0, 0, false, nil
}

type redisWelfareRankStore struct {
	rdb *redis.Client
}

func newRedisWelfareRankStore(rdb *redis.Client) *redisWelfareRankStore {
	return &redisWelfareRankStore{rdb: rdb}
}

func (s *redisWelfareRankStore) setScore(ctx context.Context, board, accountID string, score float64) error {
	key := welfareZKey(board)
	rm, add := welfareScoreUpdate(board, score)
	if add {
		return s.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: accountID}).Err()
	}
	if rm {
		return s.rdb.ZRem(ctx, key, accountID).Err()
	}
	return nil
}

func (s *redisWelfareRankStore) batchSetScores(ctx context.Context, cmds []welfareBoardScoreCmd) error {
	if len(cmds) == 0 {
		return nil
	}
	pipe := s.rdb.Pipeline()
	for _, c := range cmds {
		key := welfareZKey(c.board)
		rm, add := welfareScoreUpdate(c.board, c.score)
		if add {
			pipe.ZAdd(ctx, key, redis.Z{Score: c.score, Member: c.accountID})
		} else if rm {
			pipe.ZRem(ctx, key, c.accountID)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisWelfareRankStore) remove(ctx context.Context, board, accountID string) error {
	return s.rdb.ZRem(ctx, welfareZKey(board), accountID).Err()
}

func (s *redisWelfareRankStore) top(ctx context.Context, board string, limit int) ([]WelfareRankRow, error) {
	limit = ClampRankListLimit(limit)
	if limit <= 0 {
		return nil, nil
	}
	zs, err := s.rdb.ZRevRangeWithScores(ctx, welfareZKey(board), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]WelfareRankRow, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		out = append(out, WelfareRankRow{AccountID: member, Score: z.Score})
	}
	return out, nil
}

func (s *redisWelfareRankStore) memberRank(ctx context.Context, board, accountID string) (int64, float64, bool, error) {
	key := welfareZKey(board)
	score, err := s.rdb.ZScore(ctx, key, accountID).Result()
	if err == redis.Nil {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	idx, err := s.rdb.ZRevRank(ctx, key, accountID).Result()
	if err == redis.Nil {
		return 0, score, true, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return idx + 1, score, true, nil
}

func newWelfareRankStoreFromGold(gold GoldRedisStore, rankRedis RankRedisConfig) welfareRankStore {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		addr = strings.TrimSpace(rankRedis.Addr)
	}
	if addr == "" {
		return newMemoryWelfareRankStore()
	}
	if rg, ok := gold.(*redisGoldStore); ok {
		return newRedisWelfareRankStore(rg.rdb)
	}
	password := rankRedis.Password
	if _, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		password = os.Getenv("REDIS_PASSWORD")
	}
	db := rankRedis.DB
	if d := strings.TrimSpace(os.Getenv("REDIS_DB")); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 0 && v < 16 {
			db = v
		}
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})
	return newRedisWelfareRankStore(rdb)
}
