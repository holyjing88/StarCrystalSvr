package api

import (
	"net/http"
	"strings"
)

type inviteHeartbeatResponse struct {
	PendingDownlineL1Contrib float64 `json:"pendingDownlineL1Contrib"`
	PendingDownlineL2Contrib float64 `json:"pendingDownlineL2Contrib"`
}

func (s *Server) handleAuthHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	if s.economy == nil || s.economy.InviteNotify == nil || !s.economy.InviteNotify.Enabled() {
		s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: inviteHeartbeatResponse{}})
		return
	}
	pending, err := s.economy.InviteNotify.OnHeartbeat(r.Context(), accountID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "heartbeat failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "success",
		Data: inviteHeartbeatResponse{
			PendingDownlineL1Contrib: pending.PendingL1,
			PendingDownlineL2Contrib: pending.PendingL2,
		},
	})
}

func (s *Server) handleAuthInviteContribAckNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	if s.economy == nil || s.economy.InviteNotify == nil {
		s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success"})
		return
	}
	if err := s.economy.InviteNotify.Ack(r.Context(), accountID); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "ack failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success"})
}

func (s *Server) handleIDIPInvitePlatformContrib(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	yyyymm := strings.TrimSpace(r.URL.Query().Get("yyyymm"))
	if yyyymm == "" {
		yyyymm = strings.TrimSpace(r.URL.Query().Get("month"))
	}
	repo := s.authService.PlayerRepository()
	type row struct {
		Yyyymm         string  `json:"yyyymm"`
		PlatformShare  float64 `json:"platformShare"`
	}
	if yyyymm == "" || repo == nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "yyyymm required"})
		return
	}
	sum, err := repo.SumPlatformShareForMonth(r.Context(), yyyymm)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2003, Message: "query failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: row{Yyyymm: yyyymm, PlatformShare: sum}})
}
