package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
)

// PublishServeH5OnAPI 是否在 API 进程注册 GET /h5/*（联调 CDN 独立时关闭）。
func PublishServeH5OnAPI() bool {
	p := loadPublishConfig()
	if p.ServeH5OnAPI != nil {
		return *p.ServeH5OnAPI
	}
	if v := strings.TrimSpace(os.Getenv("DISABLE_API_H5_STATIC")); v == "1" || strings.EqualFold(v, "true") {
		return false
	}
	return true
}

func publishRsyncEnabled(pub config.PublishConfig) bool {
	if v := strings.TrimSpace(os.Getenv("PUBLISH_RSYNC_DISABLED")); v == "1" || strings.EqualFold(v, "true") {
		return false
	}
	return pub.Rsync.Enabled
}

// SyncH5AssetsToCDN 将 H5 源目录 rsync 到 publish.rsync.cdnH5Dir（需 rsync.enabled）。
func SyncH5AssetsToCDN() error {
	pub := loadPublishConfig()
	return syncH5AssetsToCDN(pub)
}

// SyncH5GameToCDN 将单个游戏的 live 目录与 {gameDir}_v{version}.tar.gz 同步到 CDN；CDN 上已有该目录时先备份。
func SyncH5GameToCDN(gameDirName, minigameVersion string) error {
	pub := loadPublishConfig()
	if !publishRsyncEnabled(pub) {
		return nil
	}
	destRoot := strings.TrimSpace(pub.Rsync.CdnH5Dir)
	if destRoot == "" {
		return fmt.Errorf("publish.rsync.cdnH5Dir is empty")
	}
	srcRoot := resolveH5AssetsDir(pub)
	gameDirName = strings.TrimSpace(gameDirName)
	if gameDirName == "" {
		return fmt.Errorf("gameDirName required for cdn sync")
	}
	localDir := filepath.Join(srcRoot, gameDirName)
	if st, err := os.Stat(localDir); err != nil || !st.IsDir() {
		return fmt.Errorf("h5 live dir not found: %s", localDir)
	}
	localTar := PackageTarPath(srcRoot, gameDirName, minigameVersion)
	if st, err := os.Stat(localTar); err != nil || st.IsDir() {
		return fmt.Errorf("h5 package not found: %s", localTar)
	}

	cdnDir := filepath.Join(destRoot, gameDirName)
	if st, err := os.Stat(cdnDir); err == nil && st.IsDir() {
		if err := backupCDNGameDir(gameDirName, cdnDir, destRoot); err != nil {
			return fmt.Errorf("cdn backup: %w", err)
		}
		pruneCDNBackupsForGame(gameDirName, destRoot, h5BackupMaxPerGame())
	}

	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}
	if err := runRsync(localDir+string(filepath.Separator), cdnDir+string(filepath.Separator)); err != nil {
		return err
	}
	if err := ensureCDNReadableTree(cdnDir); err != nil {
		return fmt.Errorf("cdn chmod after rsync: %w", err)
	}
	cdnTar := filepath.Join(destRoot, filepath.Base(localTar))
	if err := runRsync(localTar, cdnTar); err != nil {
		return err
	}
	if err := os.Chmod(cdnTar, 0o644); err != nil {
		return fmt.Errorf("cdn tar chmod: %w", err)
	}
	return nil
}

func cdnH5BackupRoot(cdnH5Dir string) string {
	return filepath.Join(filepath.Dir(cdnH5Dir), "h5_cdn_backup")
}

func backupCDNGameDir(gameDirName, cdnGameDir, cdnH5Dir string) error {
	dst := filepath.Join(cdnH5BackupRoot(cdnH5Dir), gameDirName, fmt.Sprintf("%d", time.Now().Unix()))
	return copyTree(cdnGameDir, dst)
}

func pruneCDNBackupsForGame(gameDirName, cdnH5Dir string, max int) {
	pruneTimestampSubdirs(filepath.Join(cdnH5BackupRoot(cdnH5Dir), gameDirName), max)
}

func syncH5AssetsToCDN(pub config.PublishConfig) error {
	if !publishRsyncEnabled(pub) {
		return nil
	}
	dest := strings.TrimSpace(pub.Rsync.CdnH5Dir)
	if dest == "" {
		return fmt.Errorf("publish.rsync.cdnH5Dir is empty")
	}
	src := resolveH5AssetsDir(pub)
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return fmt.Errorf("h5 source not found: %s", src)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	srcArg := filepath.ToSlash(src)
	if !strings.HasSuffix(srcArg, "/") {
		srcArg += "/"
	}
	destArg := filepath.ToSlash(dest)
	if !strings.HasSuffix(destArg, "/") {
		destArg += "/"
	}
	if err := runRsync(srcArg, destArg); err != nil {
		return err
	}
	return ensureCDNReadableTree(dest)
}

// ensureCDNReadableTree 保证 nginx（非 owner）可读：目录 755、文件 644。
func ensureCDNReadableTree(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, 0o755)
		}
		return os.Chmod(path, 0o644)
	})
}

func runRsync(srcArg, destArg string) error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found in PATH")
	}
	cmd := exec.Command("rsync", "-a", "--chmod=Du=rwx,go=rx,Fu=rw,go=r", srcArg, destArg)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logger.Info(logger.TopicAPI, "[cdn_sync] rsync %s -> %s", srcArg, destArg)
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("rsync failed: %s", msg)
	}
	logger.Info(logger.TopicAPI, "[cdn_sync] ok dest=%s", destArg)
	return nil
}
