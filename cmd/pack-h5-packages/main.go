// pack-h5-packages：从 release/assets/h5/{gameDirName} 打 tar.gz，并回写 games.json 三字段。
// 约定：tar 内唯一顶层目录 {gameDirName}_v{minigameVersion}；与 Unity H5GamePackageDownloadValidator 一致。
//
// 用法（在 server 根目录）：
//
//	go run ./cmd/pack-h5-packages -write
//	go run ./cmd/pack-h5-packages -game g001 -write
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type gameConfigFile struct {
	List []gameItem `json:"list"`
}

type gameItem struct {
	GameID         string `json:"gameId"`
	Name           string `json:"name,omitempty"`
	NameEn         string `json:"nameEn,omitempty"`
	NameUr         string `json:"nameUr,omitempty"`
	Note           string `json:"note,omitempty"`
	NoteEn         string `json:"noteEn,omitempty"`
	NoteUr         string `json:"noteUr,omitempty"`
	IconLink       string `json:"iconLink,omitempty"`
	CoverURL       string `json:"coverUrl,omitempty"`
	EntryType      string `json:"entryType"`
	EntryURL       string `json:"entryUrl"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	PackageBytes   int64  `json:"packageBytes,omitempty"`
	DownloadSha256 string `json:"downloadSha256,omitempty"`
	MinAppVersion  string `json:"minAppVersion,omitempty"`
	Sort           int    `json:"sort"`
	Status         string `json:"status,omitempty"`
	RewardRuleID   string `json:"rewardRuleId,omitempty"`
	Channels       any    `json:"channels,omitempty"`
}

func main() {
	gamesPath := flag.String("games", "release/configs/games.json", "games.json path")
	h5Dir := flag.String("h5dir", "release/assets/h5", "H5 assets root")
	onlyGame := flag.String("game", "", "only pack one gameId (optional)")
	write := flag.Bool("write", false, "write games.json back with downloadUrl/packageBytes/downloadSha256")
	flag.Parse()

	raw, err := os.ReadFile(*gamesPath)
	if err != nil {
		fatal(err)
	}

	var wrapped gameConfigFile
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		fatal(err)
	}
	if len(wrapped.List) == 0 {
		fatal(fmt.Errorf("empty games list in %s", *gamesPath))
	}

	changed := 0
	for i := range wrapped.List {
		g := &wrapped.List[i]
		if strings.ToLower(strings.TrimSpace(g.EntryType)) != "h5" {
			continue
		}
		if *onlyGame != "" && !strings.EqualFold(g.GameID, *onlyGame) {
			continue
		}
		gameDir, version, err := parseEntryURL(g.EntryURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", g.GameID, err)
			continue
		}
		packageFolder := gameDir + "_v" + version
		srcDir := filepath.Join(*h5Dir, gameDir)
		if st, err := os.Stat(srcDir); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "skip %s: missing source dir %s\n", g.GameID, srcDir)
			continue
		}
		outName := packageFolder + ".tar.gz"
		outPath := filepath.Join(*h5Dir, outName)
		if err := packTarGz(srcDir, packageFolder, outPath); err != nil {
			fatal(fmt.Errorf("%s: %w", g.GameID, err))
		}
		fi, err := os.Stat(outPath)
		if err != nil {
			fatal(err)
		}
		sum, err := fileSHA256Hex(outPath)
		if err != nil {
			fatal(err)
		}
		downloadURL := path.Join("assets/h5", outName)
		downloadURL = strings.ReplaceAll(downloadURL, "\\", "/")

		fmt.Printf("%s  gameDir=%s  version=%s  bytes=%d  sha256=%s  url=%s\n",
			g.GameID, gameDir, version, fi.Size(), sum, downloadURL)

		if *write {
			g.DownloadURL = downloadURL
			g.PackageBytes = fi.Size()
			g.DownloadSha256 = sum
			changed++
		}
	}

	if *write && changed > 0 {
		out, err := json.MarshalIndent(map[string]any{"list": wrapped.List}, "", "  ")
		if err != nil {
			fatal(err)
		}
		out = append(out, '\n')
		backup := *gamesPath + ".bak." + fmt.Sprintf("%d", os.Getpid())
		if err := copyFile(*gamesPath, backup); err == nil {
			fmt.Println("backup:", backup)
		}
		if err := os.WriteFile(*gamesPath, out, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("updated %s (%d games)\n", *gamesPath, changed)
	}
}

func parseEntryURL(entryURL string) (gameDirName, minigameVersion string, err error) {
	raw := strings.TrimSpace(entryURL)
	if raw == "" {
		return "", "", fmt.Errorf("empty entryUrl")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	gameDirName = ""
	for i, s := range segs {
		if strings.EqualFold(s, "index.html") && i > 0 {
			gameDirName = segs[i-1]
			break
		}
	}
	if gameDirName == "" {
		return "", "", fmt.Errorf("cannot parse gameDirName from %q", entryURL)
	}
	v := strings.TrimSpace(u.Query().Get("v"))
	if v == "" {
		return "", "", fmt.Errorf("missing v= in entryUrl")
	}
	return gameDirName, v, nil
}

func packTarGz(sourceDir, topFolder, outPath string) error {
	tmp := outPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	var files []string
	err = filepath.Walk(sourceDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		_ = tw.Close()
		_ = gw.Close()
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	sort.Strings(files)

	for _, abs := range files {
		rel, err := filepath.Rel(sourceDir, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := topFolder + "/" + rel
		if err := addTarFile(tw, abs, name); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}

	if err := tw.Close(); err != nil {
		_ = gw.Close()
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := gw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, outPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func addTarFile(tw *tar.Writer, absPath, tarName string) error {
	fi, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = tarName
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	r, err := os.Open(absPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(tw, r)
	_ = r.Close()
	return err
}

func fileSHA256Hex(path string) (string, error) {
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

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pack-h5-packages:", err)
	os.Exit(1)
}
