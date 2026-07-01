package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GGM-001: invite.gmGrantShareMode 仅允许 none/capped。
func TestValidateInviteConfig_GmGrantShareMode(t *testing.T) {
	cfg := DefaultInviteConfig()
	if err := ValidateInviteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.GmGrantShareMode = "normal"
	if err := ValidateInviteConfig(cfg); err == nil {
		t.Fatal("expected error")
	}
}

// GGM-002: 配置加载拒绝 gmGrantShareMode=normal。
func TestLoadEconomyConfigFrom_InviteNormalRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "starcrystal.json")
	body := `{"gold":{"timezone":"Asia/Shanghai"},"welfare":{"monthTokenPool":1000},"invite":{"gmGrantShareMode":"normal"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadEconomyConfigFrom(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gmGrantShareMode") {
		t.Fatalf("unexpected: %v", err)
	}
}
