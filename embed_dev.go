//go:build !panel_release

package main

import (
	"fmt"
	"io/fs"
	"os"
)

func frontendDist(dir string) (fs.FS, error) {
	if dir == "" {
		return nil, fmt.Errorf("PANEL_WEB_DIST is required for source builds")
	}
	dist := os.DirFS(dir)
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return nil, err
	}
	return dist, nil
}
