// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/orchestrator"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var detach bool
	var build bool
	var scale []string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start containers",
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

			scaleMap, err := parseScaleFlags(scale)
			if err != nil {
				return err
			}

			logger.Infof("Project %q: %d service(s)", project.Name, len(project.Services))

			d := driver.New(logger)
			orch := orchestrator.New(d, logger)

			if err := orch.Up(ctx, project, orchestrator.UpOptions{
				Detach: detach,
				Build:  build,
				Scale:  scaleMap,
			}); err != nil {
				return err
			}

			if !detach {
				logger.Infof("All services started. Press Ctrl+C to stop.")
				<-ctx.Done()
				logger.Infof("\nGracefully stopping...")
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer shutdownCancel()
				return orch.Down(shutdownCtx, project, orchestrator.DownOptions{
					RemoveOrphans: false,
				})
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Detached mode: Run containers in the background")
	cmd.Flags().BoolVar(&build, "build", false, "Build images before starting containers")
	cmd.Flags().StringSliceVar(&scale, "scale", nil, "Scale SERVICE to NUM instances (e.g. --scale web=3)")

	return cmd
}

func parseScaleFlags(flags []string) (map[string]int, error) {
	if len(flags) == 0 {
		return nil, nil
	}

	result := make(map[string]int)
	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid scale format %q, expected SERVICE=NUM", flag)
		}
		var n int
		if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil || n < 1 {
			return nil, fmt.Errorf("invalid replica count %q for service %q", parts[1], parts[0])
		}
		result[parts[0]] = n
	}
	return result, nil
}
