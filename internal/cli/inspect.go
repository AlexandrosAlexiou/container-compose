// Package cli implements the container-compose command-line interface using cobra.
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

func newTopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "top [SERVICE...]",
		Short: "Display the running processes",
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
				containerName := converter.ResolveContainerName(project, svc, 1)
				fmt.Fprintf(os.Stdout, "\n%s\n", svc)
				if err := d.TopContainer(ctx, containerName); err != nil {
					logger.Warnf("Failed to get processes for %s: %v", svc, err)
				}
			}
			return nil
		},
	}
}

func newPortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port SERVICE PRIVATE_PORT",
		Short: "Print the public port for a port binding",
		Args:  cobra.ExactArgs(2),
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

			serviceName := args[0]
			privatePort := args[1]

			service, ok := project.Services[serviceName]
			if !ok {
				return fmt.Errorf("service %q not found", serviceName)
			}

			_ = ctx
			_ = logger

			for _, port := range service.Ports {
				if fmt.Sprintf("%d", port.Target) == privatePort {
					hostIP := port.HostIP
					if hostIP == "" {
						hostIP = "0.0.0.0"
					}
					fmt.Fprintf(os.Stdout, "%s:%s\n", hostIP, port.Published)
					return nil
				}
			}

			return fmt.Errorf("no port mapping found for %s/%s", serviceName, privatePort)
		},
	}
}

func newImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "images [SERVICE...]",
		Short: "List images used by the created containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := output.NewLogger(os.Stdout, os.Stderr)

			projectOpts, err := projectOptionsFromCmd(cmd)
			if err != nil {
				return err
			}
			project, err := converter.LoadProject(projectOpts)
			if err != nil {
				return fmt.Errorf("loading compose file: %w", err)
			}

			_ = logger

			services := args
			if len(services) == 0 {
				for name := range project.Services {
					services = append(services, name)
				}
			}

			fmt.Fprintf(os.Stdout, "%-20s %s\n", "SERVICE", "IMAGE")
			fmt.Fprintf(os.Stdout, "%-20s %s\n", "-------", "-----")

			for _, svc := range services {
				service, ok := project.Services[svc]
				if !ok {
					continue
				}
				image := service.Image
				if image == "" {
					image = fmt.Sprintf("%s-%s (build)", project.Name, svc)
				}
				fmt.Fprintf(os.Stdout, "%-20s %s\n", svc, image)
			}
			return nil
		},
	}
}

func newRmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [SERVICE...]",
		Short: "Remove images used by services",
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
				image := service.Image
				if image == "" {
					image = fmt.Sprintf("%s-%s", project.Name, svc)
				}
				logger.Infof("Removing image %s", image)
				if err := d.DeleteImage(ctx, image); err != nil {
					logger.Warnf("Failed to remove image for %s: %v", svc, err)
				} else {
					logger.Successf("Removed %s", image)
				}
			}
			return nil
		},
	}
}

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats [SERVICE...]",
		Short: "Display a live stream of container resource usage statistics",
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
				containerName := converter.ResolveContainerName(project, svc, 1)
				if err := d.StatsContainer(ctx, containerName); err != nil {
					logger.Warnf("Failed to get stats for %s: %v", svc, err)
				}
			}
			return nil
		},
	}
}
