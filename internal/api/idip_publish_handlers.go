// IDIP 发布运维：rsync 重试等（见 idip-webclient/doc/H5游戏发布与运营登录设计.md §6.4）。
package api

import (
	"net/http"

	"starcrystal/server/internal/service"
)

func (s *Server) registerIdipPublishRoutes() {
	s.mux.HandleFunc("POST /idip/v1/publish/rsync-retry", s.idipMiddleware(s.handleIdipPublishRsyncRetry))
}

func (s *Server) handleIdipPublishRsyncRetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	p := service.LoadPublishConfig()
	if !p.Rsync.Enabled {
		s.writeJSON(w, http.StatusOK, Response{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"skipped": true,
				"reason":  "rsync disabled",
			},
		})
		return
	}
	if err := service.SyncH5AssetsToCDN(); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 1500, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: map[string]any{
			"synced": true,
		},
	})
}
