package service

import (
	"path/filepath"
	"testing"
)

func TestChannelTextService_ResolveByChannelAndLanguage(t *testing.T) {
	t.Setenv("CHANNEL_TEXTS_CONFIG", filepath.Join("..", "..", "release", "configs", "channel_texts.json"))
	s := NewChannelTextService()
	items, err := s.Resolve("GooglePlay", "en", []string{
		"LoginRoot/invitecode/Title",
		"LoginRoot/invitecode/inviteduserid/Placeholder",
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Text == "" || items[1].Text == "" {
		t.Fatalf("expected non-empty texts, got %+v", items)
	}
}
