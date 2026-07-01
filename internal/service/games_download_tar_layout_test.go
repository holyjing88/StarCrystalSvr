package service

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseTarGzTopLevelFolder(t *testing.T) {
	h5Root := filepath.Join("..", "..", "release", "assets", "h5")
	cfgPath := filepath.Join("..", "..", "release", "configs", "games.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skip(cfgPath, err)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)
	svc := NewGameService()
	games, err := svc.ListGames()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range games {
		if strings.ToLower(g.EntryType) != "h5" || g.DownloadURL == "" {
			continue
		}
		gameDir, version, err := parseEntryForPackage(g.EntryURL)
		if err != nil {
			t.Fatalf("%s: %v", g.GameID, err)
		}
		wantTop := gameDir + "_v" + version + "/"
		rel := strings.TrimPrefix(strings.ReplaceAll(g.DownloadURL, "\\", "/"), "h5/")
		tarPath := filepath.Join(h5Root, rel)
		top, err := tarGzFirstEntryPrefix(tarPath)
		if err != nil {
			t.Fatalf("%s: %v", g.GameID, err)
		}
		if top != wantTop {
			t.Fatalf("%s top=%q want=%q", g.GameID, top, wantTop)
		}
	}
}

func parseEntryForPackage(entryURL string) (gameDir, version string, err error) {
	// mirror cmd/pack-h5-packages
	raw := strings.TrimSpace(entryURL)
	u, err := urlParse(raw)
	if err != nil {
		return "", "", err
	}
	segs := strings.Split(strings.Trim(u.path, "/"), "/")
	for i, s := range segs {
		if strings.EqualFold(s, "index.html") && i > 0 {
			gameDir = segs[i-1]
			break
		}
	}
	if gameDir == "" {
		return "", "", os.ErrInvalid
	}
	version = strings.TrimSpace(u.query["v"])
	if version == "" {
		return "", "", os.ErrInvalid
	}
	return gameDir, version, nil
}

type simpleURL struct {
	path  string
	query map[string]string
}

func urlParse(raw string) (simpleURL, error) {
	// minimal parser for h5/foo/index.html?v=1.0.0.0
	out := simpleURL{query: map[string]string{}}
	pathPart := raw
	if i := strings.Index(raw, "?"); i >= 0 {
		pathPart = raw[:i]
		q := raw[i+1:]
		for _, kv := range strings.Split(q, "&") {
			p := strings.SplitN(kv, "=", 2)
			if len(p) == 2 {
				out.query[p[0]] = p[1]
			}
		}
	}
	out.path = pathPart
	return out, nil
}

func tarGzFirstEntryPrefix(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		return "", err
	}
	name := hdr.Name
	if !strings.Contains(name, "/") {
		return "", io.ErrUnexpectedEOF
	}
	return name[:strings.Index(name, "/")+1], nil
}
