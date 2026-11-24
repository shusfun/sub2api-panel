//go:build panel_release

package main

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var embeddedFrontend embed.FS

func frontendDist(_ string) (fs.FS, error) { return fs.Sub(embeddedFrontend, "web/dist") }
