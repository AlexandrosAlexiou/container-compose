// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"os"
	"path/filepath"

	"github.com/apple/container-compose/internal/converter"
	"github.com/spf13/cobra"
)

func projectOptionsFromCmd(cmd *cobra.Command) (converter.ProjectOptions, error) {
	var opts converter.ProjectOptions

	file, _ := cmd.Flags().GetString("file")
	if file != "" {
		opts.ConfigPaths = []string{file}
	}

	opts.ProjectName, _ = cmd.Flags().GetString("project-name")
	opts.ProjectDir, _ = cmd.Flags().GetString("project-directory")
	opts.Profiles, _ = cmd.Flags().GetStringSlice("profile")
	opts.EnvFiles, _ = cmd.Flags().GetStringSlice("env-file")

	if opts.ProjectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return opts, err
		}
		opts.ProjectDir = cwd
	}

	if opts.ProjectName == "" {
		opts.ProjectName = filepath.Base(opts.ProjectDir)
	}

	return opts, nil
}

// isTerminal returns true if the given file is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
