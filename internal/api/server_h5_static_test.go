package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIPort_RejectsH5PathsWhenDisabled(t *testing.T) {
	t.Setenv("DISABLE_API_H5_STATIC", "1")

	s := &Server{mux: http.NewServeMux()}
	s.registerPublishStaticRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	for _, path := range []string{
		"/h5/game2/index.html",
		"/assets/h5/game2/index.html",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: status=%d want 404", path, res.StatusCode)
		}
	}

}
