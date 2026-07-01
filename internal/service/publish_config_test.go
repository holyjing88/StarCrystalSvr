package service

import (
	"os"
	"path/filepath"
	"testing"

	"starcrystal/server/internal/config"
)

func TestResolveH5AssetsDir_RelativeToRepoRoot(t *testing.T) {
	root := t.TempDir()
	releaseDir := filepath.Join(root, "release", "configs")
	_ = os.MkdirAll(releaseDir, 0o755)
	h5Dir := filepath.Join(root, "release_h5")
	_ = os.MkdirAll(h5Dir, 0o755)
	cfgPath := filepath.Join(releaseDir, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	_ = os.Unsetenv("H5_ASSETS_DIR")

	got := resolveH5AssetsDir(config.PublishConfig{H5AssetsDir: "release_h5"})
	want := filepath.Join(root, "release_h5")
	if got != want {
		t.Fatalf("resolveH5AssetsDir=%q want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("stat resolved dir: %v", err)
	}
}

func TestResolveH5AssetsDir_MatchesH5AssetsDirDefault(t *testing.T) {
	root := t.TempDir()
	releaseDir := filepath.Join(root, "release", "configs")
	_ = os.MkdirAll(releaseDir, 0o755)
	cfgPath := filepath.Join(releaseDir, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	_ = os.Unsetenv("H5_ASSETS_DIR")

	fromPublish := resolveH5AssetsDir(config.PublishConfig{H5AssetsDir: "release_h5"})
	fromDefault := H5AssetsDir()
	if fromPublish != fromDefault {
		t.Fatalf("publish=%q default=%q", fromPublish, fromDefault)
	}
}
