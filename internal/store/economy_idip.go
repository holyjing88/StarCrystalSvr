package store

import (
	"context"
	"time"
)

// MonthlyTokenLeaderboardRow one ranked monthly settlement entry (IDIP leaderboard).
type MonthlyTokenLeaderboardRow struct {
	AccountID           string
	DisplayName         string
	InviteCode          string
	GoldSpent           float64
	TokenDelta          float64
	Rate                float64
	ServerDeltaSnapshot float64
	SettledAt           time.Time
}

// WelfareExchangeLogRow one welfare_exchange_log row for account history.
type WelfareExchangeLogRow struct {
	Yyyymm              string
	GoldSpent           float64
	TokenDelta          float64
	Rate                float64
	ServerDeltaSnapshot float64
	SettledAt           time.Time
}

// WelfareExchangeRepository monthly token leaderboard + per-account settlement history.
type WelfareExchangeRepository interface {
	CountMonthlyTokenLeaderboard(ctx context.Context, yyyymm string) (int, error)
	ListMonthlyTokenLeaderboard(ctx context.Context, yyyymm string, offset, limit int) ([]MonthlyTokenLeaderboardRow, error)
	ListWelfareExchangeLogByAccount(ctx context.Context, accountID string, limit int) ([]WelfareExchangeLogRow, error)
	CountDirectInvitees(ctx context.Context, accountID string) (int, error)
}
