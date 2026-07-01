package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starcrystal/server/internal/service"
	"starcrystal/server/internal/store"
)

// DEP-001：旧金币兑换接口已废弃。
func TestSmoke_POST_WelfareExchange_Deprecated(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/welfare/exchange", nil, `{}`)
	if st != http.StatusGone {
		t.Fatalf("status=%d want 410 Gone", st)
	}
	if env.Code != 1410 {
		t.Fatalf("business code=%d want 1410", env.Code)
	}
	if !strings.Contains(env.Message, "monthly settlement") {
		t.Fatalf("message=%q", env.Message)
	}
}

// WRK-012：福利榜不支持 period=week。
func TestSmoke_GET_WelfareRank_PeriodRejected(t *testing.T) {
	srv := httptest.NewServer(NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	st, env := doAPI(t, srv, http.MethodGet,
		"/api/v1/rank?board=welfare_gold_cur&period=week&limit=10&lang=zh", nil, "")
	if st != http.StatusBadRequest || env.Code != 1400 {
		t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

// WRK-009/010：四福利 board 查询（需 MySQL 经济模块）。
func TestSmoke_GET_WelfareFourBoards_WithEconomy(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	if s.economy == nil || s.economy.WelfareRank == nil {
		t.Skip("economy not wired (configure AUTH_MYSQL_DSN or starcrystal.json authMysqlDsn)")
	}

	ctx := t.Context()
	s.economy.WelfareRank.Notify(ctx, "e2e_welfare_a", service.WelfareChangedCurGold,
		store.EconomyBalances{CurGold: 150})
	s.economy.WelfareRank.Notify(ctx, "e2e_welfare_b", service.WelfareChangedCurGold,
		store.EconomyBalances{CurGold: 80})

	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	for _, board := range []string{
		"welfare_gold_cur", "welfare_gold_total",
		"welfare_token_cur", "welfare_token_total",
	} {
		board := board
		t.Run(board, func(t *testing.T) {
			st, env := doAPI(t, srv, http.MethodGet,
				"/api/v1/rank?board="+board+"&limit=50&lang=zh", nil, "")
			if st != http.StatusOK || env.Code != 0 {
				t.Fatalf("status=%d code=%d msg=%q", st, env.Code, env.Message)
			}
			var data struct {
				Board   string `json:"board"`
				MyRank  int64  `json:"myRank"`
				MyScore float64 `json:"myScore"`
				Items   []struct {
					AccountID string  `json:"accountId"`
					Name      string  `json:"name"`
					Gold      float64 `json:"gold"`
					Token     float64 `json:"token"`
					Rank      int64   `json:"rank"`
				} `json:"items"`
			}
			if err := json.Unmarshal(env.Data, &data); err != nil {
				t.Fatal(err)
			}
			if data.Board != board {
				t.Fatalf("board=%q", data.Board)
			}
			if board == "welfare_gold_cur" {
				if len(data.Items) < 2 {
					t.Fatalf("items=%d", len(data.Items))
				}
				if data.Items[0].AccountID == "" || data.Items[0].Rank < 1 {
					t.Fatalf("first item=%+v", data.Items[0])
				}
				if data.Items[0].Gold < data.Items[1].Gold {
					t.Fatalf("order: %+v then %+v", data.Items[0], data.Items[1])
				}
			}
		})
	}
}

// IDP-001：非内网来源拒绝（httptest 默认 RemoteAddr 为 127.0.0.1，本用例改 RemoteAddr 模拟公网）。
func TestSmoke_IDIP_ForbiddenFromPublicIP(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	req := httptest.NewRequest(http.MethodPost, "/idip/v1/gold/set-user",
		strings.NewReader(`{"accountId":"x","op":"add","amount":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
