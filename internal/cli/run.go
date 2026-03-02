// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var rm bool
	var user string
	var workdir string
	var detach bool
	var envVars []string

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] SERVICE [COMMAND] [ARGS...]",
		Short: "Run a one-off command on a service",
		Args:  cobra.MinimumNArgs(1),
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
			service, ok := project.Services[serviceName]
			if !ok {
				return fmt.Errorf("service %q not found", serviceName)
			}

			d := driver.New(logger)

			containerName := fmt.Sprintf("%s-%s-run-%d", project.Name, serviceName, os.Getpid())
			runArgs := []string{"run", "--name", containerName}

			if detach {
				runArgs = append(runArgs, "-d")
			}

			if user != "" {
				runArgs = append(runArgs, "-u", user)
			} else if service.User != "" {
				runArgs = append(runArgs, "-u", service.User)
			}

			if workdir != "" {
				runArgs = append(runArgs, "-w", workdir)
			} else if service.WorkingDir != "" {
				runArgs = append(runArgs, "-w", service.WorkingDir)
			}

			for k, v := range service.Environment {
				if v != nil {
					runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, *v))
				}
			}
			for _, e := range envVars {
				runArgs = append(runArgs, "-e", e)
			}

			for _, vol := range service.Volumes {
				source := vol.Source
				if vol.Type == "volume" && source != "" {
					source = converter.VolumeName(project.Name, source)
				}
				v := source + ":" + vol.Target
				if vol.ReadOnly {
					v += ":ro"
				}
				runArgs = append(runArgs, "-v", v)
			}

			if len(service.Networks) > 0 {
				for network := range service.Networks {
					runArgs = append(runArgs, "--network", converter.NetworkName(project.Name, network))
					break
				}
			} else {
				runArgs = append(runArgs, "--network", converter.NetworkName(project.Name, "default"))
			}

			runArgs = append(runArgs, "--hostname", serviceName)

			runArgs = append(runArgs, service.Image)

			if len(args) > 1 {
				runArgs = append(runArgs, args[1:]...)
			} else if len(service.Command) > 0 {
				runArgs = append(runArgs, service.Command...)
			}

			err = d.RunContainerInteractive(ctx, runArgs)

			if rm {
				_ = d.StopContainer(context.Background(), containerName)
				_ = d.DeleteContainer(context.Background(), containerName)
			}

			return err
		},
	}

	cmd.Flags().BoolVar(&rm, "rm", true, "Remove container after run")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in the background")
	cmd.Flags().StringVarP(&user, "user", "u", "", "Run as specified user")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.Flags().StringSliceVarP(&envVars, "env", "e", nil, "Set environment variables")

	return cmd
}

func newCpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [OPTIONS] SERVICE:SRC_PATH DEST_PATH|-\n  container-compose cp [OPTIONS] SRC_PATH|- SERVICE:DEST_PATH",
		Short: "Copy files/folders between a service container and the local filesystem",
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

			d := driver.New(logger)

			src := args[0]
			dst := args[1]

			if parts := strings.SplitN(src, ":", 2); len(parts) == 2 {
				svc := parts[0]
				if _, ok := project.Services[svc]; !ok {
					return fmt.Errorf("service %q not found", svc)
				}
				containerName := converter.ContainerName(project.Name, svc, 1)
				return d.CopyFromContainer(ctx, containerName, parts[1], dst)
			} else if parts := strings.SplitN(dst, ":", 2); len(parts) == 2 {
				svc := parts[0]
				if _, ok := project.Services[svc]; !ok {
					return fmt.Errorf("service %q not found", svc)
				}
				containerName := converter.ContainerName(project.Name, svc, 1)
				return d.CopyToContainer(ctx, containerName, src, parts[1])
			}

			return fmt.Errorf("one of src or dst must be SERVICE:PATH")
		},
	}
}

func newWaitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wait SERVICE [SERVICE...]",
		Short: "Block until the first service container stops",
		Args:  cobra.MinimumNArgs(1),
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

			for {
				for _, svc := range args {
					containerName := converter.ContainerName(project.Name, svc, 1)
					containers, err := d.ListContainers(ctx, project.Name)
					if err != nil {
						continue
					}

					found := false
					for _, c := range containers {
						if c.Name == containerName {
							found = true
							break
						}
					}

					if !found {
						logger.Infof("Service %s has stopped", svc)
						return nil
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		},
	}
}
