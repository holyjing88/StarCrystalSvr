package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrH5ZipNameMismatch     = errors.New("zip file name must match top-level directory")
	ErrH5ZipInvalidLayout    = errors.New("zip must contain exactly one top-level directory")
	ErrH5MissingBootstrap    = errors.New("package missing required bootstrap files")
	ErrH5TarNameMismatch     = errors.New("tar.gz file name must be {gameDirName}_v{minigameVersion}.tar.gz")
	ErrH5TarInvalidLayout    = errors.New("tar.gz must contain exactly one top-level directory")
	ErrH5TarVersionMismatch  = errors.New("meta minigameVersion does not match tar.gz file name")
	ErrH5UnsupportedPackage  = errors.New("unsupported package format: use .zip or .tar.gz")
	ErrH5GameDirConflict     = errors.New("gameDirName already used by another gameId")
	ErrH5GameDirMismatch     = errors.New("zip directory does not match existing entryUrl gameDirName")
	ErrH5VersionNotIncreased = errors.New("minigameVersion must be greater than current")
	ErrH5PackageTooLarge     = errors.New("package exceeds 50MB limit")
)

type H5UploadMeta struct {
	GameID         string          `json:"gameId"`
	MinigameVersion string          `json:"minigameVersion"`
	Name           string          `json:"name"`
	NameEn         string          `json:"nameEn,omitempty"`
	NameUr         string          `json:"nameUr,omitempty"`
	Note           string          `json:"note,omitempty"`
	NoteEn         string          `json:"noteEn,omitempty"`
	NoteUr         string          `json:"noteUr,omitempty"`
	EntryType      string          `json:"entryType"`
	Status         string          `json:"status"`
	Sort           int             `json:"sort"`
	IconLink       string          `json:"iconLink,omitempty"`
	CoverURL       string          `json:"coverUrl,omitempty"`
	MinAppVersion  string          `json:"minAppVersion,omitempty"`
	Channels       json.RawMessage `json:"channels,omitempty"`
}

type H5UploadResult struct {
	GameID         string `json:"gameId"`
	GameDirName    string `json:"gameDirName"`
	ConfigVersion  string `json:"configVersion"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	PackageBytes   int64  `json:"packageBytes,omitempty"`
	DownloadSha256 string `json:"downloadSha256,omitempty"`
	MinigameVersion string `json:"minigameVersion"`
}

func ProcessH5Upload(data []byte, fileName string, meta H5UploadMeta) (H5UploadResult, error) {
	var out H5UploadResult
	if len(data) == 0 {
		return out, fmt.Errorf("upload file is empty")
	}
	kind, err := detectH5PackageKind(data, fileName)
	if err != nil {
		return out, err
	}
	switch kind {
	case h5PackageTarGz:
		return processH5UploadTarGz(data, fileName, meta)
	default:
		return processH5UploadZip(bytes.NewReader(data), int64(len(data)), fileName, meta)
	}
}

type h5PackageKind int

const (
	h5PackageZip h5PackageKind = iota
	h5PackageTarGz
)

func detectH5PackageKind(data []byte, fileName string) (h5PackageKind, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return h5PackageTarGz, nil
	}
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' {
		return h5PackageZip, nil
	}
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return h5PackageTarGz, nil
	case strings.HasSuffix(lower, ".zip"):
		return h5PackageZip, nil
	}
	return 0, ErrH5UnsupportedPackage
}

// parseH5TarGzUploadName 从文件名（可含 .zip/.tar.gz）与 meta 解析 gameDir 与版本。
func parseH5TarGzUploadName(fileName string, meta H5UploadMeta) (gameDirName, fileVersion string, err error) {
	if gameDirName, fileVersion, err = ParsePackageTarGzBasename(fileName); err == nil {
		return gameDirName, fileVersion, nil
	}
	stem := archiveBaseStem(fileName)
	if stem == "" {
		return "", "", ErrH5TarNameMismatch
	}
	if idx := strings.LastIndex(stem, "_v"); idx > 0 {
		gameDirName = stem[:idx]
		fileVersion = stem[idx+2:]
		if IsValidMinigameVersion(fileVersion) {
			return gameDirName, fileVersion, nil
		}
	}
	metaVer := strings.TrimSpace(meta.MinigameVersion)
	if stem != "" && IsValidMinigameVersion(metaVer) {
		return stem, metaVer, nil
	}
	return "", "", ErrH5TarNameMismatch
}

func archiveBaseStem(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	lower := strings.ToLower(base)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			return base[:len(base)-len(ext)]
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(base, ".gz"), ".tar")
}

func resolveTarGzInnerPrefix(topDir, gameDirName, fileVersion string) (string, error) {
	packageFolder := PackageFolderName(gameDirName, fileVersion)
	if strings.EqualFold(topDir, packageFolder) || strings.EqualFold(topDir, gameDirName) {
		return topDir, nil
	}
	return "", ErrH5TarInvalidLayout
}

func validateH5UploadMeta(meta H5UploadMeta) (gameID, status string, err error) {
	gameID = strings.TrimSpace(meta.GameID)
	if gameID == "" {
		return "", "", fmt.Errorf("gameId required")
	}
	if !IsValidMinigameVersion(meta.MinigameVersion) {
		return "", "", fmt.Errorf("invalid minigameVersion")
	}
	if strings.TrimSpace(meta.Name) == "" {
		return "", "", fmt.Errorf("name required")
	}
	if !strings.EqualFold(strings.TrimSpace(meta.EntryType), "h5") {
		return "", "", fmt.Errorf("entryType must be h5")
	}
	status = strings.TrimSpace(meta.Status)
	if status != "online" && status != "offline" {
		return "", "", fmt.Errorf("status must be online or offline")
	}
	return gameID, status, nil
}

func processH5UploadZip(zipReader io.ReaderAt, zipSize int64, zipFileName string, meta H5UploadMeta) (H5UploadResult, error) {
	var out H5UploadResult
	gameID, status, err := validateH5UploadMeta(meta)
	if err != nil {
		return out, err
	}

	zr, err := zip.NewReader(zipReader, zipSize)
	if err != nil {
		return out, fmt.Errorf("invalid zip (请确认上传的是 zip 压缩包；tar.gz 请用 .tar.gz 扩展名或重新打包): %w", err)
	}
	topDir, err := detectZipTopDir(zr)
	if err != nil {
		return out, err
	}
	zipBase := strings.TrimSuffix(filepath.Base(zipFileName), filepath.Ext(zipFileName))
	if !strings.EqualFold(zipBase, topDir) {
		return out, ErrH5ZipNameMismatch
	}
	if err := validateBootstrapFiles(zr, topDir); err != nil {
		return out, err
	}

	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return out, err
	}
	if err := ensureGameDirAvailable(cfg.Items, gameID, topDir); err != nil {
		return out, err
	}
	plan, _, _, err := prepareH5DeployFromConfig(cfg, gameID, topDir, status, meta)
	if err != nil {
		return out, err
	}

	tempDir, err := os.MkdirTemp(plan.h5Root, ".upload-"+topDir+"-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(tempDir)

	if err := extractZipToDir(zr, topDir, tempDir); err != nil {
		return out, err
	}
	extracted := filepath.Join(tempDir, topDir)
	if st, err := os.Stat(extracted); err != nil || !st.IsDir() {
		return out, ErrH5ZipInvalidLayout
	}
	plan.extractedDir = extracted
	return commitH5Deploy(plan)
}

func processH5UploadTarGz(data []byte, tarFileName string, meta H5UploadMeta) (H5UploadResult, error) {
	var out H5UploadResult
	gameID, status, err := validateH5UploadMeta(meta)
	if err != nil {
		return out, err
	}
	gameDirName, fileVersion, err := parseH5TarGzUploadName(tarFileName, meta)
	if err != nil {
		return out, ErrH5TarNameMismatch
	}
	if !strings.EqualFold(strings.TrimSpace(meta.MinigameVersion), fileVersion) {
		return out, ErrH5TarVersionMismatch
	}
	topDir, err := detectTarGzTopDir(data)
	if err != nil {
		return out, err
	}
	tarPrefix, err := resolveTarGzInnerPrefix(topDir, gameDirName, fileVersion)
	if err != nil {
		return out, err
	}
	if err := validateBootstrapFilesTarGz(data, tarPrefix); err != nil {
		return out, err
	}
	if int64(len(data)) > MaxH5PackageBytes {
		return out, ErrH5PackageTooLarge
	}

	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return out, err
	}
	if err := ensureGameDirAvailable(cfg.Items, gameID, gameDirName); err != nil {
		return out, err
	}
	plan, _, _, err := prepareH5DeployFromConfig(cfg, gameID, gameDirName, status, meta)
	if err != nil {
		return out, err
	}

	tempDir, err := os.MkdirTemp(plan.h5Root, ".upload-"+gameDirName+"-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(tempDir)

	if err := extractTarGzToLiveDir(data, tarPrefix, gameDirName, tempDir); err != nil {
		return out, err
	}
	extracted := filepath.Join(tempDir, gameDirName)
	if st, err := os.Stat(extracted); err != nil || !st.IsDir() {
		return out, ErrH5TarInvalidLayout
	}
	plan.extractedDir = extracted
	return commitH5Deploy(plan)
}

func ensureGameDirAvailable(items []GameItem, gameID, topDir string) error {
	for _, g := range items {
		if strings.EqualFold(strings.TrimSpace(g.GameID), gameID) {
			continue
		}
		dir, err := ParseGameDirFromEntryURL(g.EntryURL)
		if err != nil {
			continue
		}
		if strings.EqualFold(dir, topDir) {
			return ErrH5GameDirConflict
		}
	}
	return nil
}

func detectZipTopDir(zr *zip.Reader) (string, error) {
	tops := map[string]struct{}{}
	for _, f := range zr.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		if parts[0] == "" {
			continue
		}
		tops[parts[0]] = struct{}{}
	}
	if len(tops) != 1 {
		return "", ErrH5ZipInvalidLayout
	}
	for k := range tops {
		return k, nil
	}
	return "", ErrH5ZipInvalidLayout
}

func validateBootstrapFiles(zr *zip.Reader, topDir string) error {
	required := []string{
		topDir + "/index.html",
	}
	set := map[string]struct{}{}
	for _, f := range zr.File {
		set[strings.TrimPrefix(filepath.ToSlash(f.Name), "./")] = struct{}{}
	}
	for _, rel := range required {
		if _, ok := set[rel]; !ok {
			return fmt.Errorf("%w: missing %s", ErrH5MissingBootstrap, rel)
		}
	}
	return nil
}

func extractZipToDir(zr *zip.Reader, topDir, destRoot string) error {
	for _, f := range zr.File {
		rel := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if !strings.HasPrefix(rel, topDir+"/") && rel != topDir {
			continue
		}
		inner := strings.TrimPrefix(rel, topDir+"/")
		if inner == "" {
			continue
		}
		outPath := filepath.Join(destRoot, topDir, filepath.FromSlash(inner))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(outPath)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(w, rc)
		_ = rc.Close()
		_ = w.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func detectTarGzTopDir(data []byte) (string, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid tar.gz: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	tops := map[string]struct{}{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid tar.gz: %w", err)
		}
		name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		if parts[0] == "" {
			continue
		}
		tops[parts[0]] = struct{}{}
	}
	if len(tops) != 1 {
		return "", ErrH5TarInvalidLayout
	}
	for k := range tops {
		return k, nil
	}
	return "", ErrH5TarInvalidLayout
}

func validateBootstrapFilesTarGz(data []byte, packageFolder string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid tar.gz: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	required := packageFolder + "/index.html"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar.gz: %w", err)
		}
		name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if name == required {
			return nil
		}
	}
	return fmt.Errorf("%w: missing %s", ErrH5MissingBootstrap, required)
}

func extractTarGzToLiveDir(data []byte, packageFolder, gameDirName, destRoot string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid tar.gz: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	liveRoot := filepath.Join(destRoot, gameDirName)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if !strings.HasPrefix(rel, packageFolder+"/") && rel != packageFolder {
			continue
		}
		inner := strings.TrimPrefix(rel, packageFolder+"/")
		if inner == "" {
			continue
		}
		outPath := filepath.Join(liveRoot, filepath.FromSlash(inner))
		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		w, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, tr); err != nil {
			_ = w.Close()
			return err
		}
		_ = w.Close()
	}
	return nil
}

func backupH5Dir(gameDirName, srcDir string) error {
	dst := filepath.Join(H5BackupDir(), gameDirName, fmt.Sprintf("%d", time.Now().Unix()))
	return copyTree(srcDir, dst)
}

func restoreH5DirBackup(gameDirName, destDir string) error {
	backupRoot := filepath.Join(H5BackupDir(), gameDirName)
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no backup")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	latest := filepath.Join(backupRoot, entries[0].Name())
	return copyTree(latest, destDir)
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, in, 0o644)
	})
}

func ParseH5UploadMetaJSON(raw []byte) (H5UploadMeta, error) {
	var meta H5UploadMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}
