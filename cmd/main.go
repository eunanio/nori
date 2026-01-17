// Package main is the entry point for the Nori CLI.
package main

import (
	"fmt"
	"os"

	"github.com/eunanio/nori/cmd/commands"
)

// Version information (set at build time)
var (
	version = "1.0.0-dev2"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd := commands.NewRootCommand(version, commit, date)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
