package service

import (
	"testing"
	"time"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/store"
)

// NTF-003: pending = cur_downline - watermark。
func TestInviteNotifyService_ComputePending(t *testing.T) {
	svc := NewInviteNotifyService(nil, config.InviteConfig{Enabled: true}, "Asia/Shanghai")
	rec := &store.AuthUserRecord{
		CurDownlineL1Contrib: 10, NotifyWatermarkDownlineL1: 3,
		CurDownlineL2Contrib: 5, NotifyWatermarkDownlineL2: 5,
	}
	p := svc.ComputePending(rec)
	if p.PendingL1 != 7 || p.PendingL2 != 0 {
		t.Fatalf("got %+v", p)
	}
}

// NTF-004: 7 天 auto-ack 时间窗判定。
func TestInviteNotifyAutoAckEligible(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	pending := InvitePendingContrib{PendingL1: 10}
	old := now.AddDate(0, 0, -8)
	recent := now.AddDate(0, 0, -3)

	if !inviteNotifyAutoAckEligible(now, 7, &old, pending) {
		t.Fatal("want eligible after 8 days")
	}
	if inviteNotifyAutoAckEligible(now, 7, &recent, pending) {
		t.Fatal("want not eligible after 3 days")
	}
	if inviteNotifyAutoAckEligible(now, 7, nil, pending) {
		t.Fatal("want not eligible without since")
	}
	if inviteNotifyAutoAckEligible(now, 7, &old, InvitePendingContrib{}) {
		t.Fatal("want not eligible without pending")
	}
}
