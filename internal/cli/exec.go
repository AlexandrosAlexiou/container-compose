// Package cli implements the container-compose command-line interface using cobra.
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
	var noInteractive bool

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
			containerName := converter.ResolveContainerName(project, serviceName, 1)

			interactive := !noInteractive
			// Allocate a TTY when stdin is a terminal, unless -T was passed.
			tty := interactive && !noInteractive && isTerminal(os.Stdin)

			return d.ExecContainer(ctx, containerName, execCmd, driver.ExecOptions{
				Detach:      detach,
				User:        user,
				Workdir:     workdir,
				Interactive: interactive,
				TTY:         tty,
			})
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Detached mode: run command in the background")
	cmd.Flags().BoolVarP(&noInteractive, "no-TTY", "T", false, "Disable pseudo-TTY allocation. By default exec allocates a TTY")
	cmd.Flags().StringVarP(&user, "user", "u", "", "Username or UID")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.Flags().SetInterspersed(false)

	return cmd
}
