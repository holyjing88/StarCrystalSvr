package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStarcrystalReleaseLogDirBesideConfigs_ReleaseLayout(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "starcrystal.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	logDir, ok := starcrystalReleaseLogDirBesideConfigs(dir)
	if !ok {
		t.Fatal("expected ok")
	}
	want := filepath.Join(dir, "log")
	if logDir != want {
		t.Fatalf("got %q want %q", logDir, want)
	}
}

func TestStarcrystalReleaseLogDirBesideConfigs_NoConfigs(t *testing.T) {
	dir := t.TempDir()
	if _, ok := starcrystalReleaseLogDirBesideConfigs(dir); ok {
		t.Fatal("expected not ok without configs/starcrystal.json")
	}
}

func TestStarcrystalReleaseLogDirsForPersist_NoDoubleRelease(t *testing.T) {
	releaseRoot := filepath.Join("..", "..", "release")
	cfg := filepath.Join(releaseRoot, "configs", "starcrystal.json")
	if st, err := os.Stat(cfg); err != nil || st.IsDir() {
		t.Skip("repo release/configs/starcrystal.json not present")
	}
	absRelease, err := filepath.Abs(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	wantLog := filepath.Join(absRelease, "log")
	for _, d := range starcrystalReleaseLogDirsForPersist() {
		if strings.Contains(filepath.ToSlash(d), "/release/release/") {
			t.Fatalf("must not contain release/release/log: %q", d)
		}
	}
	found := false
	for _, d := range starcrystalReleaseLogDirsForPersist() {
		abs, _ := filepath.Abs(d)
		if abs == wantLog {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in %v", wantLog, starcrystalReleaseLogDirsForPersist())
	}
}
