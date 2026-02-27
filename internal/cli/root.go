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

	// Lifecycle commands
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newCreateCmd())
	rootCmd.AddCommand(newRmCmd())
	rootCmd.AddCommand(newKillCmd())

	// Image commands
	rootCmd.AddCommand(newPullCmd())
	rootCmd.AddCommand(newPushCmd())

	// Run / Copy / Wait
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newCpCmd())
	rootCmd.AddCommand(newWaitCmd())

	// Inspection commands
	rootCmd.AddCommand(newTopCmd())
	rootCmd.AddCommand(newPortCmd())
	rootCmd.AddCommand(newImagesCmd())
	rootCmd.AddCommand(newStatsCmd())

	// Config / Version
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
