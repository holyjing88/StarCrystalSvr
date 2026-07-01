package service

import (
	"os"
	"path/filepath"
	"strings"
)

const MaxH5PackageBytes int64 = 52_428_800

// repoRootDir 为 release/configs 上两级（含 release/ 与 release_h5/ 的仓库根）。
func repoRootDir() string {
	return filepath.Clean(filepath.Join(filepath.Dir(GamesConfigPath()), "..", ".."))
}

func H5AssetsDir() string {
	if v := strings.TrimSpace(os.Getenv("H5_ASSETS_DIR")); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return v
	}
	return filepath.Join(repoRootDir(), "release_h5")
}

func H5BackupDir() string {
	if v := strings.TrimSpace(os.Getenv("H5_BACKUP_DIR")); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return v
	}
	return filepath.Join(repoRootDir(), "release_h5_backup")
}
