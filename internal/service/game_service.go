package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"starcrystal/server/internal/logger"
)

// ListGamesForClient filters online games for player API; skips entries with missing/invalid entryUrl v= (§2.6 K3).

var gamesConfigPathLogged sync.Once

type GameService struct{}

// 与 configs/games.json 及 API 的 camelCase 一致，否则 json.Unmarshal 读不到小写 key（如 note、gameId）
type GameItem struct {
	GameID        string `json:"gameId"`
	Name          string `json:"name"`
	NameEn        string `json:"nameEn,omitempty"`
	NameUr        string `json:"nameUr,omitempty"`
	Note          string `json:"note,omitempty"`
	NoteEn        string `json:"noteEn,omitempty"`
	NoteUr        string `json:"noteUr,omitempty"`
	IconLink      string `json:"iconLink,omitempty"`
	CoverURL      string `json:"coverUrl,omitempty"`
	EntryType     string `json:"entryType"`
	EntryURL         string `json:"entryUrl"`
	DownloadURL      string `json:"downloadUrl,omitempty"`
	PackageBytes     int64  `json:"packageBytes,omitempty"`
	DownloadSha256   string `json:"downloadSha256,omitempty"`
	MinAppVersion    string `json:"minAppVersion,omitempty"`
	Sort          int    `json:"sort"`
	RewardRuleID  string `json:"rewardRuleId,omitempty"`
	Status        string `json:"status,omitempty"`
	// Channels 为空或省略：所有渠道可见。
	// JSON 可为字符串 "ChannelType_A-ChannelType_B"（连字符表示多选一）或数组；数组元素内仍可用 "-" 连接。
	Channels gameChannelsJSON `json:"channels,omitempty"`
}

func NewGameService() *GameService {
	return &GameService{}
}

type gameConfigFile struct {
	List []GameItem `json:"list"`
}

func (s *GameService) ListGames() ([]GameItem, error) {
	path := strings.TrimSpace(os.Getenv("GAMES_CONFIG"))
	if path == "" {
		path = "./release/configs/games.json"
	}
	absPath := path
	if p, err := filepath.Abs(path); err == nil {
		absPath = p
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wrapped gameConfigFile
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.List) > 0 {
		gamesConfigPathLogged.Do(func() {
			logger.Info(logger.TopicAPI, "[games_config] loaded list=%d path=%s", len(wrapped.List), absPath)
		})
		return wrapped.List, nil
	}

	var plain []GameItem
	if err := json.Unmarshal(body, &plain); err == nil && len(plain) > 0 {
		gamesConfigPathLogged.Do(func() {
			logger.Info(logger.TopicAPI, "[games_config] loaded plain=%d path=%s", len(plain), absPath)
		})
		return plain, nil
	}
	return nil, errors.New("invalid games config format: expect {\"list\": [...]} or [...]")
}

func (s *GameService) ListGamesForClient(appVersion, platform, clientChannel string) ([]GameItem, error) {
	all, err := s.ListGames()
	if err != nil {
		return nil, err
	}

	filtered := make([]GameItem, 0, len(all))
	for _, g := range all {
		if strings.ToLower(strings.TrimSpace(g.Status)) != "online" {
			continue
		}
		// platform is reserved for per-platform filtering in future iterations.
		_ = platform
		if g.MinAppVersion != "" && compareVersion(appVersion, g.MinAppVersion) < 0 {
			continue
		}
		if !gameMatchesClientChannel(g, clientChannel) {
			continue
		}
		if EntryURLMissingVersion(g.EntryURL) {
			logger.Error(logger.TopicAPI, "[games_list] skip gameId=%s: missing or invalid entryUrl v=", g.GameID)
			continue
		}
		filtered = append(filtered, g)
	}
	return filtered, nil
}

// gameMatchesClientChannel：配置未声明 channels 则全渠道可见。
// 客户端 channel 仅单个渠道类型（与配置中任一规范化 token 相同则命中）。
// 配置声明了 channels 但请求未带 channel 参数：不匹配（见 gameChannelSingleClientMatch）。
func gameMatchesClientChannel(g GameItem, clientChannel string) bool {
	return gameChannelSingleClientMatch(clientChannel, []string(g.Channels))
}

func compareVersion(current, target string) int {
	currentParts := normalizeVersionParts(current)
	targetParts := normalizeVersionParts(target)
	maxLen := len(currentParts)
	if len(targetParts) > maxLen {
		maxLen = len(targetParts)
	}

	for i := 0; i < maxLen; i++ {
		var cv, tv int
		if i < len(currentParts) {
			cv = currentParts[i]
		}
		if i < len(targetParts) {
			tv = targetParts[i]
		}
		if cv < tv {
			return -1
		}
		if cv > tv {
			return 1
		}
	}
	return 0
}

func normalizeVersionParts(v string) []int {
	parts := strings.Split(strings.TrimSpace(v), ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}
