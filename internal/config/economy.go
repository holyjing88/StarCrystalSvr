package config

import (
	"encoding/json"
	"strings"

	"starcrystal/server/internal/starcrystaljson"
)

type GoldConfig struct {
	DailyProduceCapEnabled  bool               `json:"dailyProduceCapEnabled"`
	DailyProduceCap         float64            `json:"dailyProduceCap"`
	DailyProduceCapOverflow string             `json:"dailyProduceCapOverflow"`
	DailyProduceCapByBiz    map[string]float64 `json:"dailyProduceCapByBiz"`
	Timezone                string             `json:"timezone"`
}

type WelfareConfig struct {
	MonthTokenPool        float64            `json:"monthTokenPool"`
	MonthTokenPoolByMonth map[string]float64 `json:"monthTokenPoolByMonth"`
	MonthlyExchangeAt     string             `json:"monthlyExchangeAt"`
	MonthlyExchangeDay    string             `json:"monthlyExchangeDay"`
	TokenDeltaRound       string             `json:"tokenDeltaRound"`
	TokenDeltaDecimals    int                `json:"tokenDeltaDecimals"`
}

type IdipOperator struct {
	Username    string `json:"username"`
	PasswordEnc string `json:"passwordEnc,omitempty"`
	Password    string `json:"password,omitempty"`
}

type IdipConfig struct {
	Key                         string         `json:"key"`
	OperatorCipherKey           string         `json:"operatorCipherKey,omitempty"`
	Operators                   []IdipOperator `json:"operators,omitempty"`
	SessionTtlSec               int            `json:"sessionTtlSec,omitempty"`
	SessionHeartbeatIntervalSec int            `json:"sessionHeartbeatIntervalSec,omitempty"`
	SessionHeartbeatTimeoutSec  int            `json:"sessionHeartbeatTimeoutSec,omitempty"`
	LoginFailMaxAttempts        int            `json:"loginFailMaxAttempts,omitempty"`
	LoginFailWindowSec          int            `json:"loginFailWindowSec,omitempty"`
}

type PublishRsyncConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	CdnH5Dir string `json:"cdnH5Dir,omitempty"` // 本机 rsync 目标，如 /wwwroot/minigame.starlaneinfinite.com/h5
}

type PublishConfig struct {
	H5AssetsDir        string             `json:"h5AssetsDir,omitempty"`
	GamesConfigPath    string             `json:"gamesConfigPath,omitempty"`
	H5BackupDir        string             `json:"h5BackupDir,omitempty"`
	H5BackupMax        int                `json:"h5BackupMax,omitempty"`
	GamesJsonBackupMax int                `json:"gamesJsonBackupMax,omitempty"`
	ServeH5OnAPI       *bool              `json:"serveH5OnApi,omitempty"` // 默认 true；联调 CDN 独立时设 false
	Rsync              PublishRsyncConfig `json:"rsync,omitempty"`
}

type EconomyConfig struct {
	Gold    GoldConfig    `json:"gold"`
	Welfare WelfareConfig `json:"welfare"`
	Invite  InviteConfig  `json:"invite"`
	Idip    IdipConfig    `json:"idip"`
	Publish PublishConfig `json:"publish"`
}

func (c GoldConfig) LocationName() string {
	if strings.TrimSpace(c.Timezone) == "" {
		return "Asia/Shanghai"
	}
	return strings.TrimSpace(c.Timezone)
}

func (c GoldConfig) OverflowClamp() bool {
	return strings.ToLower(strings.TrimSpace(c.DailyProduceCapOverflow)) != "reject"
}

func LoadEconomyConfig() (EconomyConfig, string, bool) {
	for _, p := range starcrystaljson.ConfigCandidates() {
		cfg, err := LoadEconomyConfigFrom(p)
		if err != nil {
			continue
		}
		return cfg, p, true
	}
	return EconomyConfig{}, "", false
}

func LoadEconomyConfigFrom(path string) (EconomyConfig, error) {
	raw, err := starcrystaljson.ReadFileUTF8(path)
	if err != nil {
		return EconomyConfig{}, err
	}
	var cfg EconomyConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return EconomyConfig{}, err
	}
	NormalizeInvite(&cfg.Invite)
	if err := ValidateInviteConfig(cfg.Invite); err != nil {
		return EconomyConfig{}, err
	}
	return cfg, nil
}
