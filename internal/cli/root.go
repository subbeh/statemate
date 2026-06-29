package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mate",
	Short: "Statemate - system configuration management",
	Long: `Statemate manages your dotfiles, system configuration, and packages declaratively.

Features:
  - Stow-style multi-directory source management
  - Profile-based configuration with auto-detection
  - Template rendering with Go text/template
  - Age encryption for sensitive files
  - Declarative package management (brew, pacman, yay)
  - System file management with permission control

Use "mate [command] --help" for more information about a command.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mate version %s\n", version)
	},
}

func SetVersion(v string) {
	version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default: mate.yaml in current directory)")
	rootCmd.PersistentFlags().StringP("profile", "p", "", "override auto-detected profile")
	rootCmd.AddCommand(versionCmd)
}
