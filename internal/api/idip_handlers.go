// Package api — IDIP 运营接口见 server-go/doc/IDIP_API.md。
//
// 本文件：/idip/v1/gold/*、/idip/v1/welfare/*（金币改账、月 Token 池、月末结算）。
// 鉴权：idipMiddleware — 私网 IP +（X-IDIP-Session 或 X-IDIP-Key，见 doc/IDIP_API.md §0.1）。
// 任务类 IDIP 见 idip_task_handlers.go。
package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
)

// registerIdipRoutes 注册金币与福利结算类 IDIP 路由（不含 /tasks）。
func (s *Server) registerIdipRoutes() {
	s.mux.HandleFunc("POST /idip/v1/gold/set-user", s.idipMiddleware(s.handleIdipGoldSetUser))
	s.mux.HandleFunc("GET /idip/v1/gold/month-user", s.idipMiddleware(s.handleIdipGoldMonthUser))
	s.mux.HandleFunc("POST /idip/v1/gold/recalc-server-delta-total", s.idipMiddleware(s.handleIdipRecalcServerDelta))
	s.mux.HandleFunc("POST /idip/v1/gold/recalc-total", s.idipMiddleware(s.handleIdipRecalcServerDelta))
	s.mux.HandleFunc("GET /idip/v1/welfare/month-token-pool", s.idipMiddleware(s.handleIdipGetMonthTokenPool))
	s.mux.HandleFunc("POST /idip/v1/welfare/set-month-token-pool", s.idipMiddleware(s.handleIdipSetMonthTokenPool))
	s.mux.HandleFunc("POST /idip/v1/welfare/run-monthly-settlement", s.idipMiddleware(s.handleIdipRunSettlement))
	s.mux.HandleFunc("GET /idip/v1/invite/platform-contrib", s.idipMiddleware(s.handleIDIPInvitePlatformContrib))
	s.registerIdipAuthRoutes()
	s.registerIdipEconomyQueryRoutes()
	s.registerIdipTaskRoutes()
	s.registerIdipGamesRoutes()
	s.registerIdipPublishRoutes()
}

// idipMiddleware：内网 IP + Session（运营台）或 X-IDIP-Key（Vitest/脚本）。无 economy 的单元测试 Server 仅校验 IP。
func (s *Server) idipMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !idipClientAllowed(r) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1403, Message: "idip forbidden"})
			return
		}
		if s.economy == nil && s.idipSessions == nil {
			next(w, r)
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-IDIP-Session"))
		if token != "" && s.idipSessions != nil {
			user, err := s.idipSessions.ValidateSession(r.Context(), token)
			if err != nil {
				s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1401, Message: "idip session invalid"})
				return
			}
			next(w, withIdipUsername(r, user))
			return
		}
		if s.economy != nil {
			want := strings.TrimSpace(s.economy.Config.Idip.Key)
			if want == "" || strings.TrimSpace(r.Header.Get("X-IDIP-Key")) == want {
				next(w, withIdipUsername(r, "idip-key"))
				return
			}
		}
		s.writeJSON(w, http.StatusForbidden, Response{Code: 1403, Message: "idip auth required"})
	}
}

// idipClientAllowed 拒绝公网 RemoteAddr（IDP-001）。
func idipClientAllowed(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// handleIdipGoldSetUser — POST /idip/v1/gold/set-user（IDP-003）
// Body: accountId, op(add|deduct|set), amount, bizType? → GoldLedger ApplyGold(SkipDailyCap=true)。
func (s *Server) handleIdipGoldSetUser(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil || s.economy.GoldLedger == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var req struct {
		AccountID string  `json:"accountId"`
		Op        string  `json:"op"`
		Amount    float64 `json:"amount"`
		BizType   string  `json:"bizType"`
	}
	_ = json.Unmarshal(body, &req)
	accountID := strings.TrimSpace(req.AccountID)
	if accountID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "accountId required"})
		return
	}
	op := service.GoldOpSet
	switch strings.ToLower(strings.TrimSpace(req.Op)) {
	case "add":
		op = service.GoldOpAdd
	case "deduct":
		op = service.GoldOpDeduct
	case "set", "":
		op = service.GoldOpSet
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid op"})
		return
	}
	res, err := s.economy.GoldLedger.ApplyGold(r.Context(), accountID, op, req.Amount, service.GoldApplyOpts{
		BizType:      req.BizType,
		SkipDailyCap: true,
	})
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: res})
}

// handleIdipGoldMonthUser — GET /idip/v1/gold/month-user?accountId=&yyyymm=（IDP-004）
// v1 仅返回 Redis key 说明；v1.1 规划读取 userGoldDelta，见 doc/IDIP_API.md §1.2。
func (s *Server) handleIdipGoldMonthUser(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	yyyymm := strings.TrimSpace(r.URL.Query().Get("yyyymm"))
	if accountID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "accountId required"})
		return
	}
	if yyyymm == "" {
		yyyymm = service.GoldYYYYMM(service.GoldNow(s.economy.Config.Gold.LocationName()))
	}
	userDelta, err := s.economy.GoldRedis.GetUserMonthDelta(r.Context(), yyyymm, accountID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	userKey := "sr:gold:month:" + yyyymm + ":user:" + accountID + ":gold_delta"
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: map[string]interface{}{
		"yyyymm":        yyyymm,
		"accountId":     accountID,
		"userGoldDelta": userDelta,
		"redisKey":      userKey,
	}})
}

// handleIdipRecalcServerDelta — POST recalc-server-delta-total / recalc-total（IDP-010，未实现 501）。
func (s *Server) handleIdipRecalcServerDelta(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusNotImplemented, Response{Code: 1001, Message: "recalc-server-delta-total not implemented (P1)"})
}

// handleIdipGetMonthTokenPool — GET /idip/v1/welfare/month-token-pool（IDP-005）
func (s *Server) handleIdipGetMonthTokenPool(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	yyyymm := strings.TrimSpace(r.URL.Query().Get("yyyymm"))
	if yyyymm == "" {
		yyyymm = service.GoldYYYYMM(service.GoldNow(s.economy.Config.Gold.LocationName()))
	}
	if v, ok, err := s.economy.GoldRedis.GetMonthTokenPool(r.Context(), yyyymm); err == nil && ok {
		s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: map[string]interface{}{"yyyymm": yyyymm, "monthTokenPool": v}})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: map[string]interface{}{
		"yyyymm": yyyymm, "monthTokenPool": s.economy.Config.Welfare.MonthTokenPool, "from": "json",
	}})
}

// handleIdipSetMonthTokenPool — POST /idip/v1/welfare/set-month-token-pool body pool, yyyymm?（IDP-005）
func (s *Server) handleIdipSetMonthTokenPool(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	var req struct {
		Yyyymm string  `json:"yyyymm"`
		Pool   float64 `json:"pool"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	yyyymm := strings.TrimSpace(req.Yyyymm)
	if yyyymm == "" {
		yyyymm = service.GoldYYYYMM(service.GoldNow(s.economy.Config.Gold.LocationName()))
	}
	if err := s.economy.GoldRedis.SetMonthTokenPool(r.Context(), yyyymm, req.Pool); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success"})
}

// handleIdipRunSettlement — POST /idip/v1/welfare/run-monthly-settlement body yyyymm?, force?（IDP-006）
func (s *Server) handleIdipRunSettlement(w http.ResponseWriter, r *http.Request) {
	if s.economy == nil || s.economy.Settlement == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "economy not configured"})
		return
	}
	var req struct {
		Yyyymm string `json:"yyyymm"`
		Force  bool   `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.economy.Settlement.RunSettlement(r.Context(), strings.TrimSpace(req.Yyyymm), req.Force); err != nil {
		logger.Error(logger.TopicAuth, "[idip] settlement failed: %v", err)
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success"})
}
