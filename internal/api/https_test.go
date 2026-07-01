package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"starcrystal/server/internal/service"
)

func TestRequestIsHTTPS(t *testing.T) {
	t.Run("direct tls", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)
		r.TLS = &tls.ConnectionState{}
		if !RequestIsHTTPS(r) {
			t.Fatal("expected TLS request as HTTPS")
		}
	})
	t.Run("forwarded proto", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		if !RequestIsHTTPS(r) {
			t.Fatal("expected forwarded https")
		}
	})
	t.Run("plain http", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)
		if RequestIsHTTPS(r) {
			t.Fatal("expected plain http rejected")
		}
	})
}

func TestRequireHTTPSMiddleware(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	s.SetRequireHTTPS(true)
	h := s.Handler()
	t.Run("blocks http", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("allows forwarded https", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
