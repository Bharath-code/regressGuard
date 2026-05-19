package main

import (
	"os"

	"github.com/regressguard/regressguard/internal/cli"
)

var (
	version = "0.1.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}); err != nil {
		os.Exit(2)
	}
}
