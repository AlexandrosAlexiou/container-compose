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

func newKillCmd() *cobra.Command {
	var signal string

	cmd := &cobra.Command{
		Use:   "kill [SERVICE...]",
		Short: "Force stop service containers",
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
			services := args
			if len(services) == 0 {
				for name := range project.Services {
					services = append(services, name)
				}
			}

			for _, svc := range services {
				name := converter.ContainerName(project.Name, svc, 1)
				logger.Infof("Killing %s", svc)
				if err := d.KillContainer(ctx, name, signal); err != nil {
					logger.Warnf("Failed to kill %s: %v", svc, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&signal, "signal", "s", "SIGKILL", "Signal to send to the container")
	return cmd
}

func newPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull [SERVICE...]",
		Short: "Pull service images",
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
			services := args
			if len(services) == 0 {
				for name := range project.Services {
					services = append(services, name)
				}
			}

			for _, svc := range services {
				service, ok := project.Services[svc]
				if !ok {
					return fmt.Errorf("service %q not found", svc)
				}
				if service.Image == "" {
					logger.Infof("Skipping %s (no image, build only)", svc)
					continue
				}
				if err := d.PullImage(ctx, service.Image, service.Platform); err != nil {
					return fmt.Errorf("pulling image for %s: %w", svc, err)
				}
			}
			return nil
		},
	}
}

func newPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [SERVICE...]",
		Short: "Push service images",
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
			services := args
			if len(services) == 0 {
				for name := range project.Services {
					services = append(services, name)
				}
			}

			for _, svc := range services {
				service, ok := project.Services[svc]
				if !ok {
					return fmt.Errorf("service %q not found", svc)
				}
				if service.Image == "" {
					logger.Warnf("Skipping %s (no image tag)", svc)
					continue
				}
				if err := d.PushImage(ctx, service.Image); err != nil {
					return fmt.Errorf("pushing image for %s: %w", svc, err)
				}
			}
			return nil
		},
	}
}
