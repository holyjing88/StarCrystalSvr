package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"starcrystal/server/internal/service"
)

func TestIdipGamesH5UploadAndAudit(t *testing.T) {
	service.ResetAuditLogsForTests()
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	fixture := filepath.Join("..", "..", "tools", "idip-webclient", "src", "tests", "fixtures", "vitest-game1.zip")
	if _, err := os.Stat(fixture); err != nil {
		t.Skip(fixture, err)
	}
	zipBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{mux: http.NewServeMux()}
	s.registerIdipGamesRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	hdr := idipTestHeaders(nil)

	body, contentType := multipartUpload(t, zipBytes, "vitest-game1.zip", `{"gameId":"vitest-api-1","minigameVersion":"1.0.0.1","name":"API Test","entryType":"h5","status":"offline","sort":999}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/idip/v1/games/h5/upload", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = hdr.Clone()
	req.Header.Set("Content-Type", contentType)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.StatusCode, string(raw))
	}

	st, auditEnv := doAPI(t, srv, http.MethodGet, "/idip/v1/audit/logs?action=h5_upload&limit=5", hdr, "")
	if st != http.StatusOK || auditEnv.Code != 0 {
		t.Fatalf("audit status=%d code=%d", st, auditEnv.Code)
	}
}

func multipartUpload(t *testing.T, fileBytes []byte, fileName, metaJSON string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(fileBytes); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("meta", metaJSON); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}
