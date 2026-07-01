package antifraud

import "errors"

var (
	// ErrRateLimitStart 单账号或单 IP 的 ads/start 频率超限。
	ErrRateLimitStart = errors.New("ad start rate limited")
	// ErrRateLimitComplete 单账号或单 IP 的 ads/complete 频率超限。
	ErrRateLimitComplete = errors.New("ad complete rate limited")
	// ErrSlotNotAllowed 配置了 AD_SLOT_ALLOWLIST 时，slot 不在白名单内。
	ErrSlotNotAllowed = errors.New("ad slot not allowed")
	// ErrTooManyPendingSessions 未核销的 watchId（未过期）超过 AD_MAX_PENDING_WATCHES_ACCOUNT。
	ErrTooManyPendingSessions = errors.New("too many pending ad watch sessions")
)
