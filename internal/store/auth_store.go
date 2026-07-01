// Package store 承载「玩家等业务数据」的 Repository 契约与具体实现。
// SQL/Scylla 等方言只出现在 *MySQLPlayerRepository（或未来的 ScyllaPlayerRepository）中；
// service 层只依赖 PlayerRepository 接口，不得手写 SQL。
package store

import (
	"context"
	"time"
)

// PlayerRepository 定义玩家账号、积分、激励广告会话、注册频控等持久化操作。
// 更换存储引擎时只需提供新实现（例如基于 gocql 的 ScyllaPlayerRepository），并注入 service。
type PlayerRepository interface {
	GetByAccountID(ctx context.Context, accountID string) (*AuthUserRecord, error)
	CreateByAccountID(ctx context.Context, rec *AuthUserRecord) error
	FindLatestAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error)
	FindLatestActiveAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error)
	// FindOldestActiveGuestAccountByDeviceID 同一设备上按首次绑定时间取最早的仍为游客(account_type=guest)的活跃账号；用于游客注册回填，避免邮件注册后「最新活跃」落到邮箱导致重复建新游客。
	FindOldestActiveGuestAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error)
	CountActiveAccountsByDeviceID(ctx context.Context, deviceID string) (int, error)
	ResolveAccountIDByInviteCode(ctx context.Context, inviteCode string) (string, error)
	AddInviteMember(ctx context.Context, inviterAccountID string, inviteeAccountID string) error
	ListInviteMembersByAccountID(ctx context.Context, accountID string) ([]InviteMemberRecord, error)
	GetInviterInfoByAccountID(ctx context.Context, accountID string) (directAccountID string, directNickname string, secondAccountID string, secondNickname string, err error)
	// GetInviteCodeByAccountID 仅 SELECT；无记录返回空串、err=nil。
	GetInviteCodeByAccountID(ctx context.Context, accountID string) (string, error)
	// AllocateInviteCodeOnRegister 仅注册成功路径调用：INSERT 唯一邀请码，冲突重试。
	AllocateInviteCodeOnRegister(ctx context.Context, accountID string) (string, error)
	// DeleteAuthAccountCascade 按 account_id 物理删除账号行：先删 auth_invite_members 中作为邀请人的行，再删 auth_accounts；
	// 其余子表由 FK ON DELETE CASCADE 清理。用于注册回滚或玩家自助销号。
	DeleteAuthAccountCascade(ctx context.Context, accountID string) error
	SetDeviceSilentUntilByAccountID(ctx context.Context, accountID string, silentUntil time.Time) error
	ClearDeviceSilentByAccountID(ctx context.Context, accountID string) error
	BanAccountByAccountID(ctx context.Context, accountID string, reason string) error
	UpdatePasswordByAccountID(ctx context.Context, accountID string, passwordHash string) error
	UpdateGuestVerifiedContactByAccountID(ctx context.Context, accountID, accountType, accountValue, email, phone, provider string) error
	UpdateDisplayNameByAccountID(ctx context.Context, accountID string, displayName string) error
	UpdateAccountMetricsByAccountID(ctx context.Context, accountID string, curGold, totalGold, curToken, totalToken, curDirectShare, totalDirectShare, curSecondShare, totalSecondShare float64) error

	// ExchangeGoldForToken 已废弃（v7.1）；保留供迁移期调用，新逻辑勿用。
	ExchangeGoldForToken(ctx context.Context, accountID string) (tokenDelta, curGold, curToken, totalGold, totalToken float64, err error)

	GetEconomyBalances(ctx context.Context, accountID string) (EconomyBalances, error)
	ApplyCurGoldDelta(ctx context.Context, accountID string, delta float64) (before, after EconomyBalances, err error)
	SetCurGold(ctx context.Context, accountID string, target float64) (before, after EconomyBalances, err error)
	ApplyCurTokenDelta(ctx context.Context, accountID string, delta float64) (before, after EconomyBalances, err error)
	RedeemTokenForGift(ctx context.Context, accountID string) (redeemAmount float64, after EconomyBalances, err error)
	ApplyMonthlyGoldSettlement(ctx context.Context, accountID string, goldSpent, tokenDelta float64) (after EconomyBalances, err error)
	ApplyInviteShareEarn(ctx context.Context, p InviteShareEarnParams) error
	AckInviteNotifyWatermarks(ctx context.Context, accountID string) error
	EnsureNotifyPendingSince(ctx context.Context, accountID string) error
	SumPlatformShareForMonth(ctx context.Context, yyyymm string) (float64, error)
	ListAccountIDsForMonthlySettlement(ctx context.Context) ([]string, error)
	InsertWelfareExchangeLog(ctx context.Context, accountID, yyyymm string, goldSpent, tokenDelta, rate, snapshot float64) error

	// IDIP economy queries (v1.2).
	CountMonthlyTokenLeaderboard(ctx context.Context, yyyymm string) (int, error)
	ListMonthlyTokenLeaderboard(ctx context.Context, yyyymm string, offset, limit int) ([]MonthlyTokenLeaderboardRow, error)
	ListWelfareExchangeLogByAccount(ctx context.Context, accountID string, limit int) ([]WelfareExchangeLogRow, error)
	CountDirectInvitees(ctx context.Context, accountID string) (int, error)

	// Ad watch：一次 start 发卡，complete 核销并奖励（服务端权威）。
	CreateAdWatchSession(ctx context.Context, accountID, slot string, ttl time.Duration) (watchID string, todayCount int, totalCount int, err error)
	CountAdCompletionsToday(ctx context.Context, accountID string) (n int, err error)
	CountPendingAdWatchSessions(ctx context.Context, accountID string) (n int, err error)
	CompleteAdWatchAndGrant(ctx context.Context, accountID, watchID, slot string, rewardGold, rewardToken float64, minWatchSec int, dailyCompletionCap int) (todayCount int, totalCount int, curGold, totalGold, curToken, totalToken float64, err error)

	// 注册同日频控统计（NAT/二手机在 regpolicy + 环境变量中调参）。
	CountAccountsRegisteredTodayByDeviceID(ctx context.Context, deviceID string) (n int, err error)
	CountAccountsRegisteredTodayByRegistrationIP(ctx context.Context, registrationIP string) (n int, err error)
	UpsertDeviceAccountMap(ctx context.Context, deviceID, accountID string) error

	// Ping 连通性探测（含连接池恢复）；健康监视与中间件可用。
	Ping(ctx context.Context) error
}

type AuthUserRecord struct {
	UserID                    string
	AccountID                 string
	AccountType               string
	AccountValue              string
	Email                     string
	Phone                     string
	PasswordHash              string
	DisplayName               string
	Nickname                  string
	Provider                  string
	InvitedUserID             string
	CurGold                   float64
	TotalGold                 float64
	CurToken                  float64
	TotalToken                float64
	CurDirectInviterShare     float64
	TotalDirectInviterShare   float64
	CurSecondInviterShare     float64
	TotalSecondInviterShare   float64
	CurDownlineL1Contrib      float64
	TotalDownlineL1Contrib    float64
	CurDownlineL2Contrib      float64
	TotalDownlineL2Contrib    float64
	NotifyWatermarkDownlineL1 float64
	NotifyWatermarkDownlineL2 float64
	NotifyPendingSince        *time.Time
	DeviceID                  string
	RegistrationIP            string
	AdRewardsDisabled         int
	Status                    int
	BanReason                 string
	DeviceSilentUntil         *time.Time
}

type InviteMemberRecord struct {
	AccountID               string
	Nickname                string
	DisplayName             string
	Email                   string
	Phone                   string
	Provider                string
	CreatedAt               string
	CurGold                 float64
	TotalGold               float64
	CurToken                float64
	TotalToken              float64
	CurDirectInviterShare   float64
	TotalDirectInviterShare float64
	CurSecondInviterShare   float64
	TotalSecondInviterShare float64
}
