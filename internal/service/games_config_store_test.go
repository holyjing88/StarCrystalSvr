package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntryURLMissingVersion(t *testing.T) {
	if !EntryURLMissingVersion("") {
		t.Fatal("empty should miss")
	}
	if !EntryURLMissingVersion("h5/game1/index.html") {
		t.Fatal("no v should miss")
	}
	if !EntryURLMissingVersion("h5/game1/index.html?v=1.0") {
		t.Fatal("two segments should miss")
	}
	if EntryURLMissingVersion("h5/game1/index.html?v=1.0.0.0") {
		t.Fatal("valid four segments should not miss")
	}
}

func TestGamesConfigUpsertPreservesDownloadFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "games.json")
	initial := `{"list":[{"gameId":"g001","name":"test","entryType":"h5","entryUrl":"h5/game1/index.html?v=1.0.0.0","downloadUrl":"h5/game1_v1.0.0.0.tar.gz","packageBytes":100,"downloadSha256":"` + strings.Repeat("a", 64) + `","sort":1,"status":"online"}]}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	cfg, err := LoadGamesConfigForIDIP()
	if err != nil {
		t.Fatal(err)
	}
	name := "patched"
	newVer, err := UpsertGameItem(cfg.ConfigVersion, GameUpsertPatch{
		GameID: "g001",
		Name:   &name,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err := LoadGamesConfigForIDIP()
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.ConfigVersion != newVer {
		t.Fatalf("version mismatch")
	}
	if cfg2.Items[0].Name != "patched" {
		t.Fatalf("name=%q", cfg2.Items[0].Name)
	}
	if cfg2.Items[0].DownloadURL != "h5/game1_v1.0.0.0.tar.gz" {
		t.Fatalf("downloadUrl=%q", cfg2.Items[0].DownloadURL)
	}
	if cfg2.Items[0].PackageBytes != 100 {
		t.Fatalf("packageBytes=%d", cfg2.Items[0].PackageBytes)
	}
}

func TestGamesConfigUpsertStaleVersion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "games.json")
	initial := `{"list":[{"gameId":"g001","name":"x","entryType":"h5","entryUrl":"a/index.html?v=1.0.0.0","sort":1}]}`
	_ = os.WriteFile(cfgPath, []byte(initial), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	cfg, _ := LoadGamesConfigForIDIP()
	name := "y"
	_, err := UpsertGameItem(strings.Repeat("0", 64), GameUpsertPatch{GameID: "g001", Name: &name})
	if err != ErrGamesConfigConflict {
		t.Fatalf("err=%v", err)
	}
	_ = cfg
}
