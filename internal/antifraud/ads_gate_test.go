package antifraud

import (
	"fmt"
	"testing"
)

func TestAdsGate_CheckSlotValid(t *testing.T) {
	t.Setenv("AD_SLOT_ALLOWLIST", "slot-a, BetaSlot")

	g := NewAdsGateFromEnv()
	if err := g.CheckSlotValid("slot-a"); err != nil {
		t.Fatal(err)
	}
	if err := g.CheckSlotValid("betaslot"); err != nil {
		t.Fatal(err)
	}
	if err := g.CheckSlotValid(""); err == nil {
		t.Fatal("empty slot must fail when allowlist set")
	}
	if err := g.CheckSlotValid("unknown"); err != ErrSlotNotAllowed {
		t.Fatalf("want ErrSlotNotAllowed got %v", err)
	}
}

func TestAdsGate_StartBurstPerAccountAndIP(t *testing.T) {
	t.Setenv("AD_START_PER_MIN_ACCOUNT", "3")
	t.Setenv("AD_START_PER_MIN_IP", "4")
	t.Setenv("AD_SLOT_ALLOWLIST", "")
	g := NewAdsGateFromEnv()

	for i := 0; i < 3; i++ {
		if err := g.AllowStart("user-a", ""); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if err := g.AllowStart("user-a", ""); err != ErrRateLimitStart {
		t.Fatalf("want rate limit account, got %v", err)
	}

	ip := "198.51.100.42"
	h := NewAdsGateFromEnv()
	for i := 0; i < 4; i++ {
		if err := h.AllowStart(fmt.Sprintf("u%d", i), ip); err != nil {
			t.Fatalf("ip call %d: %v", i, err)
		}
	}
	if err := h.AllowStart("u9", ip); err != ErrRateLimitStart {
		t.Fatalf("want rate limit ip, got %v", err)
	}
}

func TestAdsGate_StartSkipsLoopbackIPDimension(t *testing.T) {
	t.Setenv("AD_START_PER_MIN_IP", "1")
	g := NewAdsGateFromEnv()
	if err := g.AllowStart("a", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := g.AllowStart("b", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
}

func TestAdsGate_CompleteBurst(t *testing.T) {
	t.Setenv("AD_COMPLETE_PER_MIN_ACCOUNT", "2")
	t.Setenv("AD_COMPLETE_PER_MIN_IP", "0")
	g := NewAdsGateFromEnv()
	if err := g.AllowComplete("c1", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.AllowComplete("c1", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.AllowComplete("c1", ""); err != ErrRateLimitComplete {
		t.Fatalf("want complete rate limit, got %v", err)
	}
}
