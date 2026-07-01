//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"starcrystal/server/internal/service"
)

// STARCRYSTAL_INTEGRATION_MYSQL=star_auth:...@tcp(127.0.0.1:3306)/starcrystal_auth?parseTime=true&loc=Local
//
// Requires auth_accounts with columns device_id, registration_ip, ad_rewards_disabled (see tools/scripts/dbscripts/sql/starcrystal_auth_mysql.sql).

func TestMySQL_DeviceDayLimit_DecideNextSignup(t *testing.T) {
	dsn := os.Getenv("STARCRYSTAL_INTEGRATION_MYSQL")
	if dsn == "" {
		t.Skip("STARCRYSTAL_INTEGRATION_MYSQL not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	dev := fmt.Sprintf("itest-dev-%s", suffix)

	t.Setenv("REG_ACCOUNTS_PER_DEVICE_PER_DAY", "2")
	t.Setenv("AUTH_MYSQL_DSN", dsn)
	svc := service.NewAuthService()

	ctx := context.Background()
	disable0, err := svc.DecideAdRewardsDisabledForSignup(ctx, dev, "")
	if err != nil {
		t.Fatal(err)
	}
	if disable0 {
		t.Fatalf("first signup on empty device/day should not disable")
	}

	var ids []string
	for i := 0; i < 2; i++ {
		aid := fmt.Sprintf("email_itest_%s_%d@invalid.local", suffix, i)
		accountID := "email_" + aid
		ids = append(ids, accountID)
		ph := fmt.Sprintf("it-pw-hash-%s-%d", suffix, i)
		_, err := db.ExecContext(ctx, `
INSERT INTO auth_accounts (
  account_id, account_type, account_value,
  email, phone, password_hash, provider, display_name,
  invited_user_id, device_id, registration_ip, ad_rewards_disabled,
  curgold, totalgold, curtoken, totaltoken,
  cur_direct_inviter_share, total_direct_inviter_share, cur_second_inviter_share, total_second_inviter_share
) VALUES (?, 'email', ?, ?, '', ?, 'password', 'it', NULL, ?, '', 0,
  0,0,0,0,0,0,0,0)`,
			accountID, aid, aid, ph, dev)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = db.ExecContext(context.Background(), "DELETE FROM auth_accounts WHERE account_id = ?", id)
		}
	})

	disableThird, err := svc.DecideAdRewardsDisabledForSignup(ctx, dev, "")
	if err != nil {
		t.Fatal(err)
	}
	if !disableThird {
		t.Fatal("third logical signup same device/day should disable ad rewards flag")
	}
}
