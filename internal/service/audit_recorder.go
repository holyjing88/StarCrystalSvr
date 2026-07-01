package service

import (
	"context"
	"sync"
	"time"
)

// AuditRow 内存审计行（P3 前；供 IDIP /audit/logs 与测试）。
type AuditRow struct {
	CreatedAt time.Time      `json:"createdAt"`
	Username  string         `json:"username"`
	Action    string         `json:"action"`
	GameID    string         `json:"gameId,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

var (
	auditMu   sync.Mutex
	auditRows []AuditRow
)

// AuditRecorder 记录 IDIP 写操作。
type AuditRecorder struct{}

func (AuditRecorder) Record(_ context.Context, username, action, gameID string, detail map[string]any) {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditRows = append(auditRows, AuditRow{
		CreatedAt: time.Now().UTC(),
		Username:  username,
		Action:    action,
		GameID:    gameID,
		Detail:    detail,
	})
	if len(auditRows) > 5000 {
		auditRows = auditRows[len(auditRows)-5000:]
	}
}

var DefaultAuditRecorder AuditRecorder

func ListAuditLogs(limit, offset int, action, gameID string) (total int, list []AuditRow) {
	auditMu.Lock()
	defer auditMu.Unlock()
	filtered := make([]AuditRow, 0, len(auditRows))
	for i := len(auditRows) - 1; i >= 0; i-- {
		r := auditRows[i]
		if action != "" && r.Action != action {
			continue
		}
		if gameID != "" && r.GameID != gameID {
			continue
		}
		filtered = append(filtered, r)
	}
	total = len(filtered)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return total, nil
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return total, filtered[offset:end]
}

func ResetAuditLogsForTests() {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditRows = nil
}
