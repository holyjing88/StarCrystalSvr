// 生成 idip-webclient Vitest 用 H5 zip fixtures。
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := filepath.Join("tools", "idip-webclient", "src", "tests", "fixtures")
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}
	if err := writeFixture(filepath.Join(outDir, "vitest-game1.zip"), "vitest-game1"); err != nil {
		fatal(err)
	}
	if err := writeFixture(filepath.Join(outDir, "vitest-badname.zip"), "other-name"); err != nil {
		fatal(err)
	}
	fmt.Println("wrote fixtures to", outDir)
}

func writeFixture(path, topName string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	files := map[string]string{
		topName + "/index.html":       "<html></html>",
		topName + "/main.js":          "// main",
		topName + "/src/settings.js":  "window._CCSettings={};",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
