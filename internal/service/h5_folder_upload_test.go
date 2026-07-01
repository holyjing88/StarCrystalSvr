package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestH5FolderUpload_Create(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	gameDir := "folder-game1"
	version := "1.0.0.4"
	entries := []H5FolderEntry{
		{RelPath: gameDir + "/index.html", Data: []byte("<html>folder</html>")},
		{RelPath: gameDir + "/main.js", Data: []byte("// main")},
	}

	res, err := ProcessH5FolderUpload(entries, H5UploadMeta{
		GameID: "folder-001", MinigameVersion: version, Name: "Folder Game", EntryType: "h5", Status: "offline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GameDirName != gameDir {
		t.Fatalf("gameDirName=%q", res.GameDirName)
	}
	if _, err := os.Stat(filepath.Join(h5, gameDir, "index.html")); err != nil {
		t.Fatal(err)
	}
	tarPath := PackageTarPath(h5, gameDir, version)
	if _, err := os.Stat(tarPath); err != nil {
		t.Fatal(err)
	}
	if res.DownloadSha256 == "" || res.PackageBytes <= 0 {
		t.Fatalf("package meta missing: %+v", res)
	}
}

func TestNormalizeH5FolderEntries_RejectsMissingIndex(t *testing.T) {
	_, _, err := normalizeH5FolderEntries([]H5FolderEntry{
		{RelPath: "only/main.js", Data: []byte("x")},
	})
	if err == nil {
		t.Fatal("want error")
	}
}
