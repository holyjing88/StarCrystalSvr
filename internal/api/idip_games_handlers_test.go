package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func idipGamesTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := &Server{mux: http.NewServeMux()}
	s.registerIdipGamesRoutes()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestIdipGamesListAndUpsert(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "..", "release", "configs", "games.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip("games.json missing: ", err)
	}
	cfgPath := filepath.Join(dir, "games.json")
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	srv := idipGamesTestServer(t)
	hdr := idipTestHeaders(nil)

	st, listEnv := doAPI(t, srv, http.MethodGet, "/idip/v1/games/list", hdr, "")
	if st != http.StatusOK || listEnv.Code != 0 {
		t.Fatalf("list status=%d code=%d msg=%q", st, listEnv.Code, listEnv.Message)
	}
	var listData struct {
		ConfigVersion string `json:"configVersion"`
		List          []struct {
			GameID      string `json:"gameId"`
			DownloadURL string `json:"downloadUrl"`
		} `json:"list"`
	}
	if err := json.Unmarshal(listEnv.Data, &listData); err != nil {
		t.Fatal(err)
	}
	if len(listData.ConfigVersion) != 64 {
		t.Fatalf("configVersion len=%d", len(listData.ConfigVersion))
	}
	var g001Download string
	for _, row := range listData.List {
		if strings.EqualFold(row.GameID, "g001") {
			g001Download = row.DownloadURL
			break
		}
	}

	staleBody := `{"expectedConfigVersion":"` + strings.Repeat("0", 64) + `","gameId":"g001","name":"x"}`
	st, staleEnv := doAPI(t, srv, http.MethodPost, "/idip/v1/games/upsert", hdr, staleBody)
	if st != http.StatusConflict || staleEnv.Code != 1409 {
		t.Fatalf("stale upsert status=%d code=%d", st, staleEnv.Code)
	}

	upBody, _ := json.Marshal(map[string]any{
		"expectedConfigVersion": listData.ConfigVersion,
		"gameId":                "g001",
		"name":                  "验收测试名",
		"status":                "online",
	})
	st, upEnv := doAPI(t, srv, http.MethodPost, "/idip/v1/games/upsert", hdr, string(upBody))
	if st != http.StatusOK || upEnv.Code != 0 {
		t.Fatalf("upsert status=%d code=%d msg=%q", st, upEnv.Code, upEnv.Message)
	}
	var upData struct {
		ConfigVersion string `json:"configVersion"`
	}
	_ = json.Unmarshal(upEnv.Data, &upData)
	if upData.ConfigVersion == "" || upData.ConfigVersion == listData.ConfigVersion {
		t.Fatalf("expected new configVersion")
	}

	st, list2 := doAPI(t, srv, http.MethodGet, "/idip/v1/games/list", hdr, "")
	if st != http.StatusOK || list2.Code != 0 {
		t.Fatal(list2.Message)
	}
	var list2Data struct {
		List []struct {
			GameID      string `json:"gameId"`
			Name        string `json:"name"`
			DownloadURL string `json:"downloadUrl"`
		} `json:"list"`
	}
	_ = json.Unmarshal(list2.Data, &list2Data)
	for _, row := range list2Data.List {
		if strings.EqualFold(row.GameID, "g001") {
			if row.Name != "验收测试名" {
				t.Fatalf("name=%q", row.Name)
			}
			if row.DownloadURL != g001Download {
				t.Fatalf("downloadUrl changed: %q -> %q", g001Download, row.DownloadURL)
			}
		}
	}

	_, _ = doAPI(t, srv, http.MethodPost, "/idip/v1/games/upsert", hdr, string(mustJSON(t, map[string]any{
		"expectedConfigVersion": upData.ConfigVersion,
		"gameId":                "g001",
		"name":                  "消除宝石",
	})))
}

func TestIdipGamesDeleteRequiresOffline(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "..", "release", "configs", "games.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	cfgPath := filepath.Join(dir, "games.json")
	_ = os.WriteFile(cfgPath, raw, 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)

	srv := idipGamesTestServer(t)
	hdr := idipTestHeaders(nil)
	_, listEnv := doAPI(t, srv, http.MethodGet, "/idip/v1/games/list", hdr, "")
	var listData struct {
		ConfigVersion string `json:"configVersion"`
	}
	_ = json.Unmarshal(listEnv.Data, &listData)
	body := mustJSON(t, map[string]any{
		"expectedConfigVersion": listData.ConfigVersion,
		"gameId":                "g001",
		"deleteH5Dir":           false,
	})
	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/games/delete", hdr, string(body))
	if st != http.StatusBadRequest || env.Code != 1400 {
		t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	if !strings.Contains(strings.ToLower(env.Message), "offline") {
		t.Fatalf("message=%q", env.Message)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
