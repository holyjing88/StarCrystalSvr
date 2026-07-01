package service

import (
	"os"
	"path/filepath"
	"strings"

	"starcrystal/server/internal/config"
)

func LoadPublishConfig() config.PublishConfig {
	return loadPublishConfig()
}

func loadPublishConfig() config.PublishConfig {
	cfg, _, ok := config.LoadEconomyConfig()
	if ok {
		return cfg.Publish
	}
	return config.PublishConfig{}
}

func gamesJsonBackupMax() int {
	p := loadPublishConfig()
	if p.GamesJsonBackupMax > 0 {
		return p.GamesJsonBackupMax
	}
	return 10
}

func h5BackupMaxPerGame() int {
	p := loadPublishConfig()
	if p.H5BackupMax > 0 {
		return p.H5BackupMax
	}
	return 5
}

func resolveGamesConfigPath(publish config.PublishConfig) string {
	if v := strings.TrimSpace(os.Getenv("GAMES_CONFIG")); v != "" {
		return v
	}
	if v := strings.TrimSpace(publish.GamesConfigPath); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return v
	}
	return GamesConfigPath()
}

func resolveH5AssetsDir(publish config.PublishConfig) string {
	if v := strings.TrimSpace(os.Getenv("H5_ASSETS_DIR")); v != "" {
		return absPathOrRepo(v)
	}
	if v := strings.TrimSpace(publish.H5AssetsDir); v != "" {
		return absPathOrRepo(v)
	}
	return H5AssetsDir()
}

func resolveH5BackupDir(publish config.PublishConfig) string {
	if v := strings.TrimSpace(os.Getenv("H5_BACKUP_DIR")); v != "" {
		return absPathOrRepo(v)
	}
	if v := strings.TrimSpace(publish.H5BackupDir); v != "" {
		return absPathOrRepo(v)
	}
	return H5BackupDir()
}

// absPathOrRepo：绝对路径原样返回；相对路径相对仓库根（与 H5AssetsDir 一致，不依赖进程 CWD）。
func absPathOrRepo(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(repoRootDir(), p)
}
