package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List containers",
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
			containers, err := d.ListContainers(ctx, project.Name)
			if err != nil {
				return err
			}

			if len(containers) == 0 {
				logger.Infof("No containers running for project %q", project.Name)
				return nil
			}

			// Print header
			fmt.Fprintf(os.Stdout, "%-30s %-20s %-15s %s\n", "NAME", "SERVICE", "STATUS", "PORTS")
			fmt.Fprintf(os.Stdout, "%-30s %-20s %-15s %s\n", "----", "-------", "------", "-----")

			for _, c := range containers {
				fmt.Fprintf(os.Stdout, "%-30s %-20s %-15s %s\n",
					c.Name, c.Service, c.Status, c.Ports)
			}

			return nil
		},
	}

	return cmd
}
