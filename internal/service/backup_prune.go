package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pruneTimestampBackups keeps newest max entries matching prefix (e.g. games.json.bak.).
func pruneTimestampBackups(dir, prefix string, max int) {
	if max <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		name string
		ts   string
	}
	var matches []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		matches = append(matches, item{name: e.Name(), ts: strings.TrimPrefix(e.Name(), prefix)})
	}
	if len(matches) <= max {
		return
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ts > matches[j].ts })
	for _, m := range matches[max:] {
		_ = os.Remove(filepath.Join(dir, m.name))
	}
}

// pruneH5BackupsForGame keeps newest max timestamp subdirs under h5_backup/{gameDirName}/.
func pruneH5BackupsForGame(gameDirName string, max int) {
	pruneTimestampSubdirs(filepath.Join(H5BackupDir(), gameDirName), max)
}

func pruneTimestampSubdirs(root string, max int) {
	if max <= 0 {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	if len(entries) <= max {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	for _, e := range entries[max:] {
		if e.IsDir() {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}
