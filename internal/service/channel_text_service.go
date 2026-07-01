package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ChannelTextService struct{}

type ChannelTextItem struct {
	Key  string
	Text string
}

type channelTextConfig struct {
	Channels map[string]map[string]localizedText `json:"channels"`
}

type localizedText struct {
	EN string `json:"en"`
	UR string `json:"ur"`
	ZH string `json:"zh"`
}

func NewChannelTextService() *ChannelTextService {
	return &ChannelTextService{}
}

func (s *ChannelTextService) Resolve(channelType, language string, keys []string) ([]ChannelTextItem, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("keys is empty")
	}

	ch := normalizeChannelType(channelType)
	lang := normalizeLanguage(language)
	channelMap := cfg.Channels[ch]
	fallbackMap := cfg.Channels["CompanyOwned"]

	out := make([]ChannelTextItem, 0, len(keys))
	for _, key := range keys {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		text := pickLocalizedText(channelMap[k], lang)
		if text == "" {
			text = pickLocalizedText(fallbackMap[k], lang)
		}
		if text == "" {
			continue
		}
		out = append(out, ChannelTextItem{Key: k, Text: text})
	}
	return out, nil
}

func resolveChannelTextsConfigPath(explicit string) string {
	if p := filepath.Clean(strings.TrimSpace(explicit)); p != "" && p != "." {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		for _, rel := range []string{
			"release/configs/channel_texts.json",
			"configs/channel_texts.json",
			"../release/configs/channel_texts.json",
			"../../release/configs/channel_texts.json",
		} {
			candidates = append(candidates, filepath.Join(wd, rel))
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "configs", "channel_texts.json"),
			filepath.Join(exeDir, "release", "configs", "channel_texts.json"),
			filepath.Join(exeDir, "..", "release", "configs", "channel_texts.json"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (s *ChannelTextService) loadConfig() (*channelTextConfig, error) {
	path := resolveChannelTextsConfigPath(os.Getenv("CHANNEL_TEXTS_CONFIG"))
	if path == "" {
		return nil, fmt.Errorf("channel texts config not found (set CHANNEL_TEXTS_CONFIG or place file under release/configs/channel_texts.json)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg channelTextConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.Channels == nil {
		cfg.Channels = map[string]map[string]localizedText{}
	}
	return &cfg, nil
}

func normalizeChannelType(channelType string) string {
	v := strings.TrimSpace(channelType)
	switch {
	case strings.EqualFold(v, "GooglePlay"), strings.EqualFold(v, "ChannelType_GooglePlay"):
		return "GooglePlay"
	case strings.EqualFold(v, "CompanyOwned"), strings.EqualFold(v, "ChannelType_CompanyOwned"):
		return "CompanyOwned"
	default:
		return "CompanyOwned"
	}
}

func normalizeLanguage(language string) string {
	v := strings.TrimSpace(language)
	if strings.EqualFold(v, "ur") {
		return "ur"
	}
	if strings.EqualFold(v, "zh") || strings.EqualFold(v, "zh-cn") || strings.EqualFold(v, "zh-hans") {
		return "zh"
	}
	return "en"
}

func pickLocalizedText(text localizedText, lang string) string {
	if lang == "ur" {
		if strings.TrimSpace(text.UR) != "" {
			return strings.TrimSpace(text.UR)
		}
		return strings.TrimSpace(text.EN)
	}
	if lang == "zh" {
		if strings.TrimSpace(text.ZH) != "" {
			return strings.TrimSpace(text.ZH)
		}
		if strings.TrimSpace(text.EN) != "" {
			return strings.TrimSpace(text.EN)
		}
		return strings.TrimSpace(text.UR)
	}
	if strings.TrimSpace(text.EN) != "" {
		return strings.TrimSpace(text.EN)
	}
	if strings.TrimSpace(text.ZH) != "" {
		return strings.TrimSpace(text.ZH)
	}
	return strings.TrimSpace(text.UR)
}
