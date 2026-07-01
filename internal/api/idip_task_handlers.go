// 福利任务 IDIP：/idip/v1/tasks/* — 详见 doc/IDIP_API.md §3。
// 玩家侧领奖见 doc/PLAYER_TASK_API.md（/api/v1/tasks/*，需 Bearer）。
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"starcrystal/server/internal/service"
)

// registerIdipTaskRoutes 注册任务运营路由（definitions / tier-policy / upsert / user-progress）。
func (s *Server) registerIdipTaskRoutes() {
	s.mux.HandleFunc("GET /idip/v1/tasks/definitions", s.idipMiddleware(s.handleIdipTaskDefinitions))
	s.mux.HandleFunc("POST /idip/v1/tasks/tier-policy", s.idipMiddleware(s.handleIdipTaskTierPolicy))
	s.mux.HandleFunc("POST /idip/v1/tasks/definition/upsert", s.idipMiddleware(s.handleIdipTaskDefinitionUpsert))
	s.mux.HandleFunc("GET /idip/v1/tasks/user-progress", s.idipMiddleware(s.handleIdipTaskUserProgress))
}

// handleIdipTaskDefinitions — GET /idip/v1/tasks/definitions（TSK-A-001）
// 返回全量 catalog、tierPolicy、activeCount（对玩家可见任务数）。
func (s *Server) handleIdipTaskDefinitions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	type row struct {
		TaskID     string  `json:"taskId"`
		Tier       string  `json:"tier"`
		Category   string  `json:"category"`
		Enabled    bool    `json:"enabled"`
		Target     float64 `json:"target"`
		RewardGold float64 `json:"rewardGold"`
		Metric     string  `json:"metric"`
	}
	defs := service.AllTaskDefsForAdmin()
	policy := service.GetTaskTierPolicy()
	rows := make([]row, 0, len(defs))
	for _, d := range defs {
		rows = append(rows, row{
			TaskID: d.TaskID, Tier: string(d.Tier), Category: string(d.Category),
			Enabled: d.Enabled, Target: d.Target, RewardGold: d.RewardGold, Metric: string(d.Metric),
		})
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]any{
			"tierPolicy": policy,
			"tasks":      rows,
			"activeCount": len(service.ListActiveTaskDefs()),
		},
	})
}

// handleIdipTaskTierPolicy — POST /idip/v1/tasks/tier-policy（TSK-S-006）
// Body: p0Enabled, p1Enabled, p2Enabled；进程内热更新，影响 ListActiveTaskDefs。
func (s *Server) handleIdipTaskTierPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var req service.TaskTierPolicy
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	service.SetTaskTierPolicy(req)
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: service.GetTaskTierPolicy()})
}

// handleIdipTaskDefinitionUpsert — POST /idip/v1/tasks/definition/upsert（TSK-A-002）
// Body: taskId + 可选 enabled, rewardGold, target；未知 taskId → 1420。
func (s *Server) handleIdipTaskDefinitionUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var req struct {
		TaskID     string   `json:"taskId"`
		Enabled    *bool    `json:"enabled"`
		RewardGold *float64 `json:"rewardGold"`
		Target     *float64 `json:"target"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	if strings.TrimSpace(req.TaskID) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "taskId required"})
		return
	}
	if !service.UpsertTaskOverride(req.TaskID, req.Enabled, req.RewardGold, req.Target) {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1420, Message: "task not found"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok"})
}

// handleIdipTaskUserProgress — GET /idip/v1/tasks/user-progress?accountId=
// 内部 GetWelfare(accountId)，与玩家 GET /api/v1/tasks/welfare 同构，无需 Bearer。
func (s *Server) handleIdipTaskUserProgress(w http.ResponseWriter, r *http.Request) {
	if s.taskService == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "task service not configured"})
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "accountId required"})
		return
	}
	data, err := s.taskService.GetWelfare(r.Context(), accountID)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}
