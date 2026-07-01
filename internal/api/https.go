package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RequestIsHTTPS reports whether the client request should be treated as HTTPS
// (direct TLS, X-Forwarded-Proto, or URL scheme).
func RequestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		first, _, _ := strings.Cut(proto, ",")
		return strings.EqualFold(strings.TrimSpace(first), "https")
	}
	return strings.EqualFold(r.URL.Scheme, "https")
}

func (s *Server) requireHTTPSMiddleware(next http.Handler) http.Handler {
	if !s.requireHTTPS {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !RequestIsHTTPS(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(Response{
				Code:    1403,
				Message: "HTTPS required",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
