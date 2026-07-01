//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Phase 8 acceptance: A→B→C earn → share → six boards → notify → settlement → contrib_log.
// Run on Linux: STARCRYSTAL_INTEGRATION_MYSQL or AUTH_MYSQL_DSN must point at starcrystal_auth.
//
// go test ./internal/service -tags=integration -count=1 -run Integration_Phase8 -timeout 300s -v

func TestIntegration_Phase8_InviteEconomyABC(t *testing.T) {
	dsn := integrationMySQLDSN(t)
	t.Setenv("AUTH_MYSQL_DSN", dsn)
	t.Setenv("AUTH_SMS_MOCK", "1")

	svc := NewAuthService()
	if svc.PlayerRepository() == nil {
		t.Fatal("mysql player repo not configured")
	}
	econ := NewEconomyBundle(svc.PlayerRepository(), RankRedisConfig{})
	svc.AttachEconomy(econ.GoldLedger, econ.WelfareRank, econ.GoldRedis)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	a, b, _ := registerABCChain(t, svc, suffix)
	aID, bID, cID := guestAccountID("int-a-"+suffix), guestAccountID("int-b-"+suffix), guestAccountID("int-c-"+suffix)
	cleanupIntegrationAccounts(t, db, aID, bID, cID)

	if a.InviteCode == "" || b.InviteCode == "" {
		t.Fatalf("missing invite codes a=%q b=%q", a.InviteCode, b.InviteCode)
	}

	direct, _, second, _, err := svc.PlayerRepository().GetInviterInfoByAccountID(ctx, cID)
	if err != nil {
		t.Fatal(err)
	}
	if direct != bID || second != aID {
		t.Fatalf("invite chain want B=%s A=%s got direct=%s second=%s", bID, aID, direct, second)
	}

	const earn = 100.0
	_, err = econ.GoldLedger.ApplyGold(ctx, cID, GoldOpAdd, earn, GoldApplyOpts{
		BizType: "task", BizNo: "int-" + suffix, SkipDailyCap: true,
	})
	if err != nil {
		t.Fatalf("ApplyGold: %v", err)
	}

	// SHR-001 / INT-001: balances after C earns 100 (10% / 5%).
	cBal, err := svc.PlayerRepository().GetEconomyBalances(ctx, cID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "C.curgold", cBal.CurGold, earn)
	assertFloat(t, "C.cur_direct", cBal.CurDirectInviterShare, 10)
	assertFloat(t, "C.cur_second", cBal.CurSecondInviterShare, 5)

	bBal, err := svc.PlayerRepository().GetEconomyBalances(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "B.cur_downline_l1", bBal.CurDownlineL1Contrib, 10)
	assertFloat(t, "B.effective_cur", bBal.EffectiveCurGold(), 10)

	aBal, err := svc.PlayerRepository().GetEconomyBalances(ctx, aID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "A.cur_downline_l2", aBal.CurDownlineL2Contrib, 5)
	assertFloat(t, "A.effective_cur", aBal.EffectiveCurGold(), 5)

	// LOG-001: contrib_log rows for this earn.
	var logRows int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_invite_contrib_log
WHERE earner_account_id = ? AND base_gold = ?`, cID, earn).Scan(&logRows); err != nil {
		t.Fatal(err)
	}
	if logRows != 2 {
		t.Fatalf("contrib_log rows=%d want 2", logRows)
	}

	// WRK-001: six welfare boards reflect effective scores (real Redis).
	type boardWant struct {
		board string
		id    string
		want  float64
	}
	checks := []boardWant{
		{BoardWelfareGoldCur, cID, 100},
		{BoardWelfareUpContribCur, cID, 15},
		{BoardWelfareGoldCur, bID, 10},
		{BoardWelfareDownContribCur, bID, 10},
		{BoardWelfareGoldCur, aID, 5},
	}
	for _, bc := range checks {
		_, score, on, err := econ.WelfareRank.MemberRank(ctx, bc.board, bc.id)
		if err != nil {
			t.Fatalf("MemberRank %s %s: %v", bc.board, bc.id, err)
		}
		if !on {
			t.Fatalf("board %s missing member %s", bc.board, bc.id)
		}
		assertFloat(t, fmt.Sprintf("%s/%s", bc.board, bc.id), score, bc.want)
	}

	// NTF-001: pending downline contrib for B, then ack clears.
	pending, err := econ.InviteNotify.OnHeartbeat(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "B.pending_l1", pending.PendingL1, 10)
	if pending.PendingL2 != 0 {
		t.Fatalf("B.pending_l2=%v want 0", pending.PendingL2)
	}
	if err := econ.InviteNotify.Ack(ctx, bID); err != nil {
		t.Fatal(err)
	}
	pending2, err := econ.InviteNotify.OnHeartbeat(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	if pending2.PendingL1 != 0 || pending2.PendingL2 != 0 {
		t.Fatalf("after ack pending=%+v", pending2)
	}

	// NTF-002: auto-ack after notify_pending_since older than 7 days.
	_, err = econ.GoldLedger.ApplyGold(ctx, cID, GoldOpAdd, earn, GoldApplyOpts{
		BizType: "task", BizNo: "int2-" + suffix, SkipDailyCap: true,
	})
	if err != nil {
		t.Fatalf("ApplyGold second earn: %v", err)
	}
	pending3, err := econ.InviteNotify.OnHeartbeat(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	if pending3.PendingL1 < 10 {
		t.Fatalf("B.pending_l1 after second earn=%v want >=10", pending3.PendingL1)
	}
	oldSince := time.Now().AddDate(0, 0, -8).Format("2006-01-02 15:04:05")
	if _, err := db.ExecContext(ctx, `UPDATE auth_accounts SET notify_pending_since = ? WHERE account_id = ?`, oldSince, bID); err != nil {
		t.Fatal(err)
	}
	pending4, err := econ.InviteNotify.OnHeartbeat(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	if pending4.PendingL1 != 0 || pending4.PendingL2 != 0 {
		t.Fatalf("NTF-002 auto-ack pending=%+v want 0", pending4)
	}

	bBal, err = svc.PlayerRepository().GetEconomyBalances(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, "B.cur_downline_l1_after_two_earns", bBal.CurDownlineL1Contrib, 20)

	// SET-001 / SET-002: scoped RunSettlementForAccounts for B only (safe on shared DB).
	yyyymm := GoldYYYYMM(GoldNow(econ.Config.Gold.LocationName()))
	snapshot, err := econ.GoldRedis.GetMonthServerDelta(ctx, yyyymm)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot <= 0 {
		t.Fatalf("month snapshot=%v want >0 after earn+share", snapshot)
	}
	goldSpent := bBal.MonthlyGoldSpent()
	if err := econ.Settlement.RunSettlementForAccounts(ctx, yyyymm, []string{bID}); err != nil {
		t.Fatalf("RunSettlementForAccounts: %v", err)
	}
	bAfter, err := svc.PlayerRepository().GetEconomyBalances(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}

	if bAfter.CurGold != 0 || bAfter.CurDownlineL1Contrib != 0 {
		t.Fatalf("B cur not cleared: %+v", bAfter)
	}
	assertFloat(t, "B.totalgold_after_settle", bAfter.TotalGold, bBal.TotalGold+goldSpent)
	if bAfter.CurToken >= 1 {
		assertFloat(t, "B.totaltoken", bAfter.TotalToken, bAfter.CurToken)
	}

	var exchCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM welfare_exchange_log WHERE account_id = ? AND yyyymm = ?`, bID, yyyymm).Scan(&exchCount); err != nil {
		t.Fatal(err)
	}
	if exchCount != 1 {
		t.Fatalf("welfare_exchange_log count=%d want 1", exchCount)
	}

	_, score, on, err := econ.WelfareRank.MemberRank(ctx, BoardWelfareGoldCur, bID)
	if err != nil {
		t.Fatal(err)
	}
	if on && score != 0 {
		t.Fatalf("B welfare_gold_cur after settle on=%v score=%v want off-board or 0", on, score)
	}
}

func integrationMySQLDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("STARCRYSTAL_INTEGRATION_MYSQL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("AUTH_MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("STARCRYSTAL_INTEGRATION_MYSQL or AUTH_MYSQL_DSN not set")
	}
	return dsn
}

func guestAccountID(guestKey string) string {
	return "guest_" + strings.ToLower(strings.TrimSpace(guestKey))
}

func registerABCChain(t *testing.T, svc *AuthService, suffix string) (a, b, c *userRecord) {
	t.Helper()
	a, _, _, err := svc.RegisterGuest("int-a-"+suffix, "", "int-dev-a-"+suffix, "", false, false)
	if err != nil {
		t.Fatalf("register A: %v", err)
	}
	b, _, _, err = svc.RegisterGuest("int-b-"+suffix, a.InviteCode, "int-dev-b-"+suffix, "", false, false)
	if err != nil {
		t.Fatalf("register B: %v", err)
	}
	c, _, _, err = svc.RegisterGuest("int-c-"+suffix, b.InviteCode, "int-dev-c-"+suffix, "", false, false)
	if err != nil {
		t.Fatalf("register C: %v", err)
	}
	return a, b, c
}

func cleanupIntegrationAccounts(t *testing.T, db *sql.DB, ids ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		for _, id := range ids {
			_, _ = db.ExecContext(ctx, `DELETE FROM auth_invite_contrib_log WHERE earner_account_id = ? OR beneficiary_account_id = ?`, id, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM welfare_exchange_log WHERE account_id = ?`, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth_invite_members WHERE accountid = ? OR memberid = ?`, id, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth_invite_codes WHERE account_id = ?`, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth_accounts WHERE account_id = ?`, id)
		}
	})
}

func assertFloat(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("%s: got %.4f want %.4f", label, got, want)
	}
}
