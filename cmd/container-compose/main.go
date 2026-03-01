// Package main is the entry point for the container-compose CLI.
package main

import (
	"fmt"
	"os"

	"github.com/apple/container-compose/internal/cli"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
