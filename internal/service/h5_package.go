package service

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func ParseGameDirFromEntryURL(entryURL string) (string, error) {
	raw := strings.TrimSpace(entryURL)
	if raw == "" {
		return "", fmt.Errorf("empty entryUrl")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, s := range segs {
		if strings.EqualFold(s, "index.html") && i > 0 {
			return segs[i-1], nil
		}
	}
	return "", fmt.Errorf("cannot parse gameDirName from entryUrl")
}

func ParseMinigameVersionFromEntryURL(entryURL string) (string, error) {
	raw := strings.TrimSpace(entryURL)
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(u.Query().Get("v"))
	if v == "" {
		return "", fmt.Errorf("missing v= in entryUrl")
	}
	return v, nil
}

func BuildEntryURL(gameDirName, minigameVersion string) string {
	return fmt.Sprintf("h5/%s/index.html?v=%s", gameDirName, minigameVersion)
}

func BuildDownloadURL(gameDirName, minigameVersion string) string {
	name := PackageFolderName(gameDirName, minigameVersion) + ".tar.gz"
	return path.Join("h5", name)
}

func PackageFolderName(gameDirName, minigameVersion string) string {
	return gameDirName + "_v" + minigameVersion
}

// ParsePackageTarGzBasename parses {gameDirName}_v{minigameVersion}.tar.gz file names.
func ParsePackageTarGzBasename(fileName string) (gameDirName, minigameVersion string, err error) {
	base := filepath.Base(strings.TrimSpace(fileName))
	lower := strings.ToLower(base)
	if !strings.HasSuffix(lower, ".tar.gz") {
		return "", "", fmt.Errorf("not a tar.gz file")
	}
	stem := base[:len(base)-len(".tar.gz")]
	idx := strings.LastIndex(stem, "_v")
	if idx <= 0 {
		return "", "", fmt.Errorf("tar.gz name must be {gameDirName}_v{minigameVersion}.tar.gz")
	}
	gameDirName = stem[:idx]
	minigameVersion = stem[idx+2:]
	if !IsValidMinigameVersion(minigameVersion) {
		return "", "", fmt.Errorf("invalid minigameVersion in tar.gz file name")
	}
	if !strings.EqualFold(stem, PackageFolderName(gameDirName, minigameVersion)) {
		return "", "", fmt.Errorf("tar.gz name must be {gameDirName}_v{minigameVersion}.tar.gz")
	}
	return gameDirName, minigameVersion, nil
}

func PackH5TarGz(sourceDir, packageFolder, outPath string) error {
	tmp := outPath + ".tmp"
	if err := packTarGz(sourceDir, packageFolder, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, outPath)
}

func packTarGz(sourceDir, topFolder, outPath string) error {
	f, err := os.Create(outPath)
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
		_ = os.Remove(outPath)
		return err
	}
	sort.Strings(files)
	for _, abs := range files {
		rel, err := filepath.Rel(sourceDir, abs)
		if err != nil {
			return err
		}
		name := topFolder + "/" + filepath.ToSlash(rel)
		if err := addTarFile(tw, abs, name); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			_ = f.Close()
			_ = os.Remove(outPath)
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gw.Close()
		_ = f.Close()
		_ = os.Remove(outPath)
		return err
	}
	if err := gw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(outPath)
		return err
	}
	return f.Close()
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

func FileSHA256Hex(path string) (string, error) {
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

func PackageTarPath(h5Root, gameDirName, minigameVersion string) string {
	name := PackageFolderName(gameDirName, minigameVersion) + ".tar.gz"
	return filepath.Join(h5Root, name)
}

func PruneOldPackageTars(h5Root, gameDirName string, keep int) error {
	if keep <= 0 {
		return nil
	}
	prefix := gameDirName + "_v"
	var matches []string
	entries, err := os.ReadDir(h5Root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".tar.gz") {
			matches = append(matches, filepath.Join(h5Root, name))
		}
	}
	if len(matches) <= keep {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return matches[i] > matches[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	for _, p := range matches[keep:] {
		_ = os.Remove(p)
	}
	return nil
}

func IsValidMinigameVersion(v string) bool {
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if _, err := parseIntPart(p); err != nil {
			return false
		}
	}
	return true
}

func parseIntPart(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not digit")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
