package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type h5DeployPlan struct {
	gameID       string
	gameDirName  string
	status       string
	meta         H5UploadMeta
	cfg          loadedGamesConfig
	idx          int
	isUpdate     bool
	h5Root       string
	extractedDir string
}

func commitH5Deploy(plan h5DeployPlan) (H5UploadResult, error) {
	var out H5UploadResult
	gameDirName := plan.gameDirName
	h5Root := plan.h5Root
	destDir := filepath.Join(h5Root, gameDirName)

	var backedUp bool
	if _, err := os.Stat(destDir); err == nil {
		if err := backupH5Dir(gameDirName, destDir); err != nil {
			return out, err
		}
		pruneH5BackupsForGame(gameDirName, h5BackupMaxPerGame())
		backedUp = true
		if err := os.RemoveAll(destDir); err != nil {
			return out, err
		}
	}
	if err := os.Rename(plan.extractedDir, destDir); err != nil {
		if backedUp {
			_ = restoreH5DirBackup(gameDirName, destDir)
		}
		return out, err
	}

	packageFolder := PackageFolderName(gameDirName, plan.meta.MinigameVersion)
	tarPath := PackageTarPath(h5Root, gameDirName, plan.meta.MinigameVersion)
	if err := PackH5TarGz(destDir, packageFolder, tarPath); err != nil {
		_ = os.RemoveAll(destDir)
		if backedUp {
			_ = restoreH5DirBackup(gameDirName, destDir)
		}
		return out, err
	}
	fi, err := os.Stat(tarPath)
	if err != nil {
		return out, err
	}
	if fi.Size() > MaxH5PackageBytes {
		_ = os.Remove(tarPath)
		_ = os.RemoveAll(destDir)
		if backedUp {
			_ = restoreH5DirBackup(gameDirName, destDir)
		}
		return out, ErrH5PackageTooLarge
	}
	sha, err := FileSHA256Hex(tarPath)
	if err != nil {
		return out, err
	}
	_ = PruneOldPackageTars(h5Root, gameDirName, 5)

	entryURL := BuildEntryURL(gameDirName, plan.meta.MinigameVersion)
	downloadURL := BuildDownloadURL(gameDirName, plan.meta.MinigameVersion)
	item := GameItem{
		GameID:         plan.gameID,
		Name:           strings.TrimSpace(plan.meta.Name),
		NameEn:         strings.TrimSpace(plan.meta.NameEn),
		NameUr:         strings.TrimSpace(plan.meta.NameUr),
		Note:           strings.TrimSpace(plan.meta.Note),
		NoteEn:         strings.TrimSpace(plan.meta.NoteEn),
		NoteUr:         strings.TrimSpace(plan.meta.NoteUr),
		IconLink:       strings.TrimSpace(plan.meta.IconLink),
		CoverURL:       strings.TrimSpace(plan.meta.CoverURL),
		EntryType:      "h5",
		EntryURL:       entryURL,
		DownloadURL:    downloadURL,
		PackageBytes:   fi.Size(),
		DownloadSha256: sha,
		MinAppVersion:  strings.TrimSpace(plan.meta.MinAppVersion),
		Sort:           plan.meta.Sort,
		Status:         plan.status,
	}
	if len(plan.meta.Channels) > 0 {
		var ch GameChannelsPatch
		if err := json.Unmarshal(plan.meta.Channels, &ch); err == nil {
			item.Channels = gameChannelsJSON(ch)
		}
	}

	if err := SyncH5GameToCDN(gameDirName, plan.meta.MinigameVersion); err != nil {
		_ = os.Remove(tarPath)
		_ = os.RemoveAll(destDir)
		if backedUp {
			_ = restoreH5DirBackup(gameDirName, destDir)
		}
		return out, fmt.Errorf("cdn sync after upload: %w", err)
	}

	cfg := plan.cfg
	if plan.isUpdate {
		cfg.Items[plan.idx] = item
	} else {
		cfg.Items = append(cfg.Items, item)
	}
	newVer, err := writeGamesConfigLocked(cfg.Path, cfg.Items)
	if err != nil {
		_ = os.Remove(tarPath)
		_ = os.RemoveAll(destDir)
		if backedUp {
			_ = restoreH5DirBackup(gameDirName, destDir)
		}
		return out, err
	}

	out = H5UploadResult{
		GameID:          plan.gameID,
		GameDirName:     gameDirName,
		ConfigVersion:   newVer,
		DownloadURL:     downloadURL,
		PackageBytes:    fi.Size(),
		DownloadSha256:  sha,
		MinigameVersion: plan.meta.MinigameVersion,
	}
	return out, nil
}

func prepareH5DeployFromConfig(
	cfg loadedGamesConfig,
	gameID, gameDirName, status string,
	meta H5UploadMeta,
) (h5DeployPlan, int, bool, error) {
	var plan h5DeployPlan
	idx := findGameIndex(cfg.Items, gameID)
	isUpdate := idx >= 0
	if isUpdate {
		existingDir, err := ParseGameDirFromEntryURL(cfg.Items[idx].EntryURL)
		if err != nil {
			return plan, idx, isUpdate, err
		}
		if !strings.EqualFold(existingDir, gameDirName) {
			return plan, idx, isUpdate, ErrH5GameDirMismatch
		}
		curVer, err := ParseMinigameVersionFromEntryURL(cfg.Items[idx].EntryURL)
		if err != nil {
			return plan, idx, isUpdate, err
		}
		if compareVersion(meta.MinigameVersion, curVer) <= 0 {
			return plan, idx, isUpdate, ErrH5VersionNotIncreased
		}
	}
	h5Root := H5AssetsDir()
	_ = os.MkdirAll(h5Root, 0o755)
	_ = os.MkdirAll(H5BackupDir(), 0o755)
	plan = h5DeployPlan{
		gameID:      gameID,
		gameDirName: gameDirName,
		status:      status,
		meta:        meta,
		cfg:         cfg,
		idx:         idx,
		isUpdate:    isUpdate,
		h5Root:      h5Root,
	}
	return plan, idx, isUpdate, nil
}
