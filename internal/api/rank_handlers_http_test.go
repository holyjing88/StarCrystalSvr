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

// TestRankHandlersHTTPMemoryBackend 不拉起 MySQL / 全量 NewServer，仅验证与 Unity 一致的
// GET /api/v1/rank?board=popularity&limit=&lang= 与 POST /api/v1/rank/play 的 HTTP 与 JSON 信封（内存排行后端）。
func TestRankHandlersHTTPMemoryBackend(t *testing.T) {
	games := service.NewGameService()
	rankSvc := service.NewRankService(games, service.RankRedisConfig{})
	s := &Server{
		mux:         http.NewServeMux(),
		gameService: games,
		rankService: rankSvc,
	}
	s.mux.HandleFunc("GET /api/v1/rank", s.handleRankList)
	s.mux.HandleFunc("POST /api/v1/rank/play", s.handleRankPlay)
	s.mux.HandleFunc("POST /api/v1/rank/activity", s.handleRankActivity)

	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)

	playURL := ts.URL + "/api/v1/rank/play"
	body := `{"board":"popularity","gameId":"e2e_game_1"}`
	res, err := http.Post(playURL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST rank/play want 200 got %d body=%s", res.StatusCode, string(b))
	}
	playRaw, _ := io.ReadAll(res.Body)
	var playEnv struct {
		Code int `json:"code"`
		Data struct {
			GameID    string `json:"gameId"`
			PlayCount int64  `json:"playCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(playRaw, &playEnv); err != nil {
		t.Fatal(err)
	}
	if playEnv.Code != 0 || playEnv.Data.PlayCount < 1 {
		t.Fatalf("play response: code=%d data=%+v raw=%s", playEnv.Code, playEnv.Data, string(playRaw))
	}

	getURL := ts.URL + "/api/v1/rank?board=popularity&limit=10&lang=zh"
	res2, err := http.Get(getURL)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("GET rank want 200 got %d body=%s", res2.StatusCode, string(b))
	}
	listRaw, _ := io.ReadAll(res2.Body)
	var listEnv struct {
		Code int `json:"code"`
		Data struct {
			Board string `json:"board"`
			Items []struct {
				GameID    string `json:"gameId"`
				Name      string `json:"name"`
				PlayCount int64  `json:"playCount"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRaw, &listEnv); err != nil {
		t.Fatal(err)
	}
	if listEnv.Code != 0 || listEnv.Data.Board != "popularity" {
		t.Fatalf("list envelope: code=%d board=%q raw=%s", listEnv.Code, listEnv.Data.Board, string(listRaw))
	}
	found := false
	for _, it := range listEnv.Data.Items {
		if it.GameID == "e2e_game_1" && it.PlayCount >= 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected e2e_game_1 in list, got %s", string(listRaw))
	}

	actURL := ts.URL + "/api/v1/rank/activity"
	actBody := `{"board":"activity","durationSec":30}`
	res3, err := http.Post(actURL, "application/json", strings.NewReader(actBody))
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(res3.Body)
		t.Fatalf("POST rank/activity without bearer want 401 got %d body=%s", res3.StatusCode, string(b))
	}
}
