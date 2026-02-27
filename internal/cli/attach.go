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

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach SERVICE",
		Short: "Attach local standard input, output, and error streams to a service's running container",
		Args:  cobra.ExactArgs(1),
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

			serviceName := args[0]
			if _, ok := project.Services[serviceName]; !ok {
				return fmt.Errorf("service %q not found", serviceName)
			}

			d := driver.New(logger)
			containerName := converter.ContainerName(project.Name, serviceName, 1)
			return d.AttachContainer(ctx, containerName)
		},
	}
}
