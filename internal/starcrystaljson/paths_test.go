package starcrystaljson

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigCandidatesEnvFirst(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "starcrystal.json")
	if err := os.WriteFile(cfg, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ConfigEnv, cfg)
	got := ConfigCandidates()
	if len(got) == 0 || got[0] != cfg {
		t.Fatalf("expected %q first, got %v", cfg, got)
	}
}
