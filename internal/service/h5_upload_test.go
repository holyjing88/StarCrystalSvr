package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH5Upload_PlainHtmlOnlyZip(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	src := filepath.Join(root, "plain-game1")
	_ = os.MkdirAll(src, 0o755)
	_ = os.WriteFile(filepath.Join(src, "index.html"), []byte("<html>plain</html>"), 0o644)

	zipBytes, _ := buildZipFromDir(t, src, "plain-game1", "plain-game1.zip")
	res, err := ProcessH5Upload(zipBytes, "plain-game1.zip", H5UploadMeta{
		GameID: "plain-001", MinigameVersion: "1.0.0.0", Name: "Plain HTML", EntryType: "h5", Status: "offline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GameDirName != "plain-game1" {
		t.Fatalf("gameDirName=%q", res.GameDirName)
	}
	if _, err := os.Stat(filepath.Join(h5, "plain-game1", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(PackageTarPath(h5, "plain-game1", "1.0.0.0")); err != nil {
		t.Fatal(err)
	}
}

func TestH5Upload_CreateAndZipNameMismatch(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	src := writeMinimalH5Tree(t, filepath.Join(root, "src-vitest-game1"))

	badZip, _ := buildZipFromDir(t, src, "vitest-game1", "wrong-name.zip")
	_, err := ProcessH5Upload(badZip, "wrong-name.zip", H5UploadMeta{
		GameID: "vitest-001", MinigameVersion: "1.0.0.1", Name: "Test", EntryType: "h5", Status: "offline",
	})
	if err != ErrH5ZipNameMismatch {
		t.Fatalf("want ErrH5ZipNameMismatch got %v", err)
	}

	goodZip, _ := buildZipFromDir(t, src, "vitest-game1", "vitest-game1.zip")
	res, err := ProcessH5Upload(goodZip, "vitest-game1.zip", H5UploadMeta{
		GameID: "vitest-001", MinigameVersion: "1.0.0.1", Name: "Vitest Game", EntryType: "h5", Status: "offline", Sort: 999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GameDirName != "vitest-game1" {
		t.Fatalf("gameDirName=%q", res.GameDirName)
	}
	if res.PackageBytes <= 0 || len(res.DownloadSha256) != 64 {
		t.Fatalf("download meta missing bytes=%d sha=%q", res.PackageBytes, res.DownloadSha256)
	}
	if _, err := os.Stat(filepath.Join(h5, "vitest-game1", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(PackageTarPath(h5, "vitest-game1", "1.0.0.1")); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadGamesConfigForIDIP()
	if len(cfg.Items) != 1 || cfg.Items[0].GameID != "vitest-001" {
		t.Fatalf("games list=%+v", cfg.Items)
	}

	del, err := DeleteGameItem(cfg.ConfigVersion, "vitest-001", false)
	if err != nil {
		t.Fatal(err)
	}
	if del.GameDirName != "vitest-game1" {
		t.Fatalf("del gameDir=%q", del.GameDirName)
	}
}

func writeMinimalH5Tree(t *testing.T, dir string) string {
	t.Helper()
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "main.js"), []byte("// main"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "src", "settings.js"), []byte("window._CCSettings={};"), 0o644)
	return dir
}

func buildZipFromDir(t *testing.T, srcDir, topName, fileName string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := topName + "/" + filepath.ToSlash(rel)
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), fileName
}

func TestH5Upload_UpdateVersionMustIncrease(t *testing.T) {
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	initial := `{"list":[{"gameId":"g-test","name":"x","entryType":"h5","entryUrl":"h5/vitest-game1/index.html?v=1.0.0.1","sort":1,"status":"offline"}]}`
	_ = os.WriteFile(cfgPath, []byte(initial), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)
	src := writeMinimalH5Tree(t, filepath.Join(root, "src-vitest-game1"))
	zipBytes, _ := buildZipFromDir(t, src, "vitest-game1", "vitest-game1.zip")
	_, err := ProcessH5Upload(zipBytes, "vitest-game1.zip", H5UploadMeta{
		GameID: "g-test", MinigameVersion: "1.0.0.0", Name: "x", EntryType: "h5", Status: "offline",
	})
	if err != ErrH5VersionNotIncreased {
		t.Fatalf("want version error got %v", err)
	}
}

func TestBuildDownloadURL(t *testing.T) {
	got := BuildDownloadURL("game1", "1.0.0.0")
	if !strings.HasSuffix(got, "game1_v1.0.0.0.tar.gz") {
		t.Fatalf("got %q", got)
	}
}

func TestParsePackageTarGzBasename(t *testing.T) {
	dir, ver, err := ParsePackageTarGzBasename("web-mobile-planefight_v1.0.0.2.tar.gz")
	if err != nil || dir != "web-mobile-planefight" || ver != "1.0.0.2" {
		t.Fatalf("parse ok got dir=%q ver=%q err=%v", dir, ver, err)
	}
	if _, _, err := ParsePackageTarGzBasename("bad-name.tar.gz"); err == nil {
		t.Fatal("want error for bad name")
	}
}

func TestH5Upload_TarGzPackage(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	gameDir := "tar-game1"
	version := "1.0.0.3"
	src := writeMinimalH5Tree(t, filepath.Join(root, "src-"+gameDir))
	tarBytes := buildTarGzFromDir(t, src, gameDir, version)

	res, err := ProcessH5Upload(tarBytes, PackageFolderName(gameDir, version)+".tar.gz", H5UploadMeta{
		GameID: gameDir + "-id", MinigameVersion: version, Name: "Tar Game", EntryType: "h5", Status: "offline",
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
	if _, err := os.Stat(PackageTarPath(h5, gameDir, version)); err != nil {
		t.Fatal(err)
	}
}

func TestH5Upload_TarGzMisnamedZipExtension(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	gameDir := "misnamed-game"
	version := "1.0.0.3"
	src := writeMinimalH5Tree(t, filepath.Join(root, "src-"+gameDir))
	tarBytes := buildTarGzFromLiveDir(t, src, gameDir)

	_, err := ProcessH5Upload(tarBytes, gameDir+".zip", H5UploadMeta{
		GameID: gameDir + "-id", MinigameVersion: version, Name: "Misnamed", EntryType: "h5", Status: "offline",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestH5Upload_TarGzLiveInnerLayout(t *testing.T) {
	ResetAuditLogsForTests()
	t.Setenv("PUBLISH_RSYNC_DISABLED", "1")
	root := t.TempDir()
	h5 := filepath.Join(root, "h5")
	cfgPath := filepath.Join(root, "games.json")
	_ = os.WriteFile(cfgPath, []byte(`{"list":[]}`), 0o644)
	t.Setenv("GAMES_CONFIG", cfgPath)
	t.Setenv("H5_ASSETS_DIR", h5)

	gameDir := "live-layout-game"
	version := "1.0.0.4"
	src := writeMinimalH5Tree(t, filepath.Join(root, "src-"+gameDir))
	tarBytes := buildTarGzFromLiveDir(t, src, gameDir)

	res, err := ProcessH5Upload(tarBytes, PackageFolderName(gameDir, version)+".tar.gz", H5UploadMeta{
		GameID: gameDir + "-id", MinigameVersion: version, Name: "Live Layout", EntryType: "h5", Status: "offline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h5, gameDir, "index.html")); err != nil {
		t.Fatal(err)
	}
	if res.GameDirName != gameDir {
		t.Fatalf("gameDirName=%q", res.GameDirName)
	}
}

func buildTarGzFromDir(t *testing.T, srcDir, gameDirName, version string) []byte {
	t.Helper()
	packageFolder := PackageFolderName(gameDirName, version)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := packageFolder + "/" + filepath.ToSlash(rel)
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildTarGzFromLiveDir(t *testing.T, srcDir, gameDirName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := gameDirName + "/" + filepath.ToSlash(rel)
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ensure zip.ReaderAt from bytes
var _ io.ReaderAt = (*bytes.Reader)(nil)
