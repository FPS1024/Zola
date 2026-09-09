package main

import (
	"fmt"
	"os"

	"zola/internal/buildinfo"
	"zola/internal/cli"
)

func main() {
	if err := cli.Execute(buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
