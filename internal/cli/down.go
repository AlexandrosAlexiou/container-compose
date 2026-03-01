// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/orchestrator"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var removeVolumes bool
	var removeOrphans bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove containers, networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			logger := output.NewLogger(os.Stdout, os.Stderr)

			projectOpts, err := projectOptionsFromCmd(cmd)
			if err != nil {
				return err
			}

			project, err := converter.LoadProject(projectOpts)
			if err != nil {
				return fmt.Errorf("loading compose file: %w", err)
			}

			d := driver.New(logger)
			orch := orchestrator.New(d, logger)

			return orch.Down(ctx, project, orchestrator.DownOptions{
				RemoveVolumes: removeVolumes,
				RemoveOrphans: removeOrphans,
			})
		},
	}

	cmd.Flags().BoolVarP(&removeVolumes, "volumes", "v", false, "Remove named volumes")
	cmd.Flags().BoolVar(&removeOrphans, "remove-orphans", false, "Remove containers for services not defined in the Compose file")

	return cmd
}
