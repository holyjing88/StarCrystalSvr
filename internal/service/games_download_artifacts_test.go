package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseGamesDownloadArtifacts 校验 release 内 games.json 三字段与 tar.gz 文件一致。
func TestReleaseGamesDownloadArtifacts(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "release", "configs", "games.json")
	h5Root := filepath.Join("..", "..", "release", "assets", "h5")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skip(cfgPath, err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var wrapped gameConfigFile
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatal(err)
	}

	for _, g := range wrapped.List {
		if strings.ToLower(strings.TrimSpace(g.EntryType)) != "h5" {
			continue
		}
		if g.DownloadURL == "" {
			t.Fatalf("%s missing downloadUrl", g.GameID)
		}
		if g.PackageBytes <= 0 {
			t.Fatalf("%s invalid packageBytes", g.GameID)
		}
		if len(g.DownloadSha256) != 64 {
			t.Fatalf("%s invalid downloadSha256", g.GameID)
		}
		rel := strings.TrimPrefix(strings.ReplaceAll(g.DownloadURL, "\\", "/"), "h5/")
		tarPath := filepath.Join(h5Root, rel)
		fi, err := os.Stat(tarPath)
		if err != nil {
			t.Fatalf("%s tar missing %s: %v", g.GameID, tarPath, err)
		}
		if fi.Size() != g.PackageBytes {
			t.Fatalf("%s packageBytes=%d file=%d", g.GameID, g.PackageBytes, fi.Size())
		}
		sum, err := sha256FileHex(tarPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(sum, g.DownloadSha256) {
			t.Fatalf("%s sha mismatch want=%s got=%s", g.GameID, g.DownloadSha256, sum)
		}
		if g.PackageBytes > 52_428_800 {
			t.Fatalf("%s exceeds 50MB limit: %d", g.GameID, g.PackageBytes)
		}
	}
}

func sha256FileHex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
