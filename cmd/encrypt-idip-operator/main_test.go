package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptIdipOperatorCLI(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	keyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	cmd := exec.Command("go", "run", "./cmd/encrypt-idip-operator",
		"-username", "ops_admin",
		"-password", "change-me-ops-password",
		"-cipher-key-base64", keyB64,
	)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	var op struct {
		Username    string `json:"username"`
		PasswordEnc string `json:"passwordEnc"`
	}
	if err := json.Unmarshal(out, &op); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if op.Username != "ops_admin" {
		t.Fatalf("username=%q", op.Username)
	}
	if !strings.HasPrefix(op.PasswordEnc, "v1:") {
		t.Fatalf("passwordEnc=%q", op.PasswordEnc)
	}
}

func TestEncryptIdipOperatorCLI_EnvCipherKey(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("IDIP_OPERATOR_CIPHER_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	cmd := exec.Command("go", "run", "./cmd/encrypt-idip-operator",
		"-username", "u",
		"-password", "p",
	)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"passwordEnc": "v1:`) {
		t.Fatalf("output: %s", out)
	}
}
