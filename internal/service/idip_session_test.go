package service

import (
	"context"
	"testing"

	"starcrystal/server/internal/config"
)

func TestIdipSession_LoginLogoutValidate(t *testing.T) {
	key, _ := decodeCipherKey("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32 bytes of 0
	cfg := config.IdipConfig{
		Operators: []config.IdipOperator{{
			Username:    "ops",
			PasswordEnc: mustEnc(t, "pw", key),
		}},
		OperatorCipherKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	svc := NewIdipSessionServiceForTest(cfg)
	ctx := context.Background()
	res, err := svc.Login(ctx, "ops", "pw", "127.0.0.1", VerifyIdipOperator)
	if err != nil {
		t.Fatal(err)
	}
	u, err := svc.ValidateSession(ctx, res.SessionToken)
	if err != nil || u != "ops" {
		t.Fatalf("validate: user=%q err=%v", u, err)
	}
	if err := svc.Logout(ctx, res.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateSession(ctx, res.SessionToken); err == nil {
		t.Fatal("expected invalid after logout")
	}
}

func mustEnc(t *testing.T, plain string, key []byte) string {
	t.Helper()
	s, err := EncryptIdipPassword(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
