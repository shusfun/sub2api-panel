package main

import (
	"os"

	"sub2api-panel/internal/cli"
)

var version = "dev"
var commit = "unknown"

func main() {
	cli.Run(os.Args[1:], version, commit, frontendDist)
}
