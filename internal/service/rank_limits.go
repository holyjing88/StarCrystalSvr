package service

// 各榜 GET /api/v1/rank 的 limit 上限（人气、活跃及后续 welfare 等统一）。
const RankListMaxLimit = 500

// RankListDefaultLimit 未传 limit 时的默认值。
const RankListDefaultLimit = 50

// ClampRankListLimit 将 limit 限制在 [1, RankListMaxLimit]。
func ClampRankListLimit(limit int) int {
	if limit <= 0 {
		return RankListDefaultLimit
	}
	if limit > RankListMaxLimit {
		return RankListMaxLimit
	}
	return limit
}
