package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var follow bool
	var tail string

	cmd := &cobra.Command{
		Use:   "logs [SERVICE...]",
		Short: "View output from containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

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

			services := args
			if len(services) == 0 {
				for name := range project.Services {
					services = append(services, name)
				}
			}

			return d.Logs(ctx, project.Name, services, driver.LogsOptions{
				Follow: follow,
				Tail:   tail,
			})
		},
	}

	cmd.Flags().BoolVar(&follow, "follow", false, "Follow log output")
	cmd.Flags().StringVar(&tail, "tail", "all", "Number of lines to show from the end of the logs")

	return cmd
}
