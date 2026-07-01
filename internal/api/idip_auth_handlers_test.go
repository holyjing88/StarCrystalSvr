package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/service"
)

func idipAuthTestServer(t *testing.T, cfg config.IdipConfig) *httptest.Server {
	t.Helper()
	s := &Server{
		mux:          http.NewServeMux(),
		idipSessions: service.NewIdipSessionServiceForTest(cfg),
		economy:      &service.EconomyBundle{Config: config.EconomyConfig{Idip: cfg}},
	}
	s.registerIdipAuthRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	return srv
}

func idipEncryptedOperatorCfg(t *testing.T) config.IdipConfig {
	t.Helper()
	keyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	key, err := service.DecodeCipherKeyForTest(keyB64)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := service.EncryptIdipPassword("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	return config.IdipConfig{
		Key:               "test-key",
		OperatorCipherKey: keyB64,
		Operators:         []config.IdipOperator{{Username: "ops_admin", PasswordEnc: enc}},
	}
}

func TestIdipAuth_LoginWithEncryptedOperator(t *testing.T) {
	srv := idipAuthTestServer(t, idipEncryptedOperatorCfg(t))

	body := `{"username":"ops_admin","password":"secret"}`
	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/auth/login", nil, body)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("login status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	var data struct {
		SessionToken string `json:"sessionToken"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil || data.SessionToken == "" {
		t.Fatalf("token: %v data=%s", err, string(env.Data))
	}
	hdr := http.Header{}
	hdr.Set("X-IDIP-Session", data.SessionToken)
	st, env = doAPI(t, srv, http.MethodPost, "/idip/v1/auth/heartbeat", hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("heartbeat status=%d code=%d", st, env.Code)
	}
}

func TestIdipAuth_LoginWrongPassword(t *testing.T) {
	srv := idipAuthTestServer(t, idipEncryptedOperatorCfg(t))

	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/auth/login", nil, `{"username":"ops_admin","password":"wrong"}`)
	if st != http.StatusUnauthorized || env.Code != 1401 {
		t.Fatalf("login status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func TestIdipAuth_KeyOnlyAccessProtectedRoute(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "games.json")
	if err := os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	cfg := config.IdipConfig{Key: "test-key"}
	s := &Server{
		mux:          http.NewServeMux(),
		idipSessions: service.NewIdipSessionServiceForTest(cfg),
		economy:      &service.EconomyBundle{Config: config.EconomyConfig{Idip: cfg}},
	}
	s.registerIdipAuthRoutes()
	s.registerIdipGamesRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	hdr := http.Header{}
	hdr.Set("X-IDIP-Key", "test-key")
	st, env := doAPI(t, srv, http.MethodGet, "/idip/v1/games/list", hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("key-only list status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func TestIdipAuth_SessionRequiredWithoutKeyOrSession(t *testing.T) {
	cfg := config.IdipConfig{Key: "test-key"}
	s := &Server{
		mux:          http.NewServeMux(),
		idipSessions: service.NewIdipSessionServiceForTest(cfg),
		economy:      &service.EconomyBundle{Config: config.EconomyConfig{Idip: cfg}},
	}
	s.registerIdipGamesRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodGet, "/idip/v1/games/list", nil, "")
	if st != http.StatusForbidden || env.Code != 1403 {
		t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func TestIdipAuth_LoginKickInvalidatesFirstSession(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "games.json")
	if err := os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	cfg := idipEncryptedOperatorCfg(t)
	s := &Server{
		mux:          http.NewServeMux(),
		idipSessions: service.NewIdipSessionServiceForTest(cfg),
		economy:      &service.EconomyBundle{Config: config.EconomyConfig{Idip: cfg}},
	}
	s.registerIdipAuthRoutes()
	s.registerIdipGamesRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)

	body := `{"username":"ops_admin","password":"secret"}`
	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/auth/login", nil, body)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatal(env.Message)
	}
	var first struct {
		SessionToken string `json:"sessionToken"`
	}
	_ = json.Unmarshal(env.Data, &first)
	hdr1 := http.Header{}
	hdr1.Set("X-IDIP-Session", first.SessionToken)
	st, _ = doAPI(t, srv, http.MethodPost, "/idip/v1/auth/heartbeat", hdr1, "")
	if st != http.StatusOK {
		t.Fatalf("first heartbeat status=%d", st)
	}

	st, env = doAPI(t, srv, http.MethodPost, "/idip/v1/auth/login", nil, body)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatal(env.Message)
	}
	st, env = doAPI(t, srv, http.MethodPost, "/idip/v1/auth/heartbeat", hdr1, "")
	if st != http.StatusUnauthorized || env.Code != 1401 {
		t.Fatalf("kicked session status=%d code=%d", st, env.Code)
	}
}
