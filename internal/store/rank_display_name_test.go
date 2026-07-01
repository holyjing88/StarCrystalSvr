package store

import (
	"testing"

	providertypes "starcrystal/server/internal/provider"
)

func TestRankListDisplayName_PrefersDisplayName(t *testing.T) {
	rec := &AuthUserRecord{
		AccountID:   "phone_+861234",
		DisplayName: "EcoTest",
		Nickname:    "nick_hidden",
	}
	if got := RankListDisplayName(rec, rec.AccountID); got != "EcoTest" {
		t.Fatalf("got %q want EcoTest", got)
	}
}

func TestRankListDisplayName_NeverReturnsAccountID(t *testing.T) {
	rec := &AuthUserRecord{
		AccountID:   "guest_abc123",
		DisplayName: "Guest",
		Provider:    providertypes.Guest,
	}
	if got := RankListDisplayName(rec, rec.AccountID); got == rec.AccountID {
		t.Fatalf("must not expose account id, got %q", got)
	}
}

func TestRankListDisplayName_PhoneHint(t *testing.T) {
	rec := &AuthUserRecord{
		AccountID: "phone_+8613786994748",
		Phone:     "+8613786994748",
	}
	if got := RankListDisplayName(rec, rec.AccountID); got != "…4748" {
		t.Fatalf("got %q want …4748", got)
	}
}
