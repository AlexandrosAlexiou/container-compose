// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/spf13/cobra"
)

func newNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage networks",
	}

	cmd.AddCommand(newNetworkLsCmd())
	cmd.AddCommand(newNetworkCreateCmd())
	cmd.AddCommand(newNetworkRmCmd())
	cmd.AddCommand(newNetworkPruneCmd())

	return cmd
}

func newNetworkLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			logger := output.NewLogger(os.Stdout, os.Stderr)
			d := driver.New(logger)

			return d.ListNetworksRaw(ctx)
		},
	}
}

func newNetworkCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create NETWORK",
		Short: "Create a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			logger := output.NewLogger(os.Stdout, os.Stderr)
			d := driver.New(logger)

			return d.CreateNetwork(ctx, args[0])
		},
	}
}

func newNetworkRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm NETWORK [NETWORK...]",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove one or more networks",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			logger := output.NewLogger(os.Stdout, os.Stderr)
			d := driver.New(logger)

			var errs []string
			for _, name := range args {
				if err := d.DeleteNetwork(ctx, name); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", name, err))
				} else {
					logger.Successf("Removed network %s", name)
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("failed to remove networks:\n%s", strings.Join(errs, "\n"))
			}
			return nil
		},
	}
}

func newNetworkPruneCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove all unused project networks",
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

			if !force {
				logger.Warnf("This will remove all networks defined in the compose file for project %q", project.Name)
				logger.Infof("Use --force to skip this warning")
				return nil
			}

			defaultNet := converter.NetworkName(project.Name, "default")
			removed := 0

			for name := range project.Networks {
				networkName := converter.NetworkName(project.Name, name)
				if err := d.DeleteNetwork(ctx, networkName); err != nil {
					logger.Warnf("Failed to remove %s: %v", networkName, err)
				} else {
					removed++
				}
			}
			// Also try the default network
			if err := d.DeleteNetwork(ctx, defaultNet); err == nil {
				removed++
			}

			logger.Successf("Removed %d network(s)", removed)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")
	return cmd
}
