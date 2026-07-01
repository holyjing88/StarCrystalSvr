package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type MonthlyGoldSettlement struct {
	repo    store.PlayerRepository
	redis   GoldRedisStore
	rank    *WelfareRankSync
	gold    config.GoldConfig
	welfare config.WelfareConfig
	tz      string
}

func NewMonthlyGoldSettlement(repo store.PlayerRepository, redis GoldRedisStore, rank *WelfareRankSync, econ config.EconomyConfig) *MonthlyGoldSettlement {
	w := econ.Welfare
	if w.TokenDeltaDecimals <= 0 {
		w.TokenDeltaDecimals = 2
	}
	if strings.TrimSpace(w.TokenDeltaRound) == "" {
		w.TokenDeltaRound = "floor"
	}
	return &MonthlyGoldSettlement{
		repo: repo, redis: redis, rank: rank,
		gold: econ.Gold, welfare: w, tz: econ.Gold.LocationName(),
	}
}

func (m *MonthlyGoldSettlement) RunSettlement(ctx context.Context, yyyymm string, force bool) error {
	now := GoldNow(m.tz)
	if yyyymm == "" {
		yyyymm = GoldYYYYMM(now)
	}
	done, err := m.redis.IsSettlementDone(ctx, yyyymm)
	if err != nil {
		return err
	}
	if done && !force {
		logger.FatalNotice(logger.TopicAuth, "[settlement] skip yyyymm=%s already done", yyyymm)
		return nil
	}

	release, acquired, err := TryAcquireSettlementLock(ctx, m.redis, yyyymm)
	if err != nil {
		return err
	}
	if !acquired {
		logger.FatalNotice(logger.TopicAuth, "[settlement] skip yyyymm=%s lock held by another instance", yyyymm)
		return nil
	}
	defer release()

	snapshot, err := m.redis.GetMonthServerDelta(ctx, yyyymm)
	if err != nil {
		return err
	}
	pool, err := m.resolveMonthTokenPool(ctx, yyyymm)
	if err != nil {
		return err
	}

	logger.FatalNotice(logger.TopicAuth, "[settlement] start yyyymm=%s snapshot=%.4f pool=%.4f", yyyymm, snapshot, pool)
	if snapshot <= 0 {
		logger.FatalNotice(logger.TopicAuth, "[settlement] abort yyyymm=%s snapshot<=0", yyyymm)
		return fmt.Errorf("month server gold delta snapshot <= 0")
	}

	ids, err := m.repo.ListAccountIDsForMonthlySettlement(ctx)
	if err != nil {
		return err
	}
	var okN, skipN, failN int
	for _, accountID := range ids {
		bal, err := m.repo.GetEconomyBalances(ctx, accountID)
		if err != nil {
			failN++
			logger.FatalNotice(logger.TopicAuth, "[settlement] load fail account=%s err=%v", accountID, err)
			continue
		}
		goldSpent := bal.MonthlyGoldSpent()
		if goldSpent <= 0 {
			skipN++
			continue
		}
		rate := goldSpent / snapshot
		tokenDelta := RoundTokenDelta(rate*pool, m.welfare.TokenDeltaRound, m.welfare.TokenDeltaDecimals)
		if tokenDelta < 1 {
			tokenDelta = 0
		}
		after, err := m.repo.ApplyMonthlyGoldSettlement(ctx, accountID, goldSpent, tokenDelta)
		if err != nil {
			failN++
			logger.FatalNotice(logger.TopicAuth, "[settlement] user fail account=%s goldSpent=%.4f err=%v", accountID, goldSpent, err)
			continue
		}
		_ = m.repo.InsertWelfareExchangeLog(ctx, accountID, yyyymm, goldSpent, tokenDelta, rate, snapshot)
		if m.rank != nil {
			ch := WelfareChangedCurGold | WelfareChangedTotalGold | WelfareChangedCurToken | WelfareChangedTotalToken | WelfareChangedInviteFields
			m.rank.Notify(ctx, accountID, ch, after)
		}
		okN++
		logger.FatalNotice(logger.TopicAuth, "[settlement] ok account=%s goldSpent=%.4f tokenDelta=%.4f rate=%.8f", accountID, goldSpent, tokenDelta, rate)
	}

	_ = m.redis.SetMonthServerDelta(ctx, yyyymm, 0)
	_ = m.redis.ClearMonthUserGoldDeltas(ctx, yyyymm)
	_ = m.redis.MarkSettlementDone(ctx, yyyymm)
	logger.FatalNotice(logger.TopicAuth, "[settlement] end yyyymm=%s ok=%d skip=%d fail=%d", yyyymm, okN, skipN, failN)
	return nil
}

func (m *MonthlyGoldSettlement) resolveMonthTokenPool(ctx context.Context, yyyymm string) (float64, error) {
	if v, ok, err := m.redis.GetMonthTokenPool(ctx, yyyymm); err != nil {
		return 0, err
	} else if ok {
		return v, nil
	}
	if v, ok := m.welfare.MonthTokenPoolByMonth[yyyymm]; ok && v > 0 {
		return v, nil
	}
	if m.welfare.MonthTokenPool > 0 {
		return m.welfare.MonthTokenPool, nil
	}
	return 0, fmt.Errorf("month token pool not configured")
}

func (m *MonthlyGoldSettlement) RunMonthRollover(ctx context.Context, now time.Time) {
	yyyymm := GoldYYYYMM(now.In(GoldLocation(m.tz)))
	_ = m.redis.SetMonthServerDelta(ctx, yyyymm, 0)
	_ = m.redis.ClearMonthUserGoldDeltas(ctx, yyyymm)
	logger.Info(logger.TopicAuth, "[settlement] month rollover yyyymm=%s server_delta and user deltas cleared", yyyymm)
}

func (m *MonthlyGoldSettlement) StartSettlementScheduler(ctx context.Context) {
	if m.repo == nil {
		return
	}
	at := strings.TrimSpace(m.welfare.MonthlyExchangeAt)
	if at == "" {
		at = "23:00:00"
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	var lastRunDay, lastRolloverMonth string
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			now := t.In(GoldLocation(m.tz))
			if now.Day() == 1 {
				monthKey := now.Format("2006-01")
				if lastRolloverMonth != monthKey {
					lastRolloverMonth = monthKey
					m.RunMonthRollover(context.Background(), now)
				}
			}
			if !IsPenultimateExchangeDay(now) {
				continue
			}
			dayKey := now.Format("2006-01-02")
			if lastRunDay == dayKey {
				continue
			}
			hm := now.Format("15:04:05")
			if hm < at {
				continue
			}
			lastRunDay = dayKey
			if err := m.RunSettlement(context.Background(), "", false); err != nil {
				logger.Error(logger.TopicAuth, "[settlement] scheduled run failed: %v", err)
			}
		}
	}
}
