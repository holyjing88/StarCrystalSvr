package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrGamesConfigConflict = errors.New("games config version conflict")
	ErrGameNotFound        = errors.New("game not found")
	ErrDeleteRequiresOffline = errors.New("game must be offline before delete")
)

var gamesConfigMu sync.Mutex

func GamesConfigPath() string {
	path := strings.TrimSpace(os.Getenv("GAMES_CONFIG"))
	if path == "" {
		path = "./release/configs/games.json"
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func ComputeGamesConfigVersion(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func EntryURLMissingVersion(entryURL string) bool {
	raw := strings.TrimSpace(entryURL)
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	v := strings.TrimSpace(u.Query().Get("v"))
	if v == "" {
		return true
	}
	parts := strings.Split(v, ".")
	if len(parts) != 4 {
		return true
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(strings.TrimSpace(p)); err != nil {
			return true
		}
	}
	return false
}

type loadedGamesConfig struct {
	Path          string
	Raw           []byte
	ConfigVersion string
	Items         []GameItem
}

func loadGamesConfigLocked() (loadedGamesConfig, error) {
	path := GamesConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return loadedGamesConfig{}, err
	}
	var wrapped gameConfigFile
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.List != nil {
		return loadedGamesConfig{
			Path:          path,
			Raw:           raw,
			ConfigVersion: ComputeGamesConfigVersion(raw),
			Items:         wrapped.List,
		}, nil
	}
	var plain []GameItem
	if err := json.Unmarshal(raw, &plain); err == nil {
		return loadedGamesConfig{
			Path:          path,
			Raw:           raw,
			ConfigVersion: ComputeGamesConfigVersion(raw),
			Items:         plain,
		}, nil
	}
	return loadedGamesConfig{}, errors.New("invalid games config format")
}

// LoadGamesConfigForIDIP 读取 games.json 及 SHA256 configVersion。
// LoadGamesConfigVersionForAPI returns SHA256 of games.json for GET /api/v1/games.
func LoadGamesConfigVersionForAPI() (string, error) {
	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()
	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return "", err
	}
	return cfg.ConfigVersion, nil
}

func LoadGamesConfigForIDIP() (loadedGamesConfig, error) {
	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()
	return loadGamesConfigLocked()
}

type GameUpsertPatch struct {
	GameID           string
	Name             *string
	NameEn           *string
	NameUr           *string
	Note             *string
	NoteEn           *string
	NoteUr           *string
	Status           *string
	Sort             *int
	IconLink         *string
	CoverURL         *string
	MinAppVersion    *string
	Channels         *GameChannelsPatch
	DownloadURL      *string
	PackageBytes     *int64
	DownloadSha256   *string
}

func applyPatch(dst *GameItem, patch GameUpsertPatch) {
	if patch.Name != nil {
		dst.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.NameEn != nil {
		dst.NameEn = strings.TrimSpace(*patch.NameEn)
	}
	if patch.NameUr != nil {
		dst.NameUr = strings.TrimSpace(*patch.NameUr)
	}
	if patch.Note != nil {
		dst.Note = strings.TrimSpace(*patch.Note)
	}
	if patch.NoteEn != nil {
		dst.NoteEn = strings.TrimSpace(*patch.NoteEn)
	}
	if patch.NoteUr != nil {
		dst.NoteUr = strings.TrimSpace(*patch.NoteUr)
	}
	if patch.Status != nil {
		dst.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Sort != nil {
		dst.Sort = *patch.Sort
	}
	if patch.IconLink != nil {
		dst.IconLink = strings.TrimSpace(*patch.IconLink)
	}
	if patch.CoverURL != nil {
		dst.CoverURL = strings.TrimSpace(*patch.CoverURL)
	}
	if patch.MinAppVersion != nil {
		dst.MinAppVersion = strings.TrimSpace(*patch.MinAppVersion)
	}
	if patch.Channels != nil {
		dst.Channels = gameChannelsJSON(*patch.Channels)
	}
	if patch.DownloadURL != nil {
		dst.DownloadURL = strings.TrimSpace(*patch.DownloadURL)
	}
	if patch.PackageBytes != nil {
		dst.PackageBytes = *patch.PackageBytes
	}
	if patch.DownloadSha256 != nil {
		dst.DownloadSha256 = strings.TrimSpace(*patch.DownloadSha256)
	}
}

func writeGamesConfigLocked(path string, items []GameItem) (string, error) {
	out, err := json.MarshalIndent(map[string]any{"list": items}, "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	backup := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	_ = copyFile(path, backup)
	pruneTimestampBackups(filepath.Dir(path), filepath.Base(path)+".bak.", gamesJsonBackupMax())
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", err
	}
	return ComputeGamesConfigVersion(out), nil
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

func findGameIndex(items []GameItem, gameID string) int {
	id := strings.TrimSpace(gameID)
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].GameID), id) {
			return i
		}
	}
	return -1
}

// UpsertGameItem 更新已有 gameId；expectedConfigVersion 为读盘时的 SHA256。
func UpsertGameItem(expectedConfigVersion string, patch GameUpsertPatch) (string, error) {
	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(expectedConfigVersion), cfg.ConfigVersion) {
		return "", ErrGamesConfigConflict
	}
	gameID := strings.TrimSpace(patch.GameID)
	if gameID == "" {
		return "", errors.New("gameId required")
	}
	idx := findGameIndex(cfg.Items, gameID)
	if idx < 0 {
		return "", ErrGameNotFound
	}
	applyPatch(&cfg.Items[idx], patch)
	return writeGamesConfigLocked(cfg.Path, cfg.Items)
}

// BatchUpsertGameItems 批量 upsert；任一条非法则整批不写盘。
func BatchUpsertGameItems(expectedConfigVersion string, patches []GameUpsertPatch) (string, error) {
	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(expectedConfigVersion), cfg.ConfigVersion) {
		return "", ErrGamesConfigConflict
	}
	items := append([]GameItem(nil), cfg.Items...)
	for _, patch := range patches {
		gameID := strings.TrimSpace(patch.GameID)
		if gameID == "" {
			return "", errors.New("gameId required")
		}
		idx := findGameIndex(items, gameID)
		if idx < 0 {
			return "", ErrGameNotFound
		}
		applyPatch(&items[idx], patch)
	}
	return writeGamesConfigLocked(cfg.Path, items)
}

// DeleteGameItem 删除 offline 条目；deleteH5Dir 为 true 时删 H5 目录。
func DeleteGameItem(expectedConfigVersion, gameID string, deleteH5Dir bool) (DeleteGameResult, error) {
	var result DeleteGameResult
	gamesConfigMu.Lock()
	defer gamesConfigMu.Unlock()

	cfg, err := loadGamesConfigLocked()
	if err != nil {
		return result, err
	}
	if !strings.EqualFold(strings.TrimSpace(expectedConfigVersion), cfg.ConfigVersion) {
		return result, ErrGamesConfigConflict
	}
	id := strings.TrimSpace(gameID)
	if id == "" {
		return result, errors.New("gameId required")
	}
	idx := findGameIndex(cfg.Items, id)
	if idx < 0 {
		return result, ErrGameNotFound
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Items[idx].Status), "offline") {
		return result, ErrDeleteRequiresOffline
	}
	gameDir, _ := ParseGameDirFromEntryURL(cfg.Items[idx].EntryURL)
	result.GameID = id
	result.GameDirName = gameDir

	items := append(cfg.Items[:idx], cfg.Items[idx+1:]...)
	newVer, err := writeGamesConfigLocked(cfg.Path, items)
	if err != nil {
		return result, err
	}
	result.ConfigVersion = newVer

	if deleteH5Dir && gameDir != "" {
		dir := filepath.Join(H5AssetsDir(), gameDir)
		if err := os.RemoveAll(dir); err != nil {
			return result, err
		}
		result.H5DirDeleted = true
		if err := SyncH5AssetsToCDN(); err != nil {
			return result, fmt.Errorf("cdn rsync after delete: %w", err)
		}
	}
	return result, nil
}

type DeleteGameResult struct {
	GameID        string `json:"gameId"`
	GameDirName   string `json:"gameDirName,omitempty"`
	ConfigVersion string `json:"configVersion"`
	H5DirDeleted  bool   `json:"h5DirDeleted"`
}
