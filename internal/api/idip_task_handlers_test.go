package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"starcrystal/server/internal/service"
)

func TestIdipTaskTierPolicy_TogglesP1(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptestNewServer(t, s.Handler())
	baseline := service.TaskTierPolicy{P0Enabled: true, P1Enabled: false, P2Enabled: false}
	service.SetTaskTierPolicy(baseline)
	t.Cleanup(func() { service.SetTaskTierPolicy(baseline) })

	p0 := len(service.ListActiveTaskDefs())
	if p0 < 10 {
		t.Fatalf("p0 active count=%d want >=10", p0)
	}

	body := `{"p0Enabled":true,"p1Enabled":true,"p2Enabled":false}`
	st, env := doAPI(t, srv, http.MethodPost, "/idip/v1/tasks/tier-policy", idipTestHeaders(s), body)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("tier-policy status=%d code=%d", st, env.Code)
	}
	p1on := len(service.ListActiveTaskDefs())
	if p1on <= p0 {
		t.Fatalf("after p1 enable active=%d want > p0=%d", p1on, p0)
	}

	body2 := `{"p0Enabled":false,"p1Enabled":true,"p2Enabled":false}`
	st, env = doAPI(t, srv, http.MethodPost, "/idip/v1/tasks/tier-policy", idipTestHeaders(s), body2)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("tier-policy2 status=%d code=%d", st, env.Code)
	}
	p0off := countActiveByTier(service.TaskTierP0)
	if p0off != 0 {
		t.Fatalf("p0 disabled tier count=%d want 0", p0off)
	}
}

func countActiveByTier(tier service.TaskTier) int {
	n := 0
	for _, d := range service.ListActiveTaskDefs() {
		if d.Tier == tier {
			n++
		}
	}
	return n
}

func TestIdipTaskDefinitions_ReturnsCatalog(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptestNewServer(t, s.Handler())
	st, env := doAPI(t, srv, http.MethodGet, "/idip/v1/tasks/definitions", idipTestHeaders(s), "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("definitions status=%d code=%d", st, env.Code)
	}
	var data struct {
		Tasks []struct {
			TaskID string `json:"taskId"`
			Tier   string `json:"tier"`
		} `json:"tasks"`
	}
	decodeData(t, env.Data, &data)
	if len(data.Tasks) < 40 {
		t.Fatalf("catalog tasks=%d want >=40", len(data.Tasks))
	}
}

func TestIdipTaskDefinitionUpsert_Readback(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptestNewServer(t, s.Handler())
	st, env := doAPI(t, srv, http.MethodGet, "/idip/v1/tasks/definitions", idipTestHeaders(s), "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("definitions status=%d code=%d", st, env.Code)
	}
	var defs struct {
		Tasks []struct {
			TaskID     string  `json:"taskId"`
			RewardGold float64 `json:"rewardGold"`
		} `json:"tasks"`
	}
	decodeData(t, env.Data, &defs)
	if len(defs.Tasks) == 0 {
		t.Fatal("empty task catalog")
	}
	pick := defs.Tasks[0]
	for _, row := range defs.Tasks {
		if row.TaskID == "daily_free_claim" {
			pick = row
			break
		}
	}
	newReward := pick.RewardGold + 1
	body, _ := json.Marshal(map[string]any{
		"taskId":     pick.TaskID,
		"rewardGold": newReward,
	})
	st, env = doAPI(t, srv, http.MethodPost, "/idip/v1/tasks/definition/upsert", idipTestHeaders(s), string(body))
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("upsert status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	st, env = doAPI(t, srv, http.MethodGet, "/idip/v1/tasks/definitions", idipTestHeaders(s), "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatal("definitions after upsert")
	}
	var defs2 struct {
		Tasks []struct {
			TaskID     string  `json:"taskId"`
			RewardGold float64 `json:"rewardGold"`
		} `json:"tasks"`
	}
	decodeData(t, env.Data, &defs2)
	for _, row := range defs2.Tasks {
		if row.TaskID == pick.TaskID {
			if row.RewardGold != newReward {
				t.Fatalf("reward got %.0f want %.0f", row.RewardGold, newReward)
			}
			restore, _ := json.Marshal(map[string]any{
				"taskId":     pick.TaskID,
				"rewardGold": pick.RewardGold,
			})
			_, _ = doAPI(t, srv, http.MethodPost, "/idip/v1/tasks/definition/upsert", idipTestHeaders(s), string(restore))
			return
		}
	}
	t.Fatalf("task %s not found after upsert", pick.TaskID)
}

func TestTasksReport_PageView(t *testing.T) {
	s := NewServer(service.RankRedisConfig{})
	srv := httptestNewServer(t, s.Handler())
	_, _ = doAPI(t, srv, http.MethodPost, "/idip/v1/tasks/tier-policy", idipTestHeaders(s),
		`{"p0Enabled":true,"p1Enabled":true,"p2Enabled":false}`)
	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"task-report-guest","deviceId":"task-report-dev"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatal("guest login failed")
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/tasks/report", hdr,
		`{"event":"page_view","page":"welfare"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("report status=%d code=%d msg=%q", st, env.Code, env.Message)
	}
	st, env = doAPI(t, srv, http.MethodGet, "/api/v1/tasks/welfare?lang=zh", hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatal("welfare get failed")
	}
	raw, _ := json.Marshal(env.Data)
	if !contains(string(raw), "daily_visit_welfare") {
		t.Fatalf("expected daily_visit_welfare in welfare response")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
