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

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [SERVICE...]",
		Short: "Build or rebuild services",
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

			for _, name := range services {
				service, ok := project.Services[name]
				if !ok {
					return fmt.Errorf("service %q not found", name)
				}

				if service.Build == nil {
					logger.Infof("Service %s has no build configuration, skipping", name)
					continue
				}

				tag := service.Image
				if tag == "" {
					tag = fmt.Sprintf("%s-%s", project.Name, name)
				}

				contextPath := service.Build.Context
				if contextPath == "" {
					contextPath = "."
				}

				if err := d.BuildImage(ctx, contextPath, service.Build.Dockerfile, tag); err != nil {
					return fmt.Errorf("building service %s: %w", name, err)
				}

				logger.Successf("Service %s built", name)
			}

			return nil
		},
	}

	return cmd
}
