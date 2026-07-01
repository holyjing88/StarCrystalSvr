package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
)

type gameFavoriteRequest struct {
	GameID string `json:"gameId"`
}

func (s *Server) handleListGameFavorites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	if s.gameFavoriteService == nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "favorite service unavailable"})
		return
	}
	ids, err := s.gameFavoriteService.ListGameIDs(r.Context(), accountID)
	if err != nil {
		logger.Error(logger.TopicAPI, "[games/favorites] list failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "list favorites failed"})
		return
	}
	if ids == nil {
		ids = []string{}
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: GameFavoritesListData{GameIDs: ids},
	})
}

func (s *Server) handleAddGameFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	gameID, okBody := s.readGameFavoriteRequest(w, r)
	if !okBody {
		return
	}
	if s.gameFavoriteService == nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "favorite service unavailable"})
		return
	}
	if err := s.gameFavoriteService.Add(r.Context(), accountID, gameID); err != nil {
		if s.mapFavoriteValidation(w, err) {
			return
		}
		logger.Error(logger.TopicAPI, "[games/favorite] add failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "add favorite failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    GameFavoriteToggleData{GameID: gameID, Favorited: true},
	})
}

func (s *Server) handleRemoveGameFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	gameID := strings.TrimSpace(r.URL.Query().Get("gameId"))
	if gameID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "gameId is required"})
		return
	}
	if s.gameFavoriteService == nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "favorite service unavailable"})
		return
	}
	if err := s.gameFavoriteService.Remove(r.Context(), accountID, gameID); err != nil {
		if s.mapFavoriteValidation(w, err) {
			return
		}
		logger.Error(logger.TopicAPI, "[games/favorite] remove failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "remove favorite failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    GameFavoriteToggleData{GameID: gameID, Favorited: false},
	})
}

func (s *Server) readGameFavoriteRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "read body failed"})
		return "", false
	}
	var req gameFavoriteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return "", false
	}
	gameID := strings.TrimSpace(req.GameID)
	if gameID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "gameId is required"})
		return "", false
	}
	return gameID, true
}

func (s *Server) mapFavoriteValidation(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if err == service.ErrGameFavoriteEmptyGameID {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
		return true
	}
	return false
}
