package service

import (
	"encoding/json"
	"testing"
)

func TestGameItem_UnmarshalDownloadFields(t *testing.T) {
	const raw = `{
		"gameId": "g001",
		"entryType": "h5",
		"entryUrl": "h5/game1/index.html?v=1.0.0.0",
		"downloadUrl": "h5/game1_v1.0.0.0.tar.gz",
		"packageBytes": 12345,
		"downloadSha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	}`
	var item GameItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatal(err)
	}
	if item.DownloadURL != "h5/game1_v1.0.0.0.tar.gz" {
		t.Fatalf("DownloadURL=%q", item.DownloadURL)
	}
	if item.PackageBytes != 12345 {
		t.Fatalf("PackageBytes=%d", item.PackageBytes)
	}
	if len(item.DownloadSha256) != 64 {
		t.Fatalf("DownloadSha256=%q", item.DownloadSha256)
	}
}
