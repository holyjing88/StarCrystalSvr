package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
	"starcrystal/server/internal/store"
)

type rankPlayRequest struct {
	Board  string `json:"board"`
	GameID string `json:"gameId"`
}

type rankActivityRequest struct {
	Board       string `json:"board"`
	GameID      string `json:"gameId"`
	DurationSec int64  `json:"durationSec"`
}

func parseRankListLimit(r *http.Request) int {
	limit := service.RankListDefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	return service.ClampRankListLimit(limit)
}

func (s *Server) handleRankPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "read body failed"})
		return
	}
	var req rankPlayRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	board := strings.TrimSpace(strings.ToLower(req.Board))
	if board == "" {
		board = "popularity"
	}
	if board != "popularity" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "unsupported board: " + board})
		return
	}
	n, err := s.rankService.ReportPopularityOpen(r.Context(), req.GameID)
	if err != nil {
		if errors.Is(err, service.ErrRankEmptyGameID) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		logger.Error(logger.TopicAPI, "[rank] play failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "rank update failed"})
		return
	}
	gameID := strings.TrimSpace(req.GameID)
	if accountID, ok := s.bearerAccountIDOptional(r); ok && s.taskService != nil {
		_ = s.taskService.OnRankPlay(r.Context(), accountID)
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: RankPlayResponseData{
			GameID:    gameID,
			PlayCount: n,
		},
	})
}

func (s *Server) handleRankActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "read body failed"})
		return
	}
	var req rankActivityRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	board := strings.TrimSpace(strings.ToLower(req.Board))
	if board == "" {
		board = "activity"
	}
	if board != "activity" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "unsupported board: " + board})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	weekID, score, err := s.rankService.ReportActivityPlay(r.Context(), accountID, req.DurationSec)
	if err != nil {
		if errors.Is(err, service.ErrRankEmptyAccountID) || errors.Is(err, service.ErrRankInvalidDuration) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		logger.Error(logger.TopicAPI, "[rank] activity failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "rank activity update failed"})
		return
	}
	gameID := strings.TrimSpace(req.GameID)
	if s.taskService != nil {
		_ = s.taskService.OnRankActivity(r.Context(), accountID, gameID, req.DurationSec)
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: RankActivityResponseData{
			AccountID:   accountID,
			GameID:      gameID,
			ActiveScore: score,
			WeekID:      weekID,
		},
	})
}

func (s *Server) handleRankList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	board := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("board")))
	if board == "" {
		board = "popularity"
	}
	limit := parseRankListLimit(r)
	lang := NormalizeGameListLang(r.URL.Query().Get("lang"))

	switch board {
	case "popularity":
		items, err := s.rankService.ListPopularity(r.Context(), lang, limit)
		if err != nil {
			logger.Error(logger.TopicAPI, "[rank] list failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "rank list failed"})
			return
		}
		s.writeJSON(w, http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data: RankListResponseData{
				Board: board,
				Items: apiRankItemsFromService(items),
			},
		})
	case "activity":
		period := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("period")))
		if period == "" {
			period = "week"
		}
		if period != "week" {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "unsupported period: " + period})
			return
		}
		weekID := strings.TrimSpace(r.URL.Query().Get("weekId"))
		resWeek, rows, err := s.rankService.ListActivity(r.Context(), weekID, limit)
		if err != nil {
			logger.Error(logger.TopicAPI, "[rank] activity list failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "rank list failed"})
			return
		}
		items, err := s.listActivityRankItems(r.Context(), rows)
		if err != nil {
			logger.Error(logger.TopicAPI, "[rank] activity list names failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "rank list failed"})
			return
		}
		var myRank int64
		var myScore float64
		if viewerID, ok := s.optionalBearerAccountID(r); ok && viewerID != "" {
			rank, score, _, merr := s.rankService.MemberActivityRank(r.Context(), resWeek, viewerID)
			if merr != nil {
				logger.Error(logger.TopicAPI, "[rank] activity my rank failed: %v", merr)
				s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "rank list failed"})
				return
			}
			myRank = rank
			myScore = float64(score)
		}
		s.writeJSON(w, http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data: RankListResponseData{
				Board:   board,
				Period:  period,
				WeekID:  resWeek,
				Items:   items,
				MyRank:  myRank,
				MyScore: myScore,
			},
		})
	case service.BoardWelfareGoldCur, service.BoardWelfareGoldTotal,
		service.BoardWelfareDownContribCur, service.BoardWelfareDownContribTotal,
		service.BoardWelfareUpContribCur, service.BoardWelfareUpContribTotal,
		service.BoardWelfareTokenCur, service.BoardWelfareTokenTotal,
		"welfare_gold", "welfare_token":
		if p := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("period"))); p != "" && p != "all" {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "welfare boards do not support period"})
			return
		}
		normBoard := service.NormalizeWelfareBoard(board)
		viewerID, _ := s.optionalBearerAccountID(r)
		items, myRank, myScore, err := s.listWelfareRankItems(r, normBoard, limit, viewerID)
		if err != nil {
			logger.Error(logger.TopicAPI, "[rank] welfare board=%s list failed: %v", normBoard, err)
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "rank list failed"})
			return
		}
		s.writeJSON(w, http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data: RankListResponseData{
				Board:   normBoard,
				Items:   items,
				MyRank:  myRank,
				MyScore: myScore,
			},
		})
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "unsupported board: " + board})
	}
}

func (s *Server) optionalBearerAccountID(r *http.Request) (string, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		return "", false
	}
	id, err := s.authService.VerifyToken(tok)
	if err != nil {
		return "", false
	}
	return id, true
}

func (s *Server) listWelfareRankItems(r *http.Request, board string, limit int, viewerAccountID string) ([]RankListItem, int64, float64, error) {
	if s.economy == nil || s.economy.WelfareRank == nil {
		// fallback legacy platform totals
		var items []service.RankListItem
		var err error
		if board == service.BoardWelfareGoldCur || board == "welfare_gold" {
			items, err = s.rankService.ListWelfareGold(r.Context(), "", limit)
		} else {
			items, err = s.rankService.ListWelfareToken(r.Context(), "", limit)
		}
		if err != nil {
			return nil, 0, 0, err
		}
		return apiRankItemsFromService(items), 0, 0, nil
	}
	rows, err := s.economy.WelfareRank.ListBoard(r.Context(), board, limit)
	if err != nil {
		return nil, 0, 0, err
	}
	out := make([]RankListItem, 0, len(rows))
	for i, row := range rows {
		var rec *store.AuthUserRecord
		if rrec, e := s.authService.PlayerRepository().GetByAccountID(r.Context(), row.AccountID); e == nil && rrec != nil {
			rec = rrec
		}
		name := store.RankListDisplayName(rec, row.AccountID)
		item := RankListItem{
			AccountID: row.AccountID,
			Name:      name,
			Score:     row.Score,
			Rank:      int64(i + 1),
		}
		switch board {
		case service.BoardWelfareGoldCur, service.BoardWelfareGoldTotal:
			item.Gold = row.Score
		default:
			item.Token = row.Score
		}
		out = append(out, item)
	}
	var myRank int64
	var myScore float64
	if viewerAccountID != "" {
		myRank, myScore, _, err = s.economy.WelfareRank.MemberRank(r.Context(), board, viewerAccountID)
		if err != nil {
			return out, 0, 0, err
		}
	}
	return out, myRank, myScore, nil
}

func (s *Server) listActivityRankItems(ctx context.Context, rows []service.RankPlayRow) ([]RankListItem, error) {
	out := make([]RankListItem, 0, len(rows))
	for i, row := range rows {
		aid := strings.TrimSpace(row.AccountID)
		var rec *store.AuthUserRecord
		if aid != "" && s.authService != nil {
			if rrec, e := s.authService.PlayerRepository().GetByAccountID(ctx, aid); e == nil && rrec != nil {
				rec = rrec
			}
		}
		out = append(out, RankListItem{
			AccountID:   aid,
			Name:        store.RankListDisplayName(rec, aid),
			ActiveScore: row.PlayCount,
			Rank:        int64(i + 1),
		})
	}
	return out, nil
}

func apiRankItemsFromService(items []service.RankListItem) []RankListItem {
	out := make([]RankListItem, 0, len(items))
	for _, it := range items {
		out = append(out, RankListItem{
			GameID:      it.GameID,
			AccountID:   it.AccountID,
			Name:        it.Name,
			PlayCount:   it.PlayCount,
			ActiveScore: it.ActiveScore,
			Gold:        it.Gold,
			Token:       it.Token,
			Score:       it.Score,
		})
	}
	return out
}
