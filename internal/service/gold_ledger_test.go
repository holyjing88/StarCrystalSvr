package service

import (
	"context"
	"testing"

	"starcrystal/server/internal/config"
)

type stubGoldRepo struct {
	bal float64
}

func (s *stubGoldRepo) GetEconomyBalances(ctx context.Context, accountID string) (b storeEconomyBalances, err error) {
	return storeEconomyBalances{CurGold: s.bal}, nil
}

func (s *stubGoldRepo) ApplyCurGoldDelta(ctx context.Context, accountID string, delta float64) (before, after storeEconomyBalances, err error) {
	before = storeEconomyBalances{CurGold: s.bal}
	s.bal += delta
	return before, storeEconomyBalances{CurGold: s.bal}, nil
}

func (s *stubGoldRepo) SetCurGold(ctx context.Context, accountID string, target float64) (before, after storeEconomyBalances, err error) {
	before = storeEconomyBalances{CurGold: s.bal}
	s.bal = target
	return before, storeEconomyBalances{CurGold: s.bal}, nil
}

// minimal aliases to avoid importing store in test — use store.EconomyBalances via embed
type storeEconomyBalances struct {
	CurGold    float64
	TotalGold  float64
	CurToken   float64
	TotalToken float64
}

// Implement only methods used by test via adapter — GoldLedger uses store.PlayerRepository.
// For brevity skip full stub; test daily cap math only.

// GLD-001: 按 bizType 日上限 clamp。
func TestCalcGrantedAddClamp(t *testing.T) {
	redis := newMemoryGoldRedisStore()
	rank := NewWelfareRankSync(newMemoryWelfareRankStore(), nil)
	cfg := config.GoldConfig{
		DailyProduceCapEnabled:  true,
		DailyProduceCap:         100,
		DailyProduceCapOverflow: "clamp",
		DailyProduceCapByBiz:    map[string]float64{"ad": 30},
	}
	_ = rank
	ledger := &GoldLedgerService{redis: redis, cfg: cfg, tz: cfg.LocationName()}
	ctx := context.Background()
	accountID := "acc1"
	granted, _, err := ledger.calcGrantedAdd(ctx, accountID, 50, GoldApplyOpts{BizType: "ad"})
	if err != nil || granted != 30 {
		t.Fatalf("first ad grant want 30 got %.2f err=%v", granted, err)
	}
	_ = redis.IncrDayUsed(ctx, GoldYYYYMMDD(GoldNow(cfg.LocationName())), accountID, 30)
	_ = redis.IncrDayBizUsed(ctx, GoldYYYYMMDD(GoldNow(cfg.LocationName())), accountID, "ad", 30)
	granted2, _, err := ledger.calcGrantedAdd(ctx, accountID, 50, GoldApplyOpts{BizType: "ad"})
	if err != nil || granted2 != 0 {
		t.Fatalf("biz cap exhausted want 0 got %.2f", granted2)
	}
}

// GLD-002: 全局日产出上限 clamp。
func TestCalcGrantedAdd_GlobalCapClamp(t *testing.T) {
	redis := newMemoryGoldRedisStore()
	cfg := config.GoldConfig{
		DailyProduceCapEnabled:  true,
		DailyProduceCap:         100,
		DailyProduceCapOverflow: "clamp",
	}
	ledger := &GoldLedgerService{redis: redis, cfg: cfg, tz: cfg.LocationName()}
	ctx := context.Background()
	accountID := "acc_cap"
	granted, _, err := ledger.calcGrantedAdd(ctx, accountID, 60, GoldApplyOpts{BizType: "task"})
	if err != nil || granted != 60 {
		t.Fatalf("first grant want 60 got %.2f err=%v", granted, err)
	}
	day := GoldYYYYMMDD(GoldNow(cfg.LocationName()))
	_ = redis.IncrDayUsed(ctx, day, accountID, 60)
	granted2, rem, err := ledger.calcGrantedAdd(ctx, accountID, 50, GoldApplyOpts{BizType: "task"})
	if err != nil || granted2 != 40 {
		t.Fatalf("second grant want 40 got %.2f", granted2)
	}
	if rem != 0 {
		t.Fatalf("remaining want 0 got %.2f", rem)
	}
	_ = redis.IncrDayUsed(ctx, day, accountID, 40)
	granted3, _, err := ledger.calcGrantedAdd(ctx, accountID, 1, GoldApplyOpts{BizType: "task"})
	if err != nil || granted3 != 0 {
		t.Fatalf("exhausted grant want 0 got %.2f", granted3)
	}
}

// GLD-003: 福利 token 折算 floor 边界。
func TestRoundTokenDelta_FloorEdges(t *testing.T) {
	w := config.WelfareConfig{TokenDeltaRound: "floor", TokenDeltaDecimals: 2}
	if got := RoundTokenDelta(0.999, w.TokenDeltaRound, w.TokenDeltaDecimals); got != 0.99 {
		t.Fatalf("floor 0.999 want 0.99 got %.2f", got)
	}
	if got := RoundTokenDelta(0.009, w.TokenDeltaRound, w.TokenDeltaDecimals); got != 0 {
		t.Fatalf("sub-cent floor want 0 got %.2f", got)
	}
}
