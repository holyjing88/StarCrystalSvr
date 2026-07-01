package api

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
)

type welfareExchangeResponseData struct {
	GoldSpent  float64 `json:"goldSpent"`
	TokenDelta float64 `json:"tokenDelta"`
	CurGold    float64 `json:"curgold"`
	CurToken   float64 `json:"curtoken"`
	TotalGold  float64 `json:"totalgold"`
	TotalToken float64 `json:"totaltoken"`
	RankToken  int64   `json:"rankToken"`
}

func (s *Server) handleWelfareExchangeDeprecated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	s.writeJSON(w, http.StatusGone, Response{
		Code:    1410,
		Message: "POST /welfare/exchange removed (v7.1): gold→token uses monthly settlement; use POST /welfare/redeem-token-gift for curtoken→gift",
	})
}

func (s *Server) handleWelfareExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))

	tokenDelta, curGold, curToken, totalGold, totalToken, err := s.authService.ExchangeGoldForToken(accountID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "insufficient") || strings.Contains(msg, "no gold") {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: msg})
			return
		}
		if strings.Contains(msg, "database not configured") {
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: msg})
			return
		}
		logger.Error(logger.TopicAPI, "[welfare/exchange] failed account=%s err=%v", accountID, err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "exchange failed"})
		return
	}

	tokenRankDelta := int64(math.Round(tokenDelta))
	if tokenRankDelta <= 0 {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid token delta"})
		return
	}
	rankToken, err := s.rankService.IncrWelfareToken(r.Context(), tokenRankDelta)
	if err != nil {
		if errors.Is(err, service.ErrRankInvalidWelfareDelta) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		logger.Error(logger.TopicAPI, "[welfare/exchange] rank token incr failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "rank update failed"})
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: welfareExchangeResponseData{
			GoldSpent:  tokenDelta,
			TokenDelta: tokenDelta,
			CurGold:    curGold,
			CurToken:   curToken,
			TotalGold:  totalGold,
			TotalToken: totalToken,
			RankToken:  rankToken,
		},
	})
}

func (s *Server) requireBearerAccountID(w http.ResponseWriter, r *http.Request) (string, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return "", false
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return "", false
	}
	return accountID, true
}

func (s *Server) bearerAccountIDOptional(r *http.Request) (string, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		return "", false
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		return "", false
	}
	return accountID, true
}
