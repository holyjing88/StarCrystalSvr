package store

import "errors"

var (
	// ErrAdCompletionTooSoon 距 start（created_at）过短，疑似脚本秒完。
	ErrAdCompletionTooSoon = errors.New("ad completion too soon after start")
	// ErrAdDailyCapExceeded 已达当日核销上限。
	ErrAdDailyCapExceeded = errors.New("ad daily completion cap exceeded")
	// ErrAdRewardsDisabledAccount 账号因注册风控被标记不发激励奖励。
	ErrAdRewardsDisabledAccount = errors.New("ad rewards disabled for this account")
)
