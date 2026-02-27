package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "container-compose",
	Short: "Docker Compose compatible orchestration for Apple Container",
	Long: `container-compose reads docker-compose.yml files and orchestrates
multi-container applications using Apple's container runtime.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringP("file", "f", "", "Compose configuration file (default: docker-compose.yml)")
	rootCmd.PersistentFlags().StringP("project-name", "p", "", "Project name (default: directory name)")
	rootCmd.PersistentFlags().String("project-directory", "", "Alternate working directory")
	rootCmd.PersistentFlags().StringSlice("profile", nil, "Specify a profile to enable")
	rootCmd.PersistentFlags().String("env-file", "", "Specify an alternate environment file")

	rootCmd.AddCommand(newUpCmd())
	rootCmd.AddCommand(newDownCmd())
	rootCmd.AddCommand(newPsCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newExecCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
