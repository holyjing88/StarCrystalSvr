package starcrystaljson

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigEnv is the environment variable for an explicit starcrystal.json path
// (release/startsvr sets it beside GAMES_CONFIG).
const ConfigEnv = "STARCrystal_CONFIG"

// ConfigCandidates returns starcrystal.json paths to try, most specific first.
func ConfigCandidates() []string {
	var candidates []string
	seen := make(map[string]struct{})
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		candidates = append(candidates, p)
	}

	if env := strings.TrimSpace(os.Getenv(ConfigEnv)); env != "" {
		add(env)
	}

	add(filepath.Join("configs", "starcrystal.json"))
	add(filepath.Join("release", "configs", "starcrystal.json"))

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			add(filepath.Join(dir, "configs", "starcrystal.json"))
			add(filepath.Join(dir, "release", "configs", "starcrystal.json"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir := filepath.Dir(exe)
		add(filepath.Join(exeDir, "configs", "starcrystal.json"))
		add(filepath.Join(exeDir, "release", "configs", "starcrystal.json"))
		add(filepath.Join(exeDir, "..", "release", "configs", "starcrystal.json"))
	}

	return candidates
}
