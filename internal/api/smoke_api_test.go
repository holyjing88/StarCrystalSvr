// Smoke / 契约测试：覆盖公开路由的 HTTP 状态码与业务 code（不依赖外部 MySQL/SMTP）。
// 反外挂：进程内逻辑见 internal/antifraud、internal/regpolicy、internal/httpx；
// 注册同日设备/IP 封顶需 MySQL：见 internal/integration（-tags=integration）。
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"starcrystal/server/internal/service"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doAPI(t *testing.T, srv *httptest.Server, method, pathWithQuery string, hdr http.Header, body string) (int, apiEnvelope) {
	t.Helper()
	u := srv.URL + pathWithQuery
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		t.Fatal(err)
	}
	if hdr != nil {
		req.Header = hdr.Clone()
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var env apiEnvelope
	_ = json.Unmarshal(raw, &env)
	return res.StatusCode, env
}

func idipTestHeaders(s *Server) http.Header {
	h := http.Header{}
	key := "change-me-in-production"
	if s != nil && s.economy != nil {
		if k := strings.TrimSpace(s.economy.Config.Idip.Key); k != "" {
			key = k
		}
	}
	h.Set("X-IDIP-Key", key)
	return h
}

func TestSmoke_GET_RootHealthFaviconGames(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	t.Run("GET /", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodGet, "/", nil, "")
		if st != http.StatusOK || env.Code != 0 {
			t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
		}
	})

	t.Run("GET /healthz", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodGet, "/healthz", nil, "")
		if st != http.StatusOK || env.Code != 0 {
			t.Fatalf("status=%d code=%d", st, env.Code)
		}
	})

	t.Run("GET favicon", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/favicon.ico", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("favicon: %d", res.StatusCode)
		}
	})

	t.Run("GET games missing query", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodGet, "/api/v1/games", nil, "")
		if st != http.StatusBadRequest || env.Code != 1400 {
			t.Fatalf("games invalid: status=%d code=%d", st, env.Code)
		}
	})

	gameCfg := filepath.Join(t.TempDir(), "games.smoke.json")
	if err := os.WriteFile(gameCfg, []byte(`{"list":[{"gameId":"smoke1","name":"S","note":"","entryType":"webview","entryUrl":"https://example.invalid","sort":1,"status":"online","minAppVersion":""}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMES_CONFIG", gameCfg)
	srv2 := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv2.Close)

	t.Run("GET games ok with temp config", func(t *testing.T) {
		st, env := doAPI(t, srv2, http.MethodGet, "/api/v1/games?appVersion=1.0.0&platform=android", nil, "")
		if st != http.StatusOK || env.Code != 0 {
			t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
		}
	})
}

func TestSmoke_OPTIONS_CORS_NoContent(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/v1/auth/me", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS expected 204, got %d", res.StatusCode)
	}
	if res.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing CORS allow-origin header")
	}
}

func TestSmoke_GET_Wallet(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	for _, path := range []string{"/api/v1/wallet/balance", "/api/v1/wallet/ledger"} {
		t.Run(path, func(t *testing.T) {
			st, env := doAPI(t, srv, http.MethodGet, path, nil, "")
			if st != http.StatusOK || env.Code != 0 {
				t.Fatalf("%s status=%d code=%d", path, st, env.Code)
			}
		})
	}

	t.Run("GET welfare redeem-gift stub", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodGet, "/api/v1/welfare/redeem-gift/R123", nil, "")
		if st != http.StatusOK || env.Code != 0 {
			t.Fatalf("status=%d code=%d", st, env.Code)
		}
	})

	t.Run("POST redeem-token-gift requires auth", func(t *testing.T) {
		st, _ := doAPI(t, srv, http.MethodPost, "/api/v1/welfare/redeem-token-gift", nil, ``)
		if st != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", st)
		}
	})
}

func TestSmoke_POST_Ads_Callback_Placementholder(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/ads/callback/testnet", nil, `{}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
}

func TestSmoke_POST_Ads_StartComplete_NoBearer(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/ads/start", nil, `{}`)
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("ads/start no auth: status=%d code=%d", st, env.Code)
	}

	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/ads/complete", nil, `{"watchId":"deadbeef"}`)
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("ads/complete no auth: status=%d code=%d", st, env.Code)
	}
}

func TestSmoke_POST_Ads_StartComplete_InvalidBearer(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer not-a-real-token")

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/ads/start", hdr, `{}`)
	if st != http.StatusUnauthorized || env.Code != 1407 {
		t.Fatalf("ads/start bad token: status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
}

// 需可用的 MySQL（AUTH_MYSQL_DSN 或 starcrystal.json authMysqlDsn）：游客 token 后 ads/start 应成功。
func TestSmoke_Ads_Start_ValidGuestToken_WithMysql(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil, `{"guestKey":"","deviceId":"ads-smoke"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest: status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.AccessToken == "" {
		t.Fatal("empty accessToken")
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+payload.AccessToken)
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/ads/start", h, `{}`)
	if st == http.StatusServiceUnavailable && env.Code == 1501 {
		t.Fatalf("ads/start: database unavailable — configure MySQL for tests: %s", env.Message)
	}
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("ads/start: status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
}

func TestSmoke_PostMethodOnGetOnlyRoutes(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/healthz", "{}"},
		{http.MethodPost, "/", "{}"},
		{http.MethodPost, "/api/v1/games", "{}"},
	}
	for _, tc := range tests {
		tc := tc
		name := tc.method + "_" + tc.path
		name = strings.ReplaceAll(name, "/", "_")
		t.Run(name, func(t *testing.T) {
			st, _ := doAPI(t, srv, tc.method, tc.path, nil, tc.body)
			if st != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d for %s %s", st, tc.method, tc.path)
			}
		})
	}
}

func TestSmoke_Auth_InvalidJSON_CommonPostRoutes(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	posts := []string{
		"/api/v1/auth/oauth",
		"/api/v1/auth/guest",
		"/api/v1/auth/sendverificationcode",
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/password/reset/confirm",
	}

	for _, p := range posts {
		p := p
		t.Run(p, func(t *testing.T) {
			st, env := doAPI(t, srv, http.MethodPost, p, nil, `{not-json`)
			if st != http.StatusBadRequest || env.Code != 1400 {
				t.Fatalf("%s invalid json: status=%d code=%d", p, st, env.Code)
			}
		})
	}
}

func TestSmoke_Auth_OAuth_InvalidProviderBody(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	body := `{"provider":"tiktok","idToken":"x"}`
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/oauth", nil, body)
	if st != http.StatusBadRequest || env.Code != 1400 {
		t.Fatalf("oauth bad provider: status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
}

func TestSmoke_Auth_Me_GM_MyTeam_MissingBearer(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodGet, "/api/v1/auth/me", nil, "")
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("/auth/me: status=%d code=%d", st, env.Code)
	}

	st, env = doAPI(t, srv, http.MethodGet, "/api/v1/auth/my/team", nil, "")
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("/auth/my/team: status=%d code=%d", st, env.Code)
	}

	body := `{}`
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/auth/gm/metrics", nil, body)
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("/auth/gm/metrics: status=%d code=%d", st, env.Code)
	}
}

func TestSmoke_Auth_RegisterLogin_MinimalBodies(t *testing.T) {
	t.Setenv("AUTH_SMS_MOCK", "1")
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	t.Run("register account short password", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/register", nil, `{"account":"13800138099","code":"000000","password":"short","displayName":"a"}`)
		if st != http.StatusBadRequest || env.Code != 1413 {
			t.Fatalf("status=%d code=%d", st, env.Code)
		}
	})

	t.Run("login account unknown", func(t *testing.T) {
		st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/login", nil, `{"account":"nope@missing.invalid","password":"secret12"}`)
		if st != http.StatusUnauthorized || env.Code != 1415 {
			t.Fatalf("status=%d code=%d msg=%s", st, env.Code, env.Message)
		}
	})

	t.Run("login account wrong password", func(t *testing.T) {
		phone := fmt.Sprintf("+86138%08d", time.Now().UnixNano()%100000000)
		stSend, envSend := doAPI(t, srv, http.MethodPost, "/api/v1/auth/sendverificationcode", nil, fmt.Sprintf(`{"purpose":"register","phone":%q}`, phone))
		if stSend != http.StatusOK || envSend.Code != 0 {
			t.Fatalf("sms send: status=%d code=%d", stSend, envSend.Code)
		}
		var sendWrap struct {
			DevVerifyCode string `json:"devVerifyCode"`
		}
		if err := json.Unmarshal(envSend.Data, &sendWrap); err != nil {
			t.Fatal(err)
		}
		if sendWrap.DevVerifyCode == "" {
			t.Fatal("missing devVerifyCode")
		}
		regBody := fmt.Sprintf(`{"account":%q,"code":%q,"password":"rightpass1","displayName":"t"}`, phone, sendWrap.DevVerifyCode)
		stReg, envReg := doAPI(t, srv, http.MethodPost, "/api/v1/auth/register", nil, regBody)
		if stReg != http.StatusOK || envReg.Code != 0 {
			t.Fatalf("register account: status=%d code=%d msg=%s", stReg, envReg.Code, envReg.Message)
		}
		loginBody := fmt.Sprintf(`{"account":%q,"password":"badpass99"}`, phone)
		st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/login", nil, loginBody)
		if st != http.StatusUnauthorized || env.Code != 1416 {
			t.Fatalf("status=%d code=%d msg=%s", st, env.Code, env.Message)
		}
	})
}

func TestSmoke_Auth_Sms_Send_EmptyPayload(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	body := `{}`
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/sendverificationcode", nil, body)
	if st != http.StatusBadRequest || env.Code != 1410 {
		t.Fatalf("expected 400/1410 empty account, got %d code=%d", st, env.Code)
	}
}

func TestSmoke_Auth_RegisterByCode_ShortPassword(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	body := `{"account":"13800138000","code":"000000","password":"short","displayName":"t"}`
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/register", nil, body)
	if st != http.StatusBadRequest || env.Code != 1413 {
		t.Fatalf("status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
}

func TestSmoke_BufferedVsUnbufferedReadNoPanic(t *testing.T) {
	// Regression: middleware replaces Body with NopCloser buffer; downstream must read safely.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	NewServer(service.RankRedisConfig{}).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected %d", rec.Code)
	}
}
