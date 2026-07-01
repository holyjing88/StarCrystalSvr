package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"starcrystal/server/internal/service"
)

// TSK-S-001：无 Bearer 领取 → 1406
func TestTasksClaim_NoBearer(t *testing.T) {
	srv := httptestNewServer(t, NewServer(service.RankRedisConfig{}).Handler())
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/tasks/claim", nil,
		`{"taskId":"daily_free_claim","adBonus":false}`)
	if st != http.StatusUnauthorized || env.Code != 1406 {
		t.Fatalf("no bearer want 1406 got status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func TestTasksWelfare_GuestAuth(t *testing.T) {
	srv := httptestNewServer(t, NewServer(service.RankRedisConfig{}).Handler())
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"task-welfare-guest","deviceId":"task-welfare-device"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest login status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	if guest.AccessToken == "" {
		t.Fatal("empty access token")
	}
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	st, env = doAPI(t, srv, http.MethodGet, "/api/v1/tasks/welfare?lang=zh", hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("welfare status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	var data struct {
		TodayYmd int `json:"todayYmd"`
		Signin7d struct {
			CanClaim bool `json:"canClaim"`
		} `json:"signin7d"`
		Tasks []struct {
			TaskID string `json:"taskId"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	decodeData(t, env.Data, &data)
	if data.TodayYmd <= 0 || !data.Signin7d.CanClaim {
		t.Fatalf("welfare data=%+v", data)
	}
}

// TSK-S-002：未达进度领取 play_daily_60s → 1421
func TestTasksClaim_Play60s_NotClaimable(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	if s.economy == nil {
		t.Skip("economy not wired")
	}
	srv := httptestNewServer(t, s.Handler())
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"task-play60-guest","deviceId":"task-play60-device"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest login failed")
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/tasks/claim", hdr,
		`{"taskId":"play_daily_60s","adBonus":false}`)
	if st != http.StatusBadRequest || env.Code != 1421 {
		t.Fatalf("play60s without activity want 1421 got status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func TestTasksClaim_DailyFree(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	if s.economy == nil {
		t.Skip("economy not wired")
	}
	srv := httptestNewServer(t, s.Handler())
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"task-claim-guest3","deviceId":"task-claim-device3"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest login failed")
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/tasks/claim", hdr,
		`{"taskId":"daily_free_claim","adBonus":false}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("claim status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	var claim struct {
		GrantedGold float64 `json:"grantedGold"`
	}
	decodeData(t, env.Data, &claim)
	if claim.GrantedGold != 50 {
		t.Fatalf("granted=%.0f want 50", claim.GrantedGold)
	}
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/tasks/claim", hdr,
		`{"taskId":"daily_free_claim","adBonus":false}`)
	if st != http.StatusBadRequest || env.Code != 1422 {
		t.Fatalf("duplicate claim want 1422 got status=%d code=%d", st, env.Code)
	}
}

// TSK-S-007：无有效邀请时 daily_invite 不可领 → 1421
func TestTasksClaim_DailyInvite_NotClaimable(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	if s.economy == nil {
		t.Skip("economy not wired")
	}
	srv := httptestNewServer(t, s.Handler())
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"task-invite-guest","deviceId":"task-invite-device"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest login failed")
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/tasks/claim", hdr,
		`{"taskId":"daily_invite","adBonus":false}`)
	if st != http.StatusBadRequest || env.Code != 1421 {
		t.Fatalf("daily_invite without invite want 1421 got status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
}

func httptestNewServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func decodeData(t *testing.T, raw any, dest any) {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		t.Fatal(err)
	}
}
