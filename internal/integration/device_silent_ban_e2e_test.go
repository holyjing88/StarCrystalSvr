//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"starcrystal/server/internal/api"
	"starcrystal/server/internal/service"
)

type e2eEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doE2EAPI(t *testing.T, srv *httptest.Server, method, path string, body string) (int, e2eEnvelope) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var env e2eEnvelope
	_ = json.Unmarshal(raw, &env)
	return res.StatusCode, env
}

func mustSendCode(t *testing.T, srv *httptest.Server, phone string) string {
	t.Helper()
	st, env := doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/sendverificationcode",
		fmt.Sprintf(`{"purpose":"register","phone":%q}`, phone))
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("send code failed status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
	var data struct {
		DevVerifyCode string `json:"devVerifyCode"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(data.DevVerifyCode) == "" {
		t.Fatal("devVerifyCode empty")
	}
	return data.DevVerifyCode
}

func TestIntegration_DeviceSilentToBan_E2E(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STARCRYSTAL_INTEGRATION_MYSQL"))
	if dsn == "" {
		t.Skip("STARCRYSTAL_INTEGRATION_MYSQL not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Required by API auth service bootstrap.
	t.Setenv("AUTH_MYSQL_DSN", dsn)
	t.Setenv("AUTH_SMS_MOCK", "1")

	srv := httptest.NewServer(api.NewServer(service.RankRedisConfig{}).Handler())
	t.Cleanup(srv.Close)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	phoneOld := "+86139" + suffix[len(suffix)-8:]
	phoneNew := "+86138" + suffix[len(suffix)-8:]
	deviceID := "itest-silent-device-" + suffix
	oldAccountID := "phone_" + phoneOld
	newAccountID := "phone_" + phoneNew

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM auth_accounts WHERE account_id IN (?,?)", oldAccountID, newAccountID)
	})

	// 1) 先注册旧账号（占用 deviceId）
	oldCode := mustSendCode(t, srv, phoneOld)
	st, env := doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/register",
		fmt.Sprintf(`{"account":%q,"code":%q,"password":"Pass1234","displayName":"old","deviceId":%q}`,
			phoneOld, oldCode, deviceID))
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("register old failed status=%d code=%d msg=%s", st, env.Code, env.Message)
	}

	// 2) 同设备注册新账号，第一次应冲突 1420（需确认）
	newCode1 := mustSendCode(t, srv, phoneNew)
	st, env = doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/register",
		fmt.Sprintf(`{"account":%q,"code":%q,"password":"Pass1234","displayName":"new","deviceId":%q}`,
			phoneNew, newCode1, deviceID))
	if st != http.StatusConflict || env.Code != 1420 {
		t.Fatalf("expect 409/1420 first conflict, got status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
	var conflict1 struct {
		NeedConfirmation bool   `json:"needConfirmation"`
		ExistingAccount  string `json:"existingAccountId"`
	}
	_ = json.Unmarshal(env.Data, &conflict1)
	if !conflict1.NeedConfirmation {
		t.Fatalf("expect needConfirmation=true, data=%s", string(env.Data))
	}

	// 3) 确认注销后进入 6h 静默，仍返回 1420（无需再次确认）
	newCode2 := mustSendCode(t, srv, phoneNew)
	st, env = doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/register",
		fmt.Sprintf(`{"account":%q,"code":%q,"password":"Pass1234","displayName":"new","deviceId":%q,"confirmDeactivateOldAccount":true}`,
			phoneNew, newCode2, deviceID))
	if st != http.StatusConflict || env.Code != 1420 {
		t.Fatalf("expect 409/1420 after confirm, got status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
	var conflict2 struct {
		NeedConfirmation bool  `json:"needConfirmation"`
		RemainingSec     int64 `json:"remainingSilentSec"`
	}
	_ = json.Unmarshal(env.Data, &conflict2)
	if conflict2.NeedConfirmation {
		t.Fatalf("expect needConfirmation=false after confirm, data=%s", string(env.Data))
	}
	if conflict2.RemainingSec <= 0 {
		t.Fatalf("expect remainingSilentSec>0, data=%s", string(env.Data))
	}

	// 静默期内旧号登录：1421
	st, env = doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/login",
		fmt.Sprintf(`{"account":%q,"password":"Pass1234"}`, phoneOld))
	if st != http.StatusForbidden || env.Code != 1421 {
		t.Fatalf("expect login old silent 403/1421, got status=%d code=%d msg=%s", st, env.Code, env.Message)
	}

	// 4) 人为把静默期拨到过去；下次同设备注册触发旧号封禁并允许新号注册成功
	_, err = db.ExecContext(context.Background(),
		"UPDATE auth_accounts SET device_silent_until = DATE_SUB(NOW(), INTERVAL 1 SECOND), status=1, ban_reason=NULL WHERE account_id = ?",
		oldAccountID)
	if err != nil {
		t.Fatalf("update silent until failed (check schema has device_silent_until): %v", err)
	}

	newCode3 := mustSendCode(t, srv, phoneNew)
	st, env = doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/register",
		fmt.Sprintf(`{"account":%q,"code":%q,"password":"Pass1234","displayName":"new","deviceId":%q}`,
			phoneNew, newCode3, deviceID))
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("expect new register success after silent expiry, got status=%d code=%d msg=%s", st, env.Code, env.Message)
	}

	// 5) 旧号再登录：1422 + ban reason
	st, env = doE2EAPI(t, srv, http.MethodPost, "/api/v1/auth/login",
		fmt.Sprintf(`{"account":%q,"password":"Pass1234"}`, phoneOld))
	if st != http.StatusForbidden || env.Code != 1422 {
		t.Fatalf("expect old account banned 403/1422, got status=%d code=%d msg=%s", st, env.Code, env.Message)
	}
	if !strings.Contains(env.Message, "同设备账号同意注销旧账号") {
		t.Fatalf("ban reason missing, msg=%q", env.Message)
	}
}
