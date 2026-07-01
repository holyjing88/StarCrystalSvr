package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrH5FolderInvalidLayout = errors.New("folder must contain exactly one top-level directory with index.html")
	ErrH5FolderEmpty         = errors.New("folder upload has no files")
)

type H5FolderEntry struct {
	RelPath string
	Data    []byte
}

func ProcessH5FolderUpload(entries []H5FolderEntry, meta H5UploadMeta) (H5UploadResult, error) {
	var out H5UploadResult
	if len(entries) == 0 {
		return out, ErrH5FolderEmpty
	}
	gameID, status, err := validateH5UploadMeta(meta)
	if err != nil {
		return out, err
	}
	gameDirName, innerFiles, err := normalizeH5FolderEntries(entries)
	if err != nil {
		return out, err
	}

	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return out, err
	}
	h5Root := H5AssetsDir()
	_ = os.MkdirAll(h5Root, 0o755)
	_ = os.MkdirAll(H5BackupDir(), 0o755)

	if err := ensureGameDirAvailable(cfg.Items, gameID, gameDirName); err != nil {
		return out, err
	}

	idx := findGameIndex(cfg.Items, gameID)
	isUpdate := idx >= 0
	if isUpdate {
		existingDir, err := ParseGameDirFromEntryURL(cfg.Items[idx].EntryURL)
		if err != nil {
			return out, err
		}
		if !strings.EqualFold(existingDir, gameDirName) {
			return out, ErrH5GameDirMismatch
		}
		curVer, err := ParseMinigameVersionFromEntryURL(cfg.Items[idx].EntryURL)
		if err != nil {
			return out, err
		}
		if compareVersion(meta.MinigameVersion, curVer) <= 0 {
			return out, ErrH5VersionNotIncreased
		}
	}

	tempDir, err := os.MkdirTemp(h5Root, ".upload-"+gameDirName+"-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(tempDir)

	extracted := filepath.Join(tempDir, gameDirName)
	if err := os.MkdirAll(extracted, 0o755); err != nil {
		return out, err
	}
	for rel, data := range innerFiles {
		outPath := filepath.Join(extracted, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return out, err
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return out, err
		}
	}

	return commitH5Deploy(h5DeployPlan{
		gameID:       gameID,
		gameDirName:  gameDirName,
		status:       status,
		meta:         meta,
		cfg:          cfg,
		idx:          idx,
		isUpdate:     isUpdate,
		h5Root:       h5Root,
		extractedDir: extracted,
	})
}

func normalizeH5FolderEntries(entries []H5FolderEntry) (gameDirName string, innerFiles map[string][]byte, err error) {
	tops := map[string]struct{}{}
	for _, entry := range entries {
		rel := normalizeFolderRelPath(entry.RelPath)
		if rel == "" {
			continue
		}
		parts := strings.Split(rel, "/")
		if len(parts) < 2 || parts[0] == "" {
			return "", nil, ErrH5FolderInvalidLayout
		}
		tops[parts[0]] = struct{}{}
	}
	if len(tops) != 1 {
		return "", nil, ErrH5FolderInvalidLayout
	}
	for k := range tops {
		gameDirName = k
	}
	innerFiles = map[string][]byte{}
	for _, entry := range entries {
		rel := normalizeFolderRelPath(entry.RelPath)
		if rel == "" {
			continue
		}
		if !strings.HasPrefix(rel, gameDirName+"/") {
			continue
		}
		inner := strings.TrimPrefix(rel, gameDirName+"/")
		if inner == "" {
			continue
		}
		innerFiles[inner] = entry.Data
	}
	if len(innerFiles) == 0 {
		return "", nil, ErrH5FolderInvalidLayout
	}
	if _, ok := innerFiles["index.html"]; !ok {
		return "", nil, fmt.Errorf("%w: missing index.html", ErrH5MissingBootstrap)
	}
	return gameDirName, innerFiles, nil
}

func normalizeFolderRelPath(raw string) string {
	rel := filepath.ToSlash(strings.TrimSpace(raw))
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.TrimPrefix(rel, "/")
	return rel
}
