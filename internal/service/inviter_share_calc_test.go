package service

import (
	"testing"

	"starcrystal/server/internal/config"
)

// SHR-002: 10% / 5% 基础分成。
func TestCalcInviteShare_BasicChain(t *testing.T) {
	cfg := config.DefaultInviteConfig()
	got := calcInviteShare(100, cfg, "none", false)
	if got.Direct.Paid != 10 || got.Second.Paid != 5 {
		t.Fatalf("got %v/%v", got.Direct.Paid, got.Second.Paid)
	}
}

// SHR-003: 单笔上限按比例截断。
func TestCalcInviteShare_TotalCapProportional(t *testing.T) {
	cfg := config.DefaultInviteConfig()
	got := calcInviteShare(10000, cfg, "none", false)
	if got.Direct.Paid != 400 || got.Second.Paid != 200 {
		t.Fatalf("got %v/%v want 400/200", got.Direct.Paid, got.Second.Paid)
	}
}

// SHR-004: GM 发币 gmGrantShareMode=none 时 paid=0。
func TestCalcInviteShare_GmNone(t *testing.T) {
	got := calcInviteShare(100, config.DefaultInviteConfig(), "none", true)
	if got.TotalPaid != 0 || got.DenominatorAdd != 15 {
		t.Fatalf("got paid=%v denom=%v", got.TotalPaid, got.DenominatorAdd)
	}
}
