package service

import (
	"context"
	"strings"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type InviterShareService struct {
	repo   store.PlayerRepository
	redis  GoldRedisStore
	rank   *WelfareRankSync
	invite config.InviteConfig
	tz     string
}

func NewInviterShareService(repo store.PlayerRepository, redis GoldRedisStore, rank *WelfareRankSync, invite config.InviteConfig, tz string) *InviterShareService {
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	return &InviterShareService{repo: repo, redis: redis, rank: rank, invite: invite, tz: tz}
}

func (s *InviterShareService) Enabled() bool {
	return s.invite.Enabled
}

func (s *InviterShareService) OnEarn(ctx context.Context, earnerID string, grantedDelta float64, opts GoldApplyOpts) error {
	if !s.invite.Enabled || grantedDelta <= 0 || s.repo == nil {
		return nil
	}
	earnerID = strings.TrimSpace(earnerID)
	isGM := strings.EqualFold(strings.TrimSpace(opts.BizType), "gm")
	calc := calcInviteShare(grantedDelta, s.invite, s.invite.GmGrantShareMode, isGM)
	if calc.DenominatorAdd > 0 && s.redis != nil {
		yyyymm := GoldYYYYMM(GoldNow(s.tz))
		if err := s.redis.IncrMonthServerDelta(ctx, yyyymm, calc.DenominatorAdd); err != nil {
			logger.Warn(logger.TopicAuth, "[invite-share] redis denom err=%v", err)
		}
	}
	directID, _, secondID, _, err := s.repo.GetInviterInfoByAccountID(ctx, earnerID)
	if err != nil {
		return err
	}
	directOK := s.active(ctx, directID)
	secondOK := s.active(ctx, secondID)
	grantKind := "player"
	if isGM {
		grantKind = "gm"
	}
	nickname := ""
	if rec, e := s.repo.GetByAccountID(ctx, earnerID); e == nil && rec != nil {
		nickname = strings.TrimSpace(rec.Nickname)
		if nickname == "" {
			nickname = strings.TrimSpace(rec.DisplayName)
		}
	}
	layers := buildShareLayers(calc, directID, secondID, directOK, secondOK)
	if err := s.repo.ApplyInviteShareEarn(ctx, store.InviteShareEarnParams{
		EarnerAccountID: earnerID, EarnerNickname: nickname, GrantKind: grantKind,
		BaseGold: grantedDelta, BizType: opts.BizType, BizNo: opts.BizNo, Layers: layers,
	}); err != nil {
		return err
	}
	if s.rank != nil {
		affected := []string{earnerID}
		if directOK && strings.TrimSpace(directID) != "" {
			affected = append(affected, directID)
		}
		if secondOK && strings.TrimSpace(secondID) != "" {
			affected = append(affected, secondID)
		}
		s.rank.BatchNotifyWelfareRanks(ctx, affected)
	}
	return nil
}

func (s *InviterShareService) active(ctx context.Context, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	rec, err := s.repo.GetByAccountID(ctx, id)
	return err == nil && rec != nil && rec.Status != 2
}

func buildShareLayers(calc computedShareResult, directID, secondID string, directOK, secondOK bool) []store.InviteShareLayer {
	out := make([]store.InviteShareLayer, 0, 2)
	dBen, dPlat := directID, calc.Direct.Platform
	if calc.Direct.Paid > 0 && !directOK {
		dPlat += calc.Direct.Paid
		dBen = platformBeneficiaryID
	}
	out = append(out, store.InviteShareLayer{Layer: 1, BeneficiaryID: dBen, RawShare: calc.Direct.Raw, PaidShare: calc.Direct.Paid, PlatformShare: dPlat})
	sBen, sPlat := secondID, calc.Second.Platform
	if calc.Second.Paid > 0 && !secondOK {
		sPlat += calc.Second.Paid
		sBen = platformBeneficiaryID
	}
	out = append(out, store.InviteShareLayer{Layer: 2, BeneficiaryID: sBen, RawShare: calc.Second.Raw, PaidShare: calc.Second.Paid, PlatformShare: sPlat})
	return out
}
