package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"starcrystal/server/internal/logger"
)

// RankService 人气榜 / 活跃榜等：Redis（或内存）+ games 配置补全游戏名。
type RankService struct {
	games            *GameService
	store            rankStore
	activityMinSlice int64
	activityMaxSlice int64
}

// RankActivityConfig 活跃榜服务端校验（可由 starcrystal.json 扩展加载，缺省用默认值）。
type RankActivityConfig struct {
	MinSliceSec int64
	MaxSliceSec int64
}

func NewRankService(games *GameService, redisFile RankRedisConfig) *RankService {
	return NewRankServiceWithActivity(games, redisFile, RankActivityConfig{})
}

func NewRankServiceWithActivity(games *GameService, redisFile RankRedisConfig, act RankActivityConfig) *RankService {
	minS := act.MinSliceSec
	if minS <= 0 {
		minS = 10
	}
	maxS := act.MaxSliceSec
	if maxS <= 0 {
		maxS = 600
	}
	if maxS < minS {
		maxS = minS
	}
	return &RankService{
		games:            games,
		store:            NewRankStoreFromSources(redisFile),
		activityMinSlice: minS,
		activityMaxSlice: maxS,
	}
}

// LogStartupConnectivity 启动时探测 Redis（若启用）或记录使用内存后端。
func (s *RankService) LogStartupConnectivity(ctx context.Context) {
	switch rs := s.store.(type) {
	case *redisRankStore:
		addr := ""
		db := 0
		if opts := rs.rdb.Options(); opts != nil {
			addr = strings.TrimSpace(opts.Addr)
			db = opts.DB
		}
		if err := pingRedisRankStore(ctx, rs); err != nil {
			logger.FatalNotice(logger.TopicMain, "redis (rank): PING failed addr=%q db=%d err=%v (check redisAddr / REDIS_ADDR)", addr, db, err)
			return
		}
		logger.FatalNotice(logger.TopicMain, "redis (rank): PING OK addr=%q db=%d (popularity + activity week boards)", addr, db)
	default:
		logger.FatalNotice(logger.TopicMain, "rank: in-memory backend (redisAddr unset in starcrystal.json and REDIS_ADDR empty)")
	}
}

// ReportPopularityOpen 小游戏被打开一次：人气 +1。
func (s *RankService) ReportPopularityOpen(ctx context.Context, gameID string) (int64, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return 0, ErrRankEmptyGameID
	}
	n, err := s.store.incrPopularity(ctx, gameID, 1)
	if err != nil {
		logger.Warn(logger.TopicAPI, "[rank] incr popularity failed gameId=%s err=%v", gameID, err)
		return 0, err
	}
	return n, nil
}

// ReportActivityPlay 累计账号当周有效游玩秒数（活跃周榜，按 accountId 分桶）。
func (s *RankService) ReportActivityPlay(ctx context.Context, accountID string, durationSec int64) (weekID string, activeScore int64, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", 0, ErrRankEmptyAccountID
	}
	if durationSec <= 0 {
		return "", 0, ErrRankInvalidDuration
	}
	if durationSec < s.activityMinSlice {
		weekID = CurrentActivityWeekID(time.Now())
		return weekID, 0, nil
	}
	if durationSec > s.activityMaxSlice {
		durationSec = s.activityMaxSlice
	}
	weekID = CurrentActivityWeekID(time.Now())
	n, err := s.store.incrActivity(ctx, weekID, accountID, durationSec)
	if err != nil {
		logger.Warn(logger.TopicAPI, "[rank] incr activity failed accountId=%s week=%s err=%v", accountID, weekID, err)
		return weekID, 0, err
	}
	return weekID, n, nil
}

// ListPopularity 人气榜（全量累计打开次数）。
func (s *RankService) ListPopularity(ctx context.Context, lang string, limit int) ([]RankListItem, error) {
	limit = ClampRankListLimit(limit)
	rows, err := s.store.topPopularity(ctx, limit)
	if err != nil {
		return nil, err
	}
	return s.rowsToListItems(rows, lang, ""), nil
}

// ListActivity 活跃榜（当周按账号累计有效秒数）；weekID 为空则用当前周。
func (s *RankService) ListActivity(ctx context.Context, weekID string, limit int) (string, []RankPlayRow, error) {
	limit = ClampRankListLimit(limit)
	if strings.TrimSpace(weekID) == "" {
		weekID = CurrentActivityWeekID(time.Now())
	} else {
		weekID = strings.TrimSpace(weekID)
	}
	rows, err := s.store.topActivity(ctx, weekID, limit)
	if err != nil {
		return weekID, nil, err
	}
	return weekID, rows, nil
}

// MemberActivityRank 返回账号在当周活跃榜的 1-based 名次与秒数（rank=0 表示未上榜）。
func (s *RankService) MemberActivityRank(ctx context.Context, weekID, accountID string) (rank int64, score int64, onBoard bool, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, 0, false, nil
	}
	if strings.TrimSpace(weekID) == "" {
		weekID = CurrentActivityWeekID(time.Now())
	} else {
		weekID = strings.TrimSpace(weekID)
	}
	return s.store.activityMemberRank(ctx, weekID, accountID)
}

// ListWelfareGold 金币榜：全平台累计获得的金币（不按 gameId）。
func (s *RankService) ListWelfareGold(ctx context.Context, lang string, _ int) ([]RankListItem, error) {
	return s.welfarePlatformList(ctx, lang, "gold")
}

// ListWelfareToken 兑换榜：全平台累计兑换得到的 Token（不按 gameId）。
func (s *RankService) ListWelfareToken(ctx context.Context, lang string, _ int) ([]RankListItem, error) {
	return s.welfarePlatformList(ctx, lang, "token")
}

func (s *RankService) welfarePlatformList(ctx context.Context, lang, scoreKind string) ([]RankListItem, error) {
	var total int64
	var err error
	if scoreKind == "gold" {
		total, err = s.store.welfareGoldTotal(ctx)
	} else {
		total, err = s.store.welfareTokenTotal(ctx)
	}
	if err != nil {
		return nil, err
	}
	item := RankListItem{Name: pickWelfarePlatformName(lang)}
	if scoreKind == "gold" {
		item.Gold = float64(total)
	} else {
		item.Token = float64(total)
	}
	return []RankListItem{item}, nil
}

// IncrWelfareToken 兑换成功后累加全平台兑换榜。
func (s *RankService) IncrWelfareToken(ctx context.Context, tokenDelta int64) (int64, error) {
	if tokenDelta <= 0 {
		return 0, ErrRankInvalidWelfareDelta
	}
	return s.store.incrWelfareToken(ctx, tokenDelta)
}

// IncrWelfareGold 任意途径服务端发币成功后累加全平台金币榜。
func (s *RankService) IncrWelfareGold(ctx context.Context, goldDelta int64) (int64, error) {
	if goldDelta <= 0 {
		return 0, nil
	}
	return s.store.incrWelfareGold(ctx, goldDelta)
}

func pickWelfarePlatformName(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "english":
		return "All games (total)"
	case "ur", "urdu":
		return "تمام گیمز (کل)"
	default:
		return "全平台累计"
	}
}

func (s *RankService) rowsToListItems(rows []RankPlayRow, lang string, scoreKind string) []RankListItem {
	all, err := s.games.ListGames()
	if err != nil {
		logger.Warn(logger.TopicAPI, "[rank] list games for names failed: %v", err)
		all = nil
	}
	nameByID := make(map[string]GameItem, len(all))
	for _, g := range all {
		id := strings.TrimSpace(g.GameID)
		if id != "" {
			nameByID[id] = g
		}
	}
	out := make([]RankListItem, 0, len(rows))
	for _, r := range rows {
		item := RankListItem{GameID: r.GameID}
		switch scoreKind {
		case "activity":
			item.ActiveScore = r.PlayCount
		case "gold":
			item.Gold = float64(r.PlayCount)
		case "token":
			item.Token = float64(r.PlayCount)
		default:
			item.PlayCount = r.PlayCount
		}
		if g, ok := nameByID[r.GameID]; ok {
			item.Name = pickRankGameName(g, lang)
		}
		if item.Name == "" {
			item.Name = r.GameID
		}
		out = append(out, item)
	}
	return out
}

// RankListItem API 返回项。
type RankListItem struct {
	GameID      string  `json:"gameId,omitempty"`
	AccountID   string  `json:"accountId,omitempty"`
	Name        string  `json:"name"`
	PlayCount   int64   `json:"playCount,omitempty"`
	ActiveScore int64   `json:"activeScore,omitempty"`
	Gold        float64 `json:"gold,omitempty"`
	Token       float64 `json:"token,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// ErrRankEmptyGameID 上报时 gameId 为空。
var ErrRankEmptyGameID = errors.New("empty gameId")

// ErrRankEmptyAccountID 活跃上报时 accountId 为空（须已登录）。
var ErrRankEmptyAccountID = errors.New("empty accountId")

// ErrRankInvalidDuration 活跃上报时长非法。
var ErrRankInvalidDuration = errors.New("invalid durationSec")

// ErrRankUnknownGameID gameId 不在游戏配置中。
var ErrRankUnknownGameID = errors.New("unknown gameId")

// ErrRankInvalidWelfareDelta 福利榜累加增量非法。
var ErrRankInvalidWelfareDelta = errors.New("invalid welfare delta")

func pickRankGameName(g GameItem, lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "english":
		return firstNonEmptyRank(strings.TrimSpace(g.NameEn), strings.TrimSpace(g.Name), strings.TrimSpace(g.NameUr))
	case "ur", "urdu":
		return firstNonEmptyRank(strings.TrimSpace(g.NameUr), strings.TrimSpace(g.NameEn), strings.TrimSpace(g.Name))
	default:
		return firstNonEmptyRank(strings.TrimSpace(g.Name), strings.TrimSpace(g.NameEn), strings.TrimSpace(g.NameUr))
	}
}

func firstNonEmptyRank(a, b, c string) string {
	if a != "" {
		return a
	}
	if b != "" {
		return b
	}
	return c
}
