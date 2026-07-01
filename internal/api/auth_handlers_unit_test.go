package api

import (
	"net/http"
	"testing"

	"starcrystal/server/internal/service"
)

func TestMapAuthLoginFailure_Banned(t *testing.T) {
	st, code, msg := mapAuthLoginFailure(&service.AccountBannedError{
		Reason: "同设备账号同意注销旧账号",
	}, 1414)
	if st != http.StatusForbidden {
		t.Fatalf("status=%d", st)
	}
	if code != 1422 {
		t.Fatalf("code=%d", code)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestMapAuthLoginFailure_Silent(t *testing.T) {
	st, code, msg := mapAuthLoginFailure(&service.AccountSilentError{
		RemainingSec: 120,
	}, 1414)
	if st != http.StatusForbidden {
		t.Fatalf("status=%d", st)
	}
	if code != 1421 {
		t.Fatalf("code=%d", code)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
}
