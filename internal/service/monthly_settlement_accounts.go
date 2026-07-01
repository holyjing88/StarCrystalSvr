package service

import (
	"context"
	"fmt"
	"strings"

	"starcrystal/server/internal/logger"
)

// RunSettlementForAccounts settles only the given accounts (integration/admin).
// Does not mark month done or clear global redis month snapshot.
func (m *MonthlyGoldSettlement) RunSettlementForAccounts(ctx context.Context, yyyymm string, accountIDs []string) error {
	if m.repo == nil || len(accountIDs) == 0 {
		return nil
	}
	now := GoldNow(m.tz)
	if yyyymm == "" {
		yyyymm = GoldYYYYMM(now)
	}
	snapshot, err := m.redis.GetMonthServerDelta(ctx, yyyymm)
	if err != nil {
		return err
	}
	pool, err := m.resolveMonthTokenPool(ctx, yyyymm)
	if err != nil {
		return err
	}
	if snapshot <= 0 {
		return fmt.Errorf("month server gold delta snapshot <= 0")
	}
	for _, accountID := range accountIDs {
		accountID = strings.TrimSpace(accountID)
		if accountID == "" {
			continue
		}
		bal, err := m.repo.GetEconomyBalances(ctx, accountID)
		if err != nil {
			return fmt.Errorf("load account %s: %w", accountID, err)
		}
		goldSpent := bal.MonthlyGoldSpent()
		if goldSpent <= 0 {
			continue
		}
		rate := goldSpent / snapshot
		tokenDelta := RoundTokenDelta(rate*pool, m.welfare.TokenDeltaRound, m.welfare.TokenDeltaDecimals)
		if tokenDelta < 1 {
			tokenDelta = 0
		}
		after, err := m.repo.ApplyMonthlyGoldSettlement(ctx, accountID, goldSpent, tokenDelta)
		if err != nil {
			return fmt.Errorf("settle account %s: %w", accountID, err)
		}
		if err := m.repo.InsertWelfareExchangeLog(ctx, accountID, yyyymm, goldSpent, tokenDelta, rate, snapshot); err != nil {
			return fmt.Errorf("exchange log account %s: %w", accountID, err)
		}
		if m.rank != nil {
			ch := WelfareChangedCurGold | WelfareChangedTotalGold | WelfareChangedCurToken | WelfareChangedTotalToken | WelfareChangedInviteFields
			m.rank.Notify(ctx, accountID, ch, after)
		}
		logger.Info(logger.TopicAuth, "[settlement] scoped ok account=%s goldSpent=%.4f tokenDelta=%.4f", accountID, goldSpent, tokenDelta)
	}
	return nil
}
