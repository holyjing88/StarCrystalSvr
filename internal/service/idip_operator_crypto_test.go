package service

import (
	"os"
	"strings"
	"testing"

	"starcrystal/server/internal/config"
)

const testCipherKeyB64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func testCipherKey(t *testing.T) []byte {
	t.Helper()
	key, err := decodeCipherKey(testCipherKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecryptIdipPassword_RoundTrip(t *testing.T) {
	key := testCipherKey(t)
	plain := "change-me-ops-password"
	enc, err := EncryptIdipPassword(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "v1:") {
		t.Fatalf("expected v1: prefix, got %q", enc)
	}
	got, err := DecryptIdipPassword(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("decrypt: got %q want %q", got, plain)
	}
}

func TestEncryptIdipPassword_DifferentNonces(t *testing.T) {
	key := testCipherKey(t)
	a, err := EncryptIdipPassword("same", key)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncryptIdipPassword("same", key)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected distinct ciphertexts (random nonce)")
	}
}

func TestDecryptIdipPassword_Invalid(t *testing.T) {
	key := testCipherKey(t)
	cases := []string{"", "plain", "v1:not-valid-base64!!!", "v1:AA=="}
	for _, enc := range cases {
		if _, err := DecryptIdipPassword(enc, key); err == nil {
			t.Fatalf("expected error for %q", enc)
		}
	}
}

func TestDecodeCipherKey_Invalid(t *testing.T) {
	for _, s := range []string{"", "not-b64", "YQ=="} {
		if _, err := decodeCipherKey(s); err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

func TestResolveIdipOperatorCipherKey_EnvOverridesConfig(t *testing.T) {
	t.Setenv("IDIP_OPERATOR_CIPHER_KEY", testCipherKeyB64)
	key, err := ResolveIdipOperatorCipherKey(config.IdipConfig{
		OperatorCipherKey: "YQ==", // invalid, should be ignored when env set
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("len=%d", len(key))
	}
}

func TestVerifyIdipOperator_Encrypted(t *testing.T) {
	key := testCipherKey(t)
	enc, err := EncryptIdipPassword("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.IdipConfig{
		OperatorCipherKey: testCipherKeyB64,
		Operators:         []config.IdipOperator{{Username: "ops_admin", PasswordEnc: enc}},
	}
	if !VerifyIdipOperator(cfg, "ops_admin", "secret") {
		t.Fatal("expected match")
	}
	if VerifyIdipOperator(cfg, "ops_admin", "wrong") {
		t.Fatal("expected reject wrong password")
	}
	if VerifyIdipOperator(cfg, "other", "secret") {
		t.Fatal("expected reject wrong username")
	}
}

func TestVerifyIdipOperator_LegacyPlaintext(t *testing.T) {
	cfg := config.IdipConfig{
		Operators: []config.IdipOperator{{Username: "ops", Password: "plain"}},
	}
	if !VerifyIdipOperator(cfg, "ops", "plain") {
		t.Fatal("expected legacy plaintext match")
	}
	if VerifyIdipOperator(cfg, "ops", "nope") {
		t.Fatal("expected reject")
	}
}

func TestVerifyIdipOperator_EncryptedWithoutKey(t *testing.T) {
	key := testCipherKey(t)
	enc, _ := EncryptIdipPassword("x", key)
	cfg := config.IdipConfig{
		Operators: []config.IdipOperator{{Username: "u", PasswordEnc: enc}},
	}
	if VerifyIdipOperator(cfg, "u", "x") {
		t.Fatal("expected false when cipher key missing")
	}
}

func TestVerifyIdipOperator_CaseInsensitiveUsername(t *testing.T) {
	cfg := config.IdipConfig{
		Operators: []config.IdipOperator{{Username: "Ops_Admin", Password: "pw"}},
	}
	if !VerifyIdipOperator(cfg, "ops_admin", "pw") {
		t.Fatal("expected case-insensitive username")
	}
}

func TestResolveIdipOperatorCipherKey_FromConfig(t *testing.T) {
	os.Unsetenv("IDIP_OPERATOR_CIPHER_KEY")
	cfg := config.IdipConfig{OperatorCipherKey: testCipherKeyB64}
	key, err := ResolveIdipOperatorCipherKey(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("len=%d", len(key))
	}
}
