package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/service"
	"starcrystal/server/internal/store"
)

func (s *Server) registerIdipEconomyQueryRoutes() {
	s.mux.HandleFunc("GET /idip/v1/welfare/monthly-token-leaderboard", s.idipMiddleware(s.handleIdipMonthlyTokenLeaderboard))
	s.mux.HandleFunc("GET /idip/v1/economy/user-profile", s.idipMiddleware(s.handleIdipEconomyUserProfile))
	s.mux.HandleFunc("GET /idip/v1/gold/day-user", s.idipMiddleware(s.handleIdipGoldDayUser))
}

func (s *Server) playerRepo() store.PlayerRepository {
	if s.authService == nil {
		return nil
	}
	return s.authService.PlayerRepository()
}

func parseIdipPageQuery(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 50
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("pageSize")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func parseYyyymmQuery(r *http.Request, tz string) (string, bool) {
	yyyymm := strings.TrimSpace(r.URL.Query().Get("yyyymm"))
	if yyyymm == "" {
		return service.GoldYYYYMM(service.GoldNow(tz)), true
	}
	if len(yyyymm) != 6 {
		return "", false
	}
	for _, c := range yyyymm {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return yyyymm, true
}

// handleIdipMonthlyTokenLeaderboard — GET /idip/v1/welfare/monthly-token-leaderboard（IDP-011）
func (s *Server) handleIdipMonthlyTokenLeaderboard(w http.ResponseWriter, r *http.Request) {
	repo := s.playerRepo()
	if s.economy == nil || repo == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	tz := s.economy.Config.Gold.LocationName()
	yyyymm, ok := parseYyyymmQuery(r, tz)
	if !ok {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid yyyymm"})
		return
	}
	page, pageSize := parseIdipPageQuery(r)
	offset := (page - 1) * pageSize

	total, err := repo.CountMonthlyTokenLeaderboard(r.Context(), yyyymm)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	rows, err := repo.ListMonthlyTokenLeaderboard(r.Context(), yyyymm, offset, pageSize)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
		return
	}

	var monthPool float64
	var snapshot float64
	if v, okPool, _ := s.economy.GoldRedis.GetMonthTokenPool(r.Context(), yyyymm); okPool {
		monthPool = v
	} else {
		monthPool = s.economy.Config.Welfare.MonthTokenPool
	}
	snapshot, _ = s.economy.GoldRedis.GetMonthServerDelta(r.Context(), yyyymm)
	if snapshot <= 0 && len(rows) > 0 {
		snapshot = rows[0].ServerDeltaSnapshot
	}

	items := make([]map[string]interface{}, 0, len(rows))
	for i, row := range rows {
		items = append(items, map[string]interface{}{
			"rank":                    offset + i + 1,
			"accountId":               row.AccountID,
			"displayName":             row.DisplayName,
			"inviteCode":              row.InviteCode,
			"goldSpent":               row.GoldSpent,
			"tokenDelta":              row.TokenDelta,
			"rate":                    row.Rate,
			"serverDeltaSnapshot":     row.ServerDeltaSnapshot,
			"settledAt":               row.SettledAt.Format(time.RFC3339),
		})
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: map[string]interface{}{
		"yyyymm":                yyyymm,
		"monthTokenPool":        monthPool,
		"serverDeltaSnapshot":   snapshot,
		"totalRanked":           total,
		"page":                  page,
		"pageSize":              pageSize,
		"totalPages":            totalPages,
		"items":                 items,
	}})
}

// handleIdipEconomyUserProfile — GET /idip/v1/economy/user-profile（IDP-012）
func (s *Server) handleIdipEconomyUserProfile(w http.ResponseWriter, r *http.Request) {
	repo := s.playerRepo()
	if s.economy == nil || repo == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	inviteCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("inviteCode")))
	if accountID == "" && inviteCode == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "accountId or inviteCode required"})
		return
	}
	if accountID == "" {
		var err error
		accountID, err = repo.ResolveAccountIDByInviteCode(r.Context(), inviteCode)
		if err != nil || accountID == "" {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "account not found"})
			return
		}
	}

	rec, err := repo.GetByAccountID(r.Context(), accountID)
	if err != nil || rec == nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "account not found"})
		return
	}
	inviteCodeStr, _ := repo.GetInviteCodeByAccountID(r.Context(), accountID)
	inviteCodeStr = strings.TrimSpace(inviteCodeStr)

	settlementMonths := 6
	if v := strings.TrimSpace(r.URL.Query().Get("settlementMonths")); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			settlementMonths = n
		}
	}
	if settlementMonths > 24 {
		settlementMonths = 24
	}
	includeRanks := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeRanks")), "true")

	tz := s.economy.Config.Gold.LocationName()
	now := service.GoldNow(tz)
	yyyymm := service.GoldYYYYMM(now)
	yyyymmdd := service.GoldYYYYMMDD(now)

	bal, _ := repo.GetEconomyBalances(r.Context(), accountID)
	dayUsed, _ := s.economy.GoldRedis.GetDayUsed(r.Context(), yyyymmdd, accountID)
	userMonthDelta, _ := s.economy.GoldRedis.GetUserMonthDelta(r.Context(), yyyymm, accountID)
	monthServerDelta, _ := s.economy.GoldRedis.GetMonthServerDelta(r.Context(), yyyymm)

	dailyCap := s.economy.Config.Gold.DailyProduceCap
	remaining := dailyCap - dayUsed
	if !s.economy.Config.Gold.DailyProduceCapEnabled || dailyCap <= 0 {
		remaining = -1
	} else if remaining < 0 {
		remaining = 0
	}

	logs, _ := repo.ListWelfareExchangeLogByAccount(r.Context(), accountID, settlementMonths)
	monthlySettlements := make([]map[string]interface{}, 0, len(logs))
	var totalTokenAllocated float64
	for _, lg := range logs {
		totalTokenAllocated += lg.TokenDelta
		monthlySettlements = append(monthlySettlements, map[string]interface{}{
			"yyyymm":              lg.Yyyymm,
			"goldSpent":           lg.GoldSpent,
			"tokenDelta":          lg.TokenDelta,
			"rate":                lg.Rate,
			"serverDeltaSnapshot": lg.ServerDeltaSnapshot,
			"settledAt":           lg.SettledAt.Format(time.RFC3339),
		})
	}

	inviteCount, _ := repo.CountDirectInvitees(r.Context(), accountID)

	data := map[string]interface{}{
		"account": map[string]interface{}{
			"accountId":     rec.AccountID,
			"accountType":   rec.AccountType,
			"displayName":   rec.DisplayName,
			"nickname":      rec.Nickname,
			"inviteCode":    inviteCodeStr,
			"invitedUserId": rec.InvitedUserID,
			"status":        rec.Status,
			"banReason":     rec.BanReason,
			"deviceId":      rec.DeviceID,
		},
		"economy": map[string]interface{}{
			"curgold":    bal.CurGold,
			"totalgold":  bal.TotalGold,
			"curtoken":   bal.CurToken,
			"totaltoken": bal.TotalToken,
		},
		"dailyProduce": map[string]interface{}{
			"yyyymmdd":            yyyymmdd,
			"dayUsed":             dayUsed,
			"dailyCap":            dailyCap,
			"dailyCapEnabled":     s.economy.Config.Gold.DailyProduceCapEnabled,
			"dailyCapRemaining":   remaining,
		},
		"monthActivity": map[string]interface{}{
			"yyyymm":                   yyyymm,
			"userMonthGoldDelta":       userMonthDelta,
			"monthServerGoldDeltaTotal": monthServerDelta,
		},
		"monthlySettlements": monthlySettlements,
		"lifetimeSummary": map[string]interface{}{
			"totalGoldSettled":          bal.TotalGold,
			"totalTokenAllocated":       totalTokenAllocated,
			"totalTokenRedeemedToGift": bal.TotalToken,
			"settlementCount":           len(logs),
		},
		"inviteStats": map[string]interface{}{
			"directInviteeCount": inviteCount,
		},
	}

	if includeRanks && s.economy.WelfareRank != nil {
		ranks := map[string]interface{}{}
		for _, b := range []struct {
			key   string
			board string
		}{
			{"welfareGoldCur", service.BoardWelfareGoldCur},
			{"welfareGoldTotal", service.BoardWelfareGoldTotal},
			{"welfareTokenCur", service.BoardWelfareTokenCur},
			{"welfareTokenTotal", service.BoardWelfareTokenTotal},
		} {
			rk, sc, on, _ := s.economy.WelfareRank.MemberRank(r.Context(), b.board, accountID)
			if on {
				ranks[b.key] = map[string]interface{}{"score": sc, "rank": rk}
			}
		}
		data["ranks"] = ranks
	}

	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

// handleIdipGoldDayUser — GET /idip/v1/gold/day-user?accountId=&yyyymmdd=（IDP-007）
func (s *Server) handleIdipGoldDayUser(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "accountId required"})
		return
	}
	tz := s.economy.Config.Gold.LocationName()
	yyyymmdd := strings.TrimSpace(r.URL.Query().Get("yyyymmdd"))
	if yyyymmdd == "" {
		yyyymmdd = service.GoldYYYYMMDD(service.GoldNow(tz))
	}
	dayUsed, err := s.economy.GoldRedis.GetDayUsed(r.Context(), yyyymmdd, accountID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	cap := s.economy.Config.Gold.DailyProduceCap
	remaining := cap - dayUsed
	if !s.economy.Config.Gold.DailyProduceCapEnabled || cap <= 0 {
		remaining = -1
	} else if remaining < 0 {
		remaining = 0
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: map[string]interface{}{
		"accountId":           accountID,
		"yyyymmdd":            yyyymmdd,
		"dayUsed":             dayUsed,
		"dailyCap":            cap,
		"dailyCapEnabled":     s.economy.Config.Gold.DailyProduceCapEnabled,
		"dailyCapRemaining":   remaining,
	}})
}
