// 玩家福利任务 API：/api/v1/tasks/* — 详见 doc/PLAYER_TASK_API.md。
// 须 Bearer；发币仅能通过 POST /tasks/claim。运营查进度用 IDIP user-progress。
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
)

// taskClaimRequest POST /api/v1/tasks/claim
type taskClaimRequest struct {
	TaskID  string `json:"taskId"`
	AdBonus bool   `json:"adBonus"`
}

// handleTasksWelfare — GET /api/v1/tasks/welfare?lang=
// 返回 WelfareTasksData（任务 status、signin7d、tierPolicy）；进度由 rank/activity、ads、邀请等写入。
func (s *Server) handleTasksWelfare(w http.ResponseWriter, r *http.Request) {	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	if s.taskService == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "task service not configured"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	data, err := s.taskService.GetWelfare(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrTaskEmptyAccount) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		logger.Error(logger.TopicAPI, "[tasks] welfare failed: %v", err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "tasks welfare failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

// handleTasksClaim — POST /api/v1/tasks/claim
// 成功发币走 GoldLedger；1421 不可领 · 1422 已领 · 1425 adBonus 无广告凭证（TSK-S-001～007）。
func (s *Server) handleTasksClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	if s.taskService == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "task service not configured"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	var body taskClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	if strings.TrimSpace(body.TaskID) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "taskId is required"})
		return
	}
	res, err := s.taskService.Claim(r.Context(), accountID, body.TaskID, body.AdBonus)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1420, Message: err.Error()})
		case errors.Is(err, service.ErrTaskNotClaimable):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1421, Message: err.Error()})
		case errors.Is(err, service.ErrTaskAlreadyClaimed):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1422, Message: err.Error()})
		case errors.Is(err, service.ErrTaskAdProofInvalid):
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1425, Message: err.Error()})
		case errors.Is(err, service.ErrTaskNoEconomy):
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: err.Error()})
		default:
			if errors.Is(err, service.ErrGoldDailyCapExceeded) {
				s.writeJSON(w, http.StatusBadRequest, Response{Code: 1424, Message: err.Error()})
				return
			}
			logger.Error(logger.TopicAPI, "[tasks] claim failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "task claim failed"})
		}
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success", Data: res})
}

// taskReportRequest POST /api/v1/tasks/report
type taskReportRequest struct {
	Event string `json:"event"`
	Page  string `json:"page"`
}

// handleTasksReport — POST /api/v1/tasks/report
// event: page_view（需 page=welfare）| share_success
func (s *Server) handleTasksReport(w http.ResponseWriter, r *http.Request) {	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	if s.taskService == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "task service not configured"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	var body taskReportRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	switch strings.TrimSpace(strings.ToLower(body.Event)) {
	case "page_view":
		if err := s.taskService.ReportPageView(r.Context(), accountID, body.Page); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "task report failed"})
			return
		}
	case "share_success":
		if err := s.taskService.ReportShare(r.Context(), accountID); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "task report failed"})
			return
		}
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "unsupported event"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "success"})
}
