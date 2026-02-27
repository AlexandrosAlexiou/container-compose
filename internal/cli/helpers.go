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

	// Default project name from directory name
	if opts.ProjectName == "" {
		opts.ProjectName = filepath.Base(opts.ProjectDir)
	}

	return opts, nil
}
