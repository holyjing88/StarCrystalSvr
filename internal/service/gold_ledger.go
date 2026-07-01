package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type GoldOp int

const (
	GoldOpAdd GoldOp = iota
	GoldOpDeduct
	GoldOpSet
)

type GoldApplyOpts struct {
	BizType      string
	BizNo        string
	Reason       string
	SkipDailyCap bool
}

type GoldApplyResult struct {
	Before            store.EconomyBalances
	After             store.EconomyBalances
	RequestedDelta    float64
	GrantedDelta      float64
	DailyCapRemaining float64
}

var ErrGoldDailyCapExceeded = errors.New("gold daily produce cap exceeded")

type GoldLedgerService struct {
	repo  store.PlayerRepository
	redis GoldRedisStore
	rank  *WelfareRankSync
	cfg   config.GoldConfig
	tz    string
	share *InviterShareService
}

func NewGoldLedgerService(repo store.PlayerRepository, redis GoldRedisStore, rank *WelfareRankSync, cfg config.GoldConfig) *GoldLedgerService {
	return &GoldLedgerService{
		repo: repo, redis: redis, rank: rank, cfg: cfg, tz: cfg.LocationName(),
	}
}

func (s *GoldLedgerService) SetInviteShare(share *InviterShareService) {
	s.share = share
}

func (s *GoldLedgerService) applyShareAfterEarn(ctx context.Context, accountID string, grantedDelta float64, opts GoldApplyOpts) {
	if s.share == nil || grantedDelta <= 0 {
		return
	}
	if err := s.share.OnEarn(ctx, accountID, grantedDelta, opts); err != nil {
		logger.Warn(logger.TopicAuth, "[gold] invite share failed account=%s err=%v", accountID, err)
	}
}

func (s *GoldLedgerService) willRunShare(granted float64, opts GoldApplyOpts) bool {
	return granted > 0 && s.share != nil && s.share.Enabled()
}

func (s *GoldLedgerService) ApplyGold(ctx context.Context, accountID string, op GoldOp, amount float64, opts GoldApplyOpts) (GoldApplyResult, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return GoldApplyResult{}, fmt.Errorf("empty account id")
	}
	var res GoldApplyResult
	res.RequestedDelta = amount

	switch op {
	case GoldOpAdd:
		granted, remaining, err := s.calcGrantedAdd(ctx, accountID, amount, opts)
		if err != nil {
			return GoldApplyResult{}, err
		}
		res.GrantedDelta = granted
		res.DailyCapRemaining = remaining
		if granted <= 0 {
			bal, err := s.repo.GetEconomyBalances(ctx, accountID)
			if err != nil {
				return GoldApplyResult{}, err
			}
			res.Before = bal
			res.After = bal
			return res, nil
		}
		before, after, err := s.repo.ApplyCurGoldDelta(ctx, accountID, granted)
		if err != nil {
			return GoldApplyResult{}, err
		}
		res.Before = before
		res.After = after
		if err := s.syncRedisAfterDelta(ctx, accountID, granted); err != nil {
			logger.Warn(logger.TopicAuth, "[gold] redis sync after add failed account=%s err=%v", accountID, err)
		}
		if err := s.incrDayCounters(ctx, accountID, opts.BizType, granted); err != nil {
			logger.Warn(logger.TopicAuth, "[gold] day counter incr failed account=%s err=%v", accountID, err)
		}
		if s.rank != nil && !s.willRunShare(granted, opts) {
			s.rank.Notify(ctx, accountID, WelfareChangedCurGold|WelfareChangedTotalGold, after)
		}
		s.applyShareAfterEarn(ctx, accountID, granted, opts)
		return res, nil

	case GoldOpDeduct:
		if amount <= 0 {
			return GoldApplyResult{}, fmt.Errorf("invalid deduct amount")
		}
		before, after, err := s.repo.ApplyCurGoldDelta(ctx, accountID, -amount)
		if err != nil {
			return GoldApplyResult{}, err
		}
		res.Before = before
		res.After = after
		res.GrantedDelta = -amount
		delta := after.CurGold - before.CurGold
		if err := s.syncRedisAfterDelta(ctx, accountID, delta); err != nil {
			logger.Warn(logger.TopicAuth, "[gold] redis sync after deduct failed: %v", err)
		}
		if s.rank != nil {
			s.rank.Notify(ctx, accountID, WelfareChangedCurGold|WelfareChangedTotalGold, after)
		}
		return res, nil

	case GoldOpSet:
		before, after, err := s.repo.SetCurGold(ctx, accountID, amount)
		if err != nil {
			return GoldApplyResult{}, err
		}
		res.Before = before
		res.After = after
		delta := after.CurGold - before.CurGold
		res.GrantedDelta = delta
		if err := s.syncRedisAfterDelta(ctx, accountID, delta); err != nil {
			logger.Warn(logger.TopicAuth, "[gold] redis sync after set failed: %v", err)
		}
		if delta > 0 {
			_ = s.incrDayCounters(ctx, accountID, opts.BizType, delta)
		}
		if s.rank != nil && !s.willRunShare(delta, opts) {
			s.rank.Notify(ctx, accountID, WelfareChangedCurGold|WelfareChangedTotalGold, after)
		}
		if delta > 0 {
			s.applyShareAfterEarn(ctx, accountID, delta, opts)
		}
		return res, nil

	default:
		return GoldApplyResult{}, fmt.Errorf("unknown gold op")
	}
}

func (s *GoldLedgerService) calcGrantedAdd(ctx context.Context, accountID string, requested float64, opts GoldApplyOpts) (granted, remaining float64, err error) {
	if requested <= 0 {
		return 0, 0, fmt.Errorf("invalid add amount")
	}
	if opts.SkipDailyCap || !s.cfg.DailyProduceCapEnabled {
		return requested, 0, nil
	}
	now := GoldNow(s.tz)
	dayKey := GoldYYYYMMDD(now)
	dayUsed, err := s.redis.GetDayUsed(ctx, dayKey, accountID)
	if err != nil {
		return 0, 0, err
	}
	globalCap := s.cfg.DailyProduceCap
	remainingGlobal := globalCap - dayUsed
	if globalCap > 0 && remainingGlobal < 0 {
		remainingGlobal = 0
	}
	remainingBiz := requested
	if bizCap, ok := s.cfg.DailyProduceCapByBiz[strings.TrimSpace(opts.BizType)]; ok && bizCap > 0 {
		bizUsed, e := s.redis.GetDayBizUsed(ctx, dayKey, accountID, opts.BizType)
		if e != nil {
			return 0, 0, e
		}
		remainingBiz = bizCap - bizUsed
		if remainingBiz < 0 {
			remainingBiz = 0
		}
	}
	granted = requested
	if globalCap > 0 && granted > remainingGlobal {
		granted = remainingGlobal
	}
	if granted > remainingBiz {
		granted = remainingBiz
	}
	remaining = remainingGlobal - granted
	if globalCap > 0 && remaining < 0 {
		remaining = 0
	}
	if granted < requested && !s.cfg.OverflowClamp() {
		return 0, remaining, ErrGoldDailyCapExceeded
	}
	if granted <= 0 && globalCap > 0 {
		if !s.cfg.OverflowClamp() {
			return 0, remaining, ErrGoldDailyCapExceeded
		}
		return 0, remaining, nil
	}
	return granted, remaining, nil
}

func (s *GoldLedgerService) syncRedisAfterDelta(ctx context.Context, accountID string, delta float64) error {
	if delta == 0 {
		return nil
	}
	yyyymm := GoldYYYYMM(GoldNow(s.tz))
	if err := s.redis.IncrMonthServerDelta(ctx, yyyymm, delta); err != nil {
		return err
	}
	return s.redis.IncrUserMonthDelta(ctx, yyyymm, accountID, delta)
}

func (s *GoldLedgerService) incrDayCounters(ctx context.Context, accountID, bizType string, granted float64) error {
	if granted <= 0 {
		return nil
	}
	dayKey := GoldYYYYMMDD(GoldNow(s.tz))
	if err := s.redis.IncrDayUsed(ctx, dayKey, accountID, granted); err != nil {
		return err
	}
	if bizType != "" {
		if _, ok := s.cfg.DailyProduceCapByBiz[bizType]; ok {
			return s.redis.IncrDayBizUsed(ctx, dayKey, accountID, bizType, granted)
		}
	}
	return nil
}
