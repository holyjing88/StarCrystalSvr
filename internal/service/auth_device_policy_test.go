package service

import (
	"testing"
	"time"
)

func TestDeviceAccountConflictError_Error(t *testing.T) {
	e := &DeviceAccountConflictError{
		NeedConfirmation: true,
		Message:          "该设备已有账号，请先确认注销旧账号",
	}
	if got := e.Error(); got == "" {
		t.Fatal("empty message")
	}
}

func TestAccountBannedError_ErrorContainsReason(t *testing.T) {
	e := &AccountBannedError{Reason: "同设备账号同意注销旧账号"}
	if got := e.Error(); got == "" || got == "账号已被封禁" {
		t.Fatalf("unexpected message=%q", got)
	}
}

func TestAccountSilentError_Error(t *testing.T) {
	e := &AccountSilentError{RemainingSec: 30}
	if got := e.Error(); got == "" {
		t.Fatal("empty message")
	}
}

func TestDeviceAccountConflictError_RemainingMessage(t *testing.T) {
	e := &DeviceAccountConflictError{
		QuietUntil:   ptrTime(time.Now().Add(5 * time.Minute)),
		RemainingSec: 300,
	}
	if got := e.Error(); got == "" {
		t.Fatal("empty message")
	}
}

func TestPolicyFromStarcrystal_DeviceSilentUnified(t *testing.T) {
	var cfg starCrystalConfig
	cfg.Policy.MaxAccountsPerDevice = 2
	cfg.Policy.DeviceSilentSeconds = 3600
	cfg.Policy.DeviceSilentSecondsGuest = 7200
	cfg.Policy.DeviceSilentSecondsEmailPhone = 9000
	maxA, g, e := policyFromStarcrystal(cfg)
	if maxA != 2 || g != 7200 || e != 9000 {
		t.Fatalf("expected per-type overrides, got max=%d guest=%d emailPhone=%d", maxA, g, e)
	}
}

func TestPolicyFromStarcrystal_DeviceSilentUnifiedOnly(t *testing.T) {
	var cfg starCrystalConfig
	cfg.Policy.DeviceSilentSeconds = 1800
	maxA, g, e := policyFromStarcrystal(cfg)
	if maxA != defaultMaxAccountsPerDevice || g != 1800 || e != 1800 {
		t.Fatalf("expected unified silent, got max=%d guest=%d emailPhone=%d", maxA, g, e)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
