// Package cli implements the container-compose command-line interface using cobra.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [REGISTRY]",
		Short: "Log in to a container registry",
		Long: `Log in to a container registry using Apple Container's registry login.

This is a convenience wrapper around 'container registry login'.
For private registries (e.g. Azure ACR, AWS ECR), you must log in
before using images from those registries in your compose files.

Examples:
  container-compose login myregistry.azurecr.io
  container-compose login --username user --password pass myregistry.azurecr.io
  echo "mytoken" | container-compose login --username user --password-stdin myregistry.azurecr.io`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := args[0]
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			passwordStdin, _ := cmd.Flags().GetBool("password-stdin")

			cmdArgs := []string{"registry", "login"}
			if username != "" {
				cmdArgs = append(cmdArgs, "--username", username)
			}

			// Apple Container only supports --password-stdin
			if password != "" || passwordStdin {
				cmdArgs = append(cmdArgs, "--password-stdin")
			}
			cmdArgs = append(cmdArgs, registry)

			c := exec.Command("container", cmdArgs...)
			if password != "" {
				c.Stdin = strings.NewReader(password)
			} else {
				c.Stdin = os.Stdin
			}
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Run(); err != nil {
				return fmt.Errorf("registry login failed: %w", err)
			}

			fmt.Printf("Login succeeded for %s\n", registry)
			return nil
		},
	}

	cmd.Flags().StringP("username", "u", "", "Registry username")
	cmd.Flags().String("password", "", "Registry password")
	cmd.Flags().Bool("password-stdin", false, "Read password from stdin")

	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [REGISTRY]",
		Short: "Log out from a container registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("container", "registry", "logout", args[0])
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Run(); err != nil {
				return fmt.Errorf("registry logout failed: %w", err)
			}

			fmt.Printf("Logout succeeded for %s\n", args[0])
			return nil
		},
	}
}
