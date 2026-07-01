package service

import (
	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type EconomyBundle struct {
	Config       config.EconomyConfig
	ConfigPath   string
	GoldRedis    GoldRedisStore
	GoldLedger   *GoldLedgerService
	WelfareRank  *WelfareRankSync
	Settlement   *MonthlyGoldSettlement
	InviteNotify *InviteNotifyService
}

func NewEconomyBundle(repo store.PlayerRepository, rankRedis RankRedisConfig) *EconomyBundle {
	econ, path, _ := config.LoadEconomyConfig()
	goldRedis := NewGoldRedisStore(rankRedis)
	welfareStore := newWelfareRankStoreFromGold(goldRedis, rankRedis)
	welfareRank := NewWelfareRankSync(welfareStore, repo)
	ledger := NewGoldLedgerService(repo, goldRedis, welfareRank, econ.Gold)
	share := NewInviterShareService(repo, goldRedis, welfareRank, econ.Invite, econ.Gold.LocationName())
	ledger.SetInviteShare(share)
	settlement := NewMonthlyGoldSettlement(repo, goldRedis, welfareRank, econ)
	inviteNotify := NewInviteNotifyService(repo, econ.Invite, econ.Gold.LocationName())
	if path != "" {
		logger.Info(logger.TopicMain, "economy config loaded from %s (dailyCapEnabled=%v cap=%.0f monthPool=%.0f invite=%v gmMode=%s)",
			path, econ.Gold.DailyProduceCapEnabled, econ.Gold.DailyProduceCap, econ.Welfare.MonthTokenPool,
			econ.Invite.Enabled, econ.Invite.GmGrantShareMode)
	} else {
		logger.Warn(logger.TopicMain, "economy config: starcrystal.json gold/welfare not found; using zero defaults")
	}
	return &EconomyBundle{
		Config: econ, ConfigPath: path, GoldRedis: goldRedis,
		GoldLedger: ledger, WelfareRank: welfareRank, Settlement: settlement, InviteNotify: inviteNotify,
	}
}
