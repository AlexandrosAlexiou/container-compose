package cli

import (
	"fmt"
	"os"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"

	"go.yaml.in/yaml/v4"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Parse, resolve and render compose file in canonical format",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := output.NewLogger(os.Stdout, os.Stderr)

			projectOpts, err := projectOptionsFromCmd(cmd)
			if err != nil {
				return err
			}
			project, err := converter.LoadProject(projectOpts)
			if err != nil {
				return fmt.Errorf("loading compose file: %w", err)
			}

			_ = logger

			out, err := yaml.Marshal(project)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			fmt.Fprint(os.Stdout, string(out))
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the container-compose version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stdout, "container-compose version %s\n", Version)
			fmt.Fprintf(os.Stdout, "  compose-go: v2.10.1\n")
			fmt.Fprintf(os.Stdout, "  backend:    Apple Container CLI\n")
		},
	}
}

// Version is set at build time via ldflags.
var Version = "0.1.0-dev"
