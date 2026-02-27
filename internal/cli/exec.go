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

func newExecCmd() *cobra.Command {
	var detach bool
	var user string
	var workdir string

	cmd := &cobra.Command{
		Use:   "exec SERVICE COMMAND [ARGS...]",
		Short: "Execute a command in a running service container",
		Args:  cobra.MinimumNArgs(2),
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
			execCmd := args[1:]

			if _, ok := project.Services[serviceName]; !ok {
				return fmt.Errorf("service %q not found", serviceName)
			}

			d := driver.New(logger)
			containerName := converter.ContainerName(project.Name, serviceName, 1)

			return d.ExecContainer(ctx, containerName, execCmd, driver.ExecOptions{
				Detach:  detach,
				User:    user,
				Workdir: workdir,
			})
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Detached mode: run command in the background")
	cmd.Flags().StringVarP(&user, "user", "u", "", "Username or UID")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")

	return cmd
}
