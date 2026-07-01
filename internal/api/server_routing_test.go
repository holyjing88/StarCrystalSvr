package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starcrystal/server/internal/service"
)

// resolveTestAuthMysqlDSN 供会 chdir 的用例：子进程 cwd 变化后仍能用 MySQL。
func resolveTestAuthMysqlDSN(fromDir string) string {
	if v := strings.TrimSpace(os.Getenv("AUTH_MYSQL_DSN")); v != "" {
		return v
	}
	for _, rel := range []string{
		filepath.Join("..", "..", "release", "configs", "starcrystal.json"),
		filepath.Join("release", "configs", "starcrystal.json"),
	} {
		path := filepath.Join(fromDir, rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			AuthMysqlDsn string `json:"authMysqlDsn"`
		}
		if json.Unmarshal(raw, &cfg) == nil && strings.TrimSpace(cfg.AuthMysqlDsn) != "" {
			return strings.TrimSpace(cfg.AuthMysqlDsn)
		}
	}
	return ""
}

const jsonLegacyPasswordResetEmailOnly = `{"email":"a@b.com"}`

// Regression: POST sendverificationcode（找回密码兼容仅 email 字段）不得 405。
func TestSendVerificationCodePasswordResetAcceptsPost(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/sendverificationcode", strings.NewReader(jsonLegacyPasswordResetEmailOnly))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })

	if res.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("POST sendverificationcode: got 405, Allow=%q", res.Header.Get("Allow"))
	}
	if res.StatusCode < 200 || res.StatusCode >= 600 {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
}

// 未注册的 POST .../sendverificationcode/ 不应命中业务处理器。
func TestSendVerificationCodeTrailingSlashNotCanonical(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/sendverificationcode/", strings.NewReader(`{"purpose":"register","account":"a@b.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	switch res.StatusCode {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
	default:
		t.Fatalf("POST .../sendverificationcode/ want 404 or 405, got %d (Allow=%q)", res.StatusCode, res.Header.Get("Allow"))
	}
}

// Which GET pattern makes POST to trailing path return 405 with Allow=GET,HEAD?
func TestMinimalMuxPostTrailingPath405(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/welfare/redeem-gift/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /api/v1/auth/sendverificationcode", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sendverificationcode/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	body, _ := io.ReadAll(w.Result().Body)
	t.Logf("minimal mux POST sendverificationcode/ -> %d allow=%q body=%q", w.Code, w.Header().Get("Allow"), string(body))
}

// 进程 cwd 下存在 ./assets 且已挂载时，canonical POST .../sendverificationcode 不得 405。
func TestSendVerificationCodeWithAssetsMounted(t *testing.T) {
	root := t.TempDir()
	assetsDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "dummy.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if dsn := resolveTestAuthMysqlDSN(oldWd); dsn != "" {
		t.Setenv("AUTH_MYSQL_DSN", dsn)
	} else {
		t.Skip("AUTH_MYSQL_DSN not configured (needed after chdir)")
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	s := NewServer(service.RankRedisConfig{})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/sendverificationcode", strings.NewReader(jsonLegacyPasswordResetEmailOnly))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("with ./assets mounted, POST sendverificationcode should not 405, got Allow=%q", res.Header.Get("Allow"))
	}
}

// Only file server + one POST: confirm whether GET /assets/ steals POST to unrelated paths.
func TestOnlyFileServerAndPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(t.TempDir()))))
	mux.HandleFunc("POST /api/v1/auth/sendverificationcode", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sendverificationcode/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	b, _ := io.ReadAll(w.Result().Body)
	t.Logf("only assets+post: %d allow=%q body=%q", w.Code, w.Header().Get("Allow"), string(b))
}
