package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdipPublishRsyncRetry_DisabledNoop(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}
	s.registerIdipPublishRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/publish/rsync-retry", idipTestHeaders(s), `{}`)
	if st != http.StatusOK {
		t.Fatalf("status=%d env=%v", st, env)
	}
	if env.Code != 0 {
		t.Fatalf("code=%d msg=%s", env.Code, env.Message)
	}
}
