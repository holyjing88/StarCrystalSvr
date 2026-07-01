package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starcrystal/server/internal/service"
)

// 人气榜：POST 上报 + GET 列表（未配 REDIS_ADDR 时为内存存储）。
func TestRankPlayAndListMemory(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	s := NewServer(service.RankRedisConfig{})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	playURL := srv.URL + "/api/v1/rank/play"
	body := `{"board":"popularity","gameId":"game_demo_1"}`
	req, err := http.NewRequest(http.MethodPost, playURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST rank/play want 200 got %d body=%s", res.StatusCode, string(b))
	}

	getURL := srv.URL + "/api/v1/rank?board=popularity&limit=10&lang=zh"
	res2, err := http.Get(getURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res2.Body.Close() })
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("GET rank want 200 got %d body=%s", res2.StatusCode, string(b))
	}
	raw, _ := io.ReadAll(res2.Body)
	var env struct {
		Code int `json:"code"`
		Data struct {
			Board string `json:"board"`
			Items []struct {
				GameID    string `json:"gameId"`
				Name      string `json:"name"`
				PlayCount int64 `json:"playCount"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 {
		t.Fatalf("code=%d body=%s", env.Code, string(raw))
	}
	if env.Data.Board != "popularity" {
		t.Fatalf("board=%q", env.Data.Board)
	}
	if len(env.Data.Items) < 1 {
		t.Fatalf("expected at least 1 item, got %d", len(env.Data.Items))
	}
	found := false
	for _, it := range env.Data.Items {
		if it.GameID == "game_demo_1" && it.PlayCount >= 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing game_demo_1 in %s", string(raw))
	}
}
