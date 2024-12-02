package main

import (
	"os"
	"path/filepath"
)

func findUp(path, name string) (string, bool) {
	for {
		parent := filepath.Dir(path)
		if parent == path {
			return "", false
		}
		file := filepath.Join(path, name)
		if _, err := os.Stat(file); err == nil {
			return file, true
		}
		path = parent
	}
}
