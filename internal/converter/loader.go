package converter

import (
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

// ProjectOptions holds options for loading a compose project.
type ProjectOptions struct {
	ConfigPaths []string
	ProjectName string
	ProjectDir  string
	Profiles    []string
	EnvFiles    []string
}

// LoadProject loads and parses a compose file into a Project.
func LoadProject(opts ProjectOptions) (*types.Project, error) {
	cliOpts := []cli.ProjectOptionsFn{
		cli.WithWorkingDirectory(opts.ProjectDir),
	}

	if opts.ProjectName != "" {
		cliOpts = append(cliOpts, cli.WithName(opts.ProjectName))
	}

	if len(opts.Profiles) > 0 {
		cliOpts = append(cliOpts, cli.WithProfiles(opts.Profiles))
	}

	if len(opts.EnvFiles) > 0 {
		cliOpts = append(cliOpts, cli.WithEnvFiles(opts.EnvFiles...))
	}

	// When no config paths are explicitly given, allow env-based and default file discovery
	if len(opts.ConfigPaths) == 0 {
		cliOpts = append(cliOpts, cli.WithConfigFileEnv)
		cliOpts = append(cliOpts, cli.WithDefaultConfigPath)
	}

	projectOpts, err := cli.NewProjectOptions(opts.ConfigPaths, cliOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating project options: %w", err)
	}

	project, err := projectOpts.LoadProject(context.Background())
	if err != nil {
		return nil, fmt.Errorf("loading project: %w", err)
	}

	return project, nil
}
