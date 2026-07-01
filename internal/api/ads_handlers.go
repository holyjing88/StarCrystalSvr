package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/antifraud"
	"starcrystal/server/internal/httpx"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type adsStartReq struct {
	Slot string `json:"slot"`
}

type adsStartData struct {
	WatchID    string `json:"watchId"`
	ExpiresIn  int    `json:"expiresIn"`
	TodayCount int    `json:"todayCount"`
	TotalCount int    `json:"totalCount"`
}

type adsCompleteReq struct {
	WatchID string `json:"watchId"`
	Slot    string `json:"slot"`
}

type adsCompleteData struct {
	TodayCount          int     `json:"todayCount"`
	TotalCount          int     `json:"totalCount"`
	CurGold             float64 `json:"curgold"`
	TotalGold           float64 `json:"totalgold"`
	CurToken            float64 `json:"curtoken"`
	TotalToken          float64 `json:"totaltoken"`
	GrantedGold         float64 `json:"grantedGold,omitempty"`
	DailyCapRemaining   float64 `json:"dailyCapRemaining,omitempty"`
}

func (s *Server) handleAdStart(w http.ResponseWriter, r *http.Request) {
	traceID := "ads-start-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if r.Method != http.MethodPost {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] method not allowed %s", r.Method)
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] missing token")
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] verify failed err=%v", err)
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	var body adsStartReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] bad json err=%v", err)
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	clientIP := httpx.ClientIP(r)
	logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] account_id=%s slot=%q ip=%s", accountID, body.Slot, clientIP)
	watchID, today, total, expSec, err := s.authService.AdsRewardedStart(accountID, body.Slot, clientIP)
	if err != nil {
		msg := err.Error()
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] failed err=%v", err)
		if strings.Contains(msg, "database not configured") {
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: msg})
			return
		}
		switch {
		case errors.Is(err, antifraud.ErrRateLimitStart):
			s.writeJSON(w, http.StatusTooManyRequests, Response{Code: 1423, Message: msg})
			return
		case errors.Is(err, antifraud.ErrSlotNotAllowed):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1429, Message: msg})
			return
		case errors.Is(err, store.ErrAdDailyCapExceeded):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1425, Message: msg})
			return
		case errors.Is(err, antifraud.ErrTooManyPendingSessions):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1427, Message: msg})
			return
		case errors.Is(err, store.ErrAdRewardsDisabledAccount):
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1430, Message: msg})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1420, Message: msg})
		return
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[ads/start] ok watchId=%s today=%d total=%d exp=%d", watchID, today, total, expSec)
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: adsStartData{
			WatchID:    watchID,
			ExpiresIn:  expSec,
			TodayCount: today,
			TotalCount: total,
		},
	})
}

func (s *Server) handleAdComplete(w http.ResponseWriter, r *http.Request) {
	traceID := "ads-complete-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if r.Method != http.MethodPost {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] method not allowed %s", r.Method)
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] missing token")
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] verify failed err=%v", err)
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	var body adsCompleteReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] bad json err=%v", err)
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	if strings.TrimSpace(body.WatchID) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "watchId is required"})
		return
	}
	clientIP := httpx.ClientIP(r)
	logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] account_id=%s watchId=%s slot=%q ip=%s", accountID, body.WatchID, body.Slot, clientIP)
	today, total, cg, tg, cm, tm, granted, dailyRem, err := s.authService.AdsRewardedComplete(accountID, body.WatchID, body.Slot, clientIP)
	if err != nil {
		msg := err.Error()
		logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] failed err=%v", err)
		if strings.Contains(msg, "database not configured") {
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: msg})
			return
		}
		if store.IsNotFound(err) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1421, Message: "invalid or expired watchId"})
			return
		}
		switch {
		case errors.Is(err, antifraud.ErrRateLimitComplete):
			s.writeJSON(w, http.StatusTooManyRequests, Response{Code: 1424, Message: msg})
			return
		case errors.Is(err, store.ErrAdCompletionTooSoon):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1426, Message: msg})
			return
		case errors.Is(err, store.ErrAdDailyCapExceeded):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1425, Message: msg})
			return
		case errors.Is(err, store.ErrAdRewardsDisabledAccount):
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1430, Message: msg})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1422, Message: msg})
		return
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[ads/complete] ok today=%d total=%d curgold=%.2f", today, total, cg)
	if s.taskService != nil {
		_ = s.taskService.OnAdsComplete(r.Context(), accountID)
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: adsCompleteData{
			TodayCount:        today,
			TotalCount:        total,
			CurGold:           cg,
			TotalGold:         tg,
			CurToken:          cm,
			TotalToken:        tm,
			GrantedGold:       granted,
			DailyCapRemaining: dailyRem,
		},
	})
}
