package config

import (
	"fmt"
	"strings"
)

type InviteConfig struct {
	Enabled                  bool    `json:"enabled"`
	DirectShareRate          float64 `json:"directShareRate"`
	SecondShareRate          float64 `json:"secondShareRate"`
	ShareDecimals            int     `json:"shareDecimals"`
	ShareRound               string  `json:"shareRound"`
	MaxDirectSharePerEvent   float64 `json:"maxDirectSharePerEvent"`
	MaxSecondSharePerEvent   float64 `json:"maxSecondSharePerEvent"`
	MaxTotalSharePerEvent    float64 `json:"maxTotalSharePerEvent"`
	GmGrantShareMode         string  `json:"gmGrantShareMode"`
	GmMaxDirectSharePerEvent float64 `json:"gmMaxDirectSharePerEvent"`
	GmMaxSecondSharePerEvent float64 `json:"gmMaxSecondSharePerEvent"`
	GmMaxTotalSharePerEvent  float64 `json:"gmMaxTotalSharePerEvent"`
	HeartbeatIntervalSec     int     `json:"heartbeatIntervalSec"`
	HeartbeatOfflineSec      int     `json:"heartbeatOfflineSec"`
	NotifyMergeSec           int     `json:"notifyMergeSec"`
	NotifyAutoAckDays        int     `json:"notifyAutoAckDays"`
}

func DefaultInviteConfig() InviteConfig {
	return InviteConfig{
		Enabled: true, DirectShareRate: 0.10, SecondShareRate: 0.05,
		ShareDecimals: 2, ShareRound: "floor",
		MaxDirectSharePerEvent: 500, MaxSecondSharePerEvent: 250, MaxTotalSharePerEvent: 600,
		GmGrantShareMode: "none",
		GmMaxDirectSharePerEvent: 100, GmMaxSecondSharePerEvent: 50, GmMaxTotalSharePerEvent: 120,
		HeartbeatIntervalSec: 60, HeartbeatOfflineSec: 180, NotifyMergeSec: 60, NotifyAutoAckDays: 7,
	}
}

func NormalizeInvite(c *InviteConfig) {
	def := DefaultInviteConfig()
	if c.ShareDecimals <= 0 {
		c.ShareDecimals = def.ShareDecimals
	}
	if strings.TrimSpace(c.ShareRound) == "" {
		c.ShareRound = def.ShareRound
	}
	if c.MaxDirectSharePerEvent <= 0 {
		c.MaxDirectSharePerEvent = def.MaxDirectSharePerEvent
	}
	if c.MaxSecondSharePerEvent <= 0 {
		c.MaxSecondSharePerEvent = def.MaxSecondSharePerEvent
	}
	if c.MaxTotalSharePerEvent <= 0 {
		c.MaxTotalSharePerEvent = def.MaxTotalSharePerEvent
	}
	if strings.TrimSpace(c.GmGrantShareMode) == "" {
		c.GmGrantShareMode = def.GmGrantShareMode
	}
	c.GmGrantShareMode = strings.ToLower(strings.TrimSpace(c.GmGrantShareMode))
	if c.GmMaxDirectSharePerEvent <= 0 {
		c.GmMaxDirectSharePerEvent = def.GmMaxDirectSharePerEvent
	}
	if c.GmMaxSecondSharePerEvent <= 0 {
		c.GmMaxSecondSharePerEvent = def.GmMaxSecondSharePerEvent
	}
	if c.GmMaxTotalSharePerEvent <= 0 {
		c.GmMaxTotalSharePerEvent = def.GmMaxTotalSharePerEvent
	}
	if c.HeartbeatIntervalSec <= 0 {
		c.HeartbeatIntervalSec = def.HeartbeatIntervalSec
	}
	if c.HeartbeatOfflineSec <= 0 {
		c.HeartbeatOfflineSec = def.HeartbeatOfflineSec
	}
	if c.NotifyMergeSec <= 0 {
		c.NotifyMergeSec = def.NotifyMergeSec
	}
	if c.NotifyAutoAckDays <= 0 {
		c.NotifyAutoAckDays = def.NotifyAutoAckDays
	}
}

func ValidateInviteConfig(c InviteConfig) error {
	mode := strings.ToLower(strings.TrimSpace(c.GmGrantShareMode))
	if mode != "none" && mode != "capped" {
		return fmt.Errorf("invite.gmGrantShareMode must be none or capped, got %q", c.GmGrantShareMode)
	}
	if c.DirectShareRate < 0 || c.SecondShareRate < 0 {
		return fmt.Errorf("invite share rates must be non-negative")
	}
	if c.DirectShareRate+c.SecondShareRate > 1 {
		return fmt.Errorf("invite directShareRate+secondShareRate must not exceed 1")
	}
	return nil
}
