package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

			logger.Infof("Project %q: %d service(s)", project.Name, len(project.Services))

			d := driver.New(logger)
			orch := orchestrator.New(d, logger)

			if err := orch.Up(ctx, project, orchestrator.UpOptions{
				Detach: detach,
				Build:  build,
			}); err != nil {
				return err
			}

			if !detach {
				logger.Infof("All services started. Press Ctrl+C to stop.")
				<-ctx.Done()
				logger.Infof("\nGracefully stopping...")
				// Use a fresh context with a timeout for shutdown
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

	return cmd
}
