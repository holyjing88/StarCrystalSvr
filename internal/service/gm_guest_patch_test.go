//go:build integration

package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// STARCRYSTAL_INTEGRATION_MYSQL=root:@tcp(127.0.0.1:3306)/starcrystal_auth?parseTime=true&loc=Local

func TestIntegration_GmPatch_Guest_CurGoldFirst_WithLedger(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STARCRYSTAL_INTEGRATION_MYSQL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("AUTH_MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("STARCRYSTAL_INTEGRATION_MYSQL or AUTH_MYSQL_DSN not set")
	}
	t.Setenv("AUTH_MYSQL_DSN", dsn)
	t.Setenv("AUTH_SMS_MOCK", "1")

	svc := NewAuthService()
	if svc.PlayerRepository() == nil {
		t.Fatal("mysql player repo not configured")
	}
	econ := NewEconomyBundle(svc.PlayerRepository(), RankRedisConfig{})
	svc.AttachEconomy(econ.GoldLedger, econ.WelfareRank, econ.GoldRedis)

	dev := fmt.Sprintf("gm-guest-curgold-%d", time.Now().UnixNano())
	u, _, _, err := svc.RegisterGuest("", "", dev, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.GmPatchAccountMetrics(u.ID, 99.5, 0, 0, 0, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("GmPatch curgold first with ledger: %v", err)
	}
}
