package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/secrets"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "mate",
	Version: version,
	Short:   "Statemate - system configuration management",
	Long: `Statemate manages your dotfiles, system configuration, and packages declaratively.

Features:
  - Multi-directory source management, one directory per application
  - Profile-based configuration with auto-detection
  - Template rendering with Go text/template
  - Age encryption for sensitive files
  - Declarative package management (brew, pacman, and the AUR via yay or paru)
  - System file management with permission control

Use "mate [command] --help" for more information about a command.`,
	// By the time this runs, flags and arguments have been parsed and validated,
	// so any later error is a runtime one -- no config, a conflict, a broken hook
	// -- and the usage block would only bury it. Flag and argument errors happen
	// earlier and still print the usage.
	//
	// Runtime errors are printed by Execute rather than cobra, so they can be
	// shortened first.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
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
	rootCmd.Version = v
}

func Execute() error {
	cmd, err := rootCmd.ExecuteC()
	if err != nil && cmd.SilenceErrors {
		cmd.PrintErrln("Error:", errorMessage(err))
	}
	return err
}

// errorMessage shortens errors whose wrapping only buries the fix. A missing
// secret is reported from deep inside template rendering, behind the file, the
// template line and the function call, when all that matters is which secret it
// is and that 'mate secrets fetch' caches it. A missing identity likewise only needs
// its own message.
func errorMessage(err error) string {
	var notCached *secrets.NotCachedError
	if errors.As(err, &notCached) {
		return notCached.Error()
	}
	if errors.Is(err, secrets.ErrNoIdentity) {
		return secrets.ErrNoIdentity.Error()
	}
	return err.Error()
}

func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)")
	rootCmd.PersistentFlags().StringP("profile", "p", "", "override auto-detected profile")
	rootCmd.AddCommand(versionCmd)
	// --version (which the Homebrew formula test runs) prints exactly what
	// `mate version` does.
	rootCmd.SetVersionTemplate("mate version {{.Version}}\n")

	_ = rootCmd.RegisterFlagCompletionFunc("profile", completeProfiles)
}
