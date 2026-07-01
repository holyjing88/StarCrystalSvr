// IDIP 游戏列表 / upsert / delete — 见 idip-webclient/doc/H5游戏发布与运营登录设计.md §5。
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"starcrystal/server/internal/service"
)

func (s *Server) registerIdipGamesRoutes() {
	s.mux.HandleFunc("GET /idip/v1/games/list", s.idipMiddleware(s.handleIdipGamesList))
	s.mux.HandleFunc("POST /idip/v1/games/upsert", s.idipMiddleware(s.handleIdipGamesUpsert))
	s.mux.HandleFunc("POST /idip/v1/games/batch-upsert", s.idipMiddleware(s.handleIdipGamesBatchUpsert))
	s.mux.HandleFunc("POST /idip/v1/games/delete", s.idipMiddleware(s.handleIdipGamesDelete))
	s.mux.HandleFunc("POST /idip/v1/games/h5/upload", s.idipMiddleware(s.handleIdipGamesH5Upload))
	s.mux.HandleFunc("GET /idip/v1/audit/logs", s.idipMiddleware(s.handleIdipAuditLogs))
}

type idipGameRow struct {
	GameID           string      `json:"gameId"`
	Name             string      `json:"name"`
	NameEn           string      `json:"nameEn,omitempty"`
	NameUr           string      `json:"nameUr,omitempty"`
	Note             string      `json:"note,omitempty"`
	NoteEn           string      `json:"noteEn,omitempty"`
	NoteUr           string      `json:"noteUr,omitempty"`
	EntryType        string      `json:"entryType"`
	EntryURL         string      `json:"entryUrl"`
	Sort             int         `json:"sort"`
	Status           string      `json:"status,omitempty"`
	MissingVersion   bool        `json:"missingVersion,omitempty"`
	Channels         interface{} `json:"channels,omitempty"`
	IconLink         string      `json:"iconLink,omitempty"`
	CoverURL         string      `json:"coverUrl,omitempty"`
	MinAppVersion    string      `json:"minAppVersion,omitempty"`
	DownloadURL      string      `json:"downloadUrl,omitempty"`
	PackageBytes     int64       `json:"packageBytes,omitempty"`
	DownloadSha256   string      `json:"downloadSha256,omitempty"`
}

func gameItemToIdipRow(g service.GameItem) idipGameRow {
	row := idipGameRow{
		GameID:         g.GameID,
		Name:           g.Name,
		NameEn:         g.NameEn,
		NameUr:         g.NameUr,
		Note:           g.Note,
		NoteEn:         g.NoteEn,
		NoteUr:         g.NoteUr,
		EntryType:      g.EntryType,
		EntryURL:       g.EntryURL,
		Sort:           g.Sort,
		Status:         g.Status,
		IconLink:       g.IconLink,
		CoverURL:       g.CoverURL,
		MinAppVersion:  g.MinAppVersion,
		DownloadURL:    g.DownloadURL,
		PackageBytes:   g.PackageBytes,
		DownloadSha256: g.DownloadSha256,
	}
	if len(g.Channels) > 0 {
		row.Channels = []string(g.Channels)
	}
	if service.EntryURLMissingVersion(g.EntryURL) {
		row.MissingVersion = true
	}
	return row
}

func (s *Server) handleIdipGamesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	cfg, err := service.LoadGamesConfigForIDIP()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	rows := make([]idipGameRow, 0, len(cfg.Items))
	for _, g := range cfg.Items {
		rows = append(rows, gameItemToIdipRow(g))
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]any{
			"list":          rows,
			"configVersion": cfg.ConfigVersion,
			"configPath":    cfg.Path,
		},
	})
}

type idipGameUpsertRequest struct {
	ExpectedConfigVersion string                  `json:"expectedConfigVersion"`
	GameID                string                  `json:"gameId"`
	MinigameVersion        string                  `json:"minigameVersion"`
	Name                  *string                 `json:"name"`
	NameEn                *string                 `json:"nameEn"`
	NameUr                *string                 `json:"nameUr"`
	Note                  *string                 `json:"note"`
	NoteEn                *string                 `json:"noteEn"`
	NoteUr                *string                 `json:"noteUr"`
	Status                *string                 `json:"status"`
	Sort                  *int                    `json:"sort"`
	IconLink              *string                 `json:"iconLink"`
	CoverURL              *string                 `json:"coverUrl"`
	MinAppVersion         *string         `json:"minAppVersion"`
	Channels              json.RawMessage `json:"channels"`
	DownloadURL           *string         `json:"downloadUrl"`
	PackageBytes          *int64                  `json:"packageBytes"`
	DownloadSha256        *string                 `json:"downloadSha256"`
}

func (req idipGameUpsertRequest) toPatch() service.GameUpsertPatch {
	patch := service.GameUpsertPatch{
		GameID:           strings.TrimSpace(req.GameID),
		Name:             req.Name,
		NameEn:           req.NameEn,
		NameUr:           req.NameUr,
		Note:             req.Note,
		NoteEn:           req.NoteEn,
		NoteUr:           req.NoteUr,
		Status:           req.Status,
		Sort:             req.Sort,
		IconLink:         req.IconLink,
		CoverURL:         req.CoverURL,
		MinAppVersion:    req.MinAppVersion,
		DownloadURL:      req.DownloadURL,
		PackageBytes:     req.PackageBytes,
		DownloadSha256:   req.DownloadSha256,
	}
	if len(req.Channels) > 0 {
		var ch service.GameChannelsPatch
		if err := json.Unmarshal(req.Channels, &ch); err == nil {
			patch.Channels = &ch
		}
	}
	return patch
}

func parseIdipGameUpsertRequest(body []byte) (idipGameUpsertRequest, error) {
	var req idipGameUpsertRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return req, err
	}
	return req, nil
}

func idipGamesWriteError(w http.ResponseWriter, s *Server, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrGamesConfigConflict):
		s.writeJSON(w, http.StatusConflict, Response{Code: 1409, Message: "config version conflict"})
	case errors.Is(err, service.ErrGameNotFound):
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "game not found"})
	case errors.Is(err, service.ErrDeleteRequiresOffline):
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "game must be offline before delete"})
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
	}
	return true
}

func (s *Server) handleIdipGamesUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	req, err := parseIdipGameUpsertRequest(body)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	if strings.TrimSpace(req.ExpectedConfigVersion) == "" || strings.TrimSpace(req.GameID) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "expectedConfigVersion and gameId required"})
		return
	}
	newVersion, err := service.UpsertGameItem(req.ExpectedConfigVersion, req.toPatch())
	if idipGamesWriteError(w, s, err) {
		return
	}
	service.DefaultAuditRecorder.Record(r.Context(), idipUsername(r), "games_upsert", req.GameID, map[string]any{
		"configVersion": newVersion,
	})
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]string{"configVersion": newVersion},
	})
}

func (s *Server) handleIdipGamesBatchUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var req struct {
		ExpectedConfigVersion string                  `json:"expectedConfigVersion"`
		Items                 []idipGameUpsertRequest `json:"items"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	if strings.TrimSpace(req.ExpectedConfigVersion) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "expectedConfigVersion required"})
		return
	}
	if len(req.Items) == 0 {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "items required"})
		return
	}
	if len(req.Items) > 50 {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "items limit 50"})
		return
	}
	patches := make([]service.GameUpsertPatch, 0, len(req.Items))
	for _, item := range req.Items {
		patches = append(patches, item.toPatch())
	}
	newVersion, err := service.BatchUpsertGameItems(req.ExpectedConfigVersion, patches)
	if idipGamesWriteError(w, s, err) {
		return
	}
	service.DefaultAuditRecorder.Record(r.Context(), idipUsername(r), "games_upsert", "", map[string]any{
		"batchCount":    len(patches),
		"configVersion": newVersion,
	})
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]string{"configVersion": newVersion},
	})
}

func (s *Server) handleIdipGamesDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var req struct {
		ExpectedConfigVersion string `json:"expectedConfigVersion"`
		GameID                string `json:"gameId"`
		DeleteH5Dir           bool   `json:"deleteH5Dir"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	if strings.TrimSpace(req.ExpectedConfigVersion) == "" || strings.TrimSpace(req.GameID) == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "expectedConfigVersion and gameId required"})
		return
	}
	newVersion, err := service.DeleteGameItem(req.ExpectedConfigVersion, req.GameID, req.DeleteH5Dir)
	if idipGamesWriteError(w, s, err) {
		return
	}
	service.DefaultAuditRecorder.Record(r.Context(), idipUsername(r), "games_delete", req.GameID, map[string]any{
		"deleteH5Dir":   req.DeleteH5Dir,
		"configVersion": newVersion.ConfigVersion,
		"gameDirName":   newVersion.GameDirName,
		"h5DirDeleted":  newVersion.H5DirDeleted,
	})
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: newVersion,
	})
}

func idipUsername(r *http.Request) string {
	if u := idipUserFrom(r); u != "" {
		return u
	}
	return "idip"
}
