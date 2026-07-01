package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListGamesForClient_FiltersByChannelAgainstSampleConfig(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "release", "configs", "games.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skip("games.json not at ", cfgPath)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	svc := NewGameService()
	all, err := svc.ListGames()
	if err != nil {
		t.Fatal(err)
	}
	var hasG001 bool
	for _, g := range all {
		if g.GameID == "g001" {
			hasG001 = true
			t.Logf("g001 channels raw count=%d %#v", len(g.Channels), []string(g.Channels))
		}
	}
	if !hasG001 {
		t.Fatal("expected g001 in config file")
	}

	listCo, err := svc.ListGamesForClient("9.9.9", "android", "ChannelType_CompanyOwned")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range listCo {
		if g.GameID == "g001" {
			t.Fatalf("CompanyOwned client must not receive g001 (channels=%#v)", []string(g.Channels))
		}
	}

	listGp, err := svc.ListGamesForClient("9.9.9", "android", "ChannelType_GooglePlay")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range listGp {
		if g.GameID == "g001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("GooglePlay client should receive g001")
	}
}

func TestListGamesForClient_EmptyChannelOmitsChannelRestrictedGames(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "release", "configs", "games.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skip("games.json not at ", cfgPath)
	}
	t.Setenv("GAMES_CONFIG", cfgPath)

	svc := NewGameService()
	list, err := svc.ListGamesForClient("9.9.9", "android", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range list {
		if g.GameID == "g001" {
			t.Fatal("empty channel query must not receive g001 (its channels field is restricted)")
		}
	}
}
