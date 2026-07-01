package service

import (
	"encoding/json"
	"testing"
)

func TestSplitGameConfigChannels(t *testing.T) {
	got := SplitGameConfigChannels(" ChannelType_A - ChannelType_B - ChannelType_C ")
	if len(got) != 3 || got[0] != "ChannelType_A" || got[1] != "ChannelType_B" || got[2] != "ChannelType_C" {
		t.Fatalf("unexpected split: %#v", got)
	}
}

func TestGameChannelSingleClientMatch(t *testing.T) {
	gameTok := []string{"ChannelType_CompanyOwned", "ChannelType_GooglePlay"}
	if !gameChannelSingleClientMatch("ChannelType_CompanyOwned", gameTok) {
		t.Fatal("expected match canonical CompanyOwned")
	}
	if gameChannelSingleClientMatch("CompanyOwned", []string{"ChannelType_GooglePlay"}) {
		t.Fatal("expected CompanyOwned not to match GooglePlay-only config")
	}
	if !gameChannelSingleClientMatch("CompanyOwned", []string{"ChannelType_CompanyOwned"}) {
		t.Fatal("expected legacy CompanyOwned to match ChannelType_CompanyOwned")
	}
	if !gameChannelSingleClientMatch("GooglePlay", []string{"ChannelType_GooglePlay"}) {
		t.Fatal("expected legacy GooglePlay to match ChannelType_GooglePlay")
	}
	if gameChannelSingleClientMatch("ChannelType_CompanyOwned", []string{"ChannelType_GooglePlay"}) {
		t.Fatal("expected no match")
	}
}

func TestGameChannelsJSONUnmarshal(t *testing.T) {
	var item GameItem
	err := json.Unmarshal([]byte(`{"gameId":"x","entryType":"h5","entryUrl":"/","sort":1,"channels":"ChannelType_A-ChannelType_B"}`), &item)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Channels) != 2 {
		t.Fatalf("channels: %#v", []string(item.Channels))
	}
	err = json.Unmarshal([]byte(`{"gameId":"y","entryType":"h5","entryUrl":"/","sort":1,"channels":["ChannelType_X","ChannelType_Y-ChannelType_Z"]}`), &item)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Channels) != 3 {
		t.Fatalf("channels array flatten: %#v", []string(item.Channels))
	}
}
