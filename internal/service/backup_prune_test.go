package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneTimestampBackups_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	prefix := "games.json.bak."
	for _, ts := range []string{"100", "200", "300", "400", "500"} {
		if err := os.WriteFile(filepath.Join(dir, prefix+ts), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneTimestampBackups(dir, prefix, 3)
	remaining := 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			remaining++
		}
	}
	if remaining != 3 {
		t.Fatalf("remaining=%d want 3", remaining)
	}
	if _, err := os.Stat(filepath.Join(dir, prefix+"500")); err != nil {
		t.Fatal("expected newest backup kept")
	}
	if _, err := os.Stat(filepath.Join(dir, prefix+"100")); !os.IsNotExist(err) {
		t.Fatal("expected oldest backup removed")
	}
}
