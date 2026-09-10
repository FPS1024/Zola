package main

import (
	"os"

	"zola/internal/buildinfo"
	"zola/internal/cli"
)

func main() {
	if err := cli.Execute(buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime); err != nil {
		cli.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
