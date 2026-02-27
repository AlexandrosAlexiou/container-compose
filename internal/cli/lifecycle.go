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

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [SERVICE...]",
		Short: "Start existing containers",
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
				logger.Infof("Starting %s", svc)
				if err := d.StartContainer(ctx, name); err != nil {
					logger.Warnf("Failed to start %s: %v", svc, err)
				}
			}
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [SERVICE...]",
		Short: "Stop running containers without removing them",
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
				logger.Infof("Stopping %s", svc)
				if err := d.StopContainer(ctx, name); err != nil {
					logger.Warnf("Failed to stop %s: %v", svc, err)
				}
			}
			return nil
		},
	}
}

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [SERVICE...]",
		Short: "Restart service containers",
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
				logger.Infof("Restarting %s", svc)
				if err := d.StopContainer(ctx, name); err != nil {
					logger.Warnf("Failed to stop %s: %v", svc, err)
				}
				if err := d.StartContainer(ctx, name); err != nil {
					return fmt.Errorf("failed to start %s: %w", svc, err)
				}
				logger.Successf("Restarted %s", svc)
			}
			return nil
		},
	}
}

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [SERVICE...]",
		Short: "Create containers without starting them",
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

				// Use create instead of run (no -d, no start)
				runArgs := converter.ContainerRunArgs(project.Name, service, svc, 1)
				// Replace "run" with "create" in the args
				runArgs[0] = "create"
				logger.Infof("Creating %s", svc)
				if err := d.RunContainer(ctx, runArgs); err != nil {
					return fmt.Errorf("creating %s: %w", svc, err)
				}
				logger.Successf("Created %s", svc)
			}
			return nil
		},
	}
}

func newRmCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "rm [SERVICE...]",
		Short: "Remove stopped service containers",
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
				if force {
					_ = d.StopContainer(ctx, name)
				}
				logger.Infof("Removing %s", svc)
				if err := d.DeleteContainer(ctx, name); err != nil {
					logger.Warnf("Failed to remove %s: %v", svc, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force stop before removing")
	return cmd
}
