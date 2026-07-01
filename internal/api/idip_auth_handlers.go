package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/httpx"
	"starcrystal/server/internal/service"
)

func (s *Server) registerIdipAuthRoutes() {
	s.mux.HandleFunc("POST /idip/v1/auth/login", s.idipIPOnlyMiddleware(s.handleIdipAuthLogin))
	s.mux.HandleFunc("POST /idip/v1/auth/logout", s.idipSessionMiddleware(s.handleIdipAuthLogout))
	s.mux.HandleFunc("POST /idip/v1/auth/heartbeat", s.idipSessionMiddleware(s.handleIdipAuthHeartbeat))
}

func (s *Server) idipIPOnlyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !idipClientAllowed(r) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1403, Message: "idip forbidden"})
			return
		}
		next(w, r)
	}
}

func (s *Server) idipSessionMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !idipClientAllowed(r) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1403, Message: "idip forbidden"})
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-IDIP-Session"))
		if token == "" || s.idipSessions == nil {
			s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1401, Message: "idip session required"})
			return
		}
		user, err := s.idipSessions.ValidateSession(r.Context(), token)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1401, Message: "idip session invalid"})
			return
		}
		next(w, withIdipUsername(r, user))
	}
}

func (s *Server) reloadIdipConfig() config.IdipConfig {
	if s.economy != nil {
		if path := strings.TrimSpace(s.economy.ConfigPath); path != "" {
			if cfg, err := config.LoadEconomyConfigFrom(path); err == nil {
				s.economy.Config = cfg
				if s.idipSessions != nil {
					s.idipSessions.UpdateConfig(cfg.Idip)
				}
				return cfg.Idip
			}
		}
		return s.economy.Config.Idip
	}
	cfg, _, ok := config.LoadEconomyConfig()
	if ok && s.idipSessions != nil {
		s.idipSessions.UpdateConfig(cfg.Idip)
		return cfg.Idip
	}
	if ok {
		return cfg.Idip
	}
	return config.IdipConfig{}
}

func (s *Server) handleIdipAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.idipSessions == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: "idip session not configured"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json"})
		return
	}
	_ = s.reloadIdipConfig()
	clientIP := httpx.ClientIP(r)
	res, err := s.idipSessions.Login(r.Context(), req.Username, req.Password, clientIP, service.VerifyIdipOperator)
	if errors.Is(err, service.ErrIdipLoginRateLimit) {
		s.writeJSON(w, http.StatusTooManyRequests, Response{Code: 1429, Message: "login rate limited"})
		return
	}
	if errors.Is(err, service.ErrIdipLoginFailed) {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1401, Message: "invalid credentials"})
		return
	}
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	if res.KickedUsername != "" {
		service.DefaultAuditRecorder.Record(r.Context(), req.Username, "login_kick", "", map[string]any{
			"kickedUsername": res.KickedUsername,
			"newUsername":    req.Username,
		})
	}
	service.DefaultAuditRecorder.Record(r.Context(), req.Username, "login", "", map[string]any{
		"clientIP": clientIP,
	})
	w.Header().Set("X-IDIP-Session", res.SessionToken)
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]any{
			"sessionToken":           res.SessionToken,
			"expiresAt":              res.ExpiresAt.Format(time.RFC3339),
			"heartbeatIntervalSec": res.HeartbeatIntervalSec,
		},
	})
}

func (s *Server) handleIdipAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.Header.Get("X-IDIP-Session"))
	_ = s.idipSessions.Logout(r.Context(), token)
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: map[string]any{}})
}

func (s *Server) handleIdipAuthHeartbeat(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.Header.Get("X-IDIP-Session"))
	exp, err := s.idipSessions.Heartbeat(r.Context(), token)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1401, Message: "idip session invalid"})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]any{"expiresAt": exp.Format(time.RFC3339)},
	})
}
