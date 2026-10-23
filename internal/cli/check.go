package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/util"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if configuration is in sync",
	Long: `Exit 0 if every managed file is in sync, and 1 if any would change.

Only files count: pending changes and conflicts. Orphans, missing packages,
pending scripts and secrets do not. Hooks are validated as well, so a broken
hook fails the check, and so does any other error, such as a config that does
not load. Use -q to print nothing and rely on the exit code.`,
	Example: `  mate check -q || echo "dotfiles out of sync"`,
	RunE:    runCheck,
}

var checkQuiet bool

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().BoolVarP(&checkQuiet, "quiet", "q", false, "suppress output, only use exit code")
	checkCmd.Flags().Bool("sudo", false, "use sudo to check files requiring elevated access")
}

func runCheck(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()

	// mate apply runs the same check before it writes anything.
	if _, _, err := collectHooks(env.Config, env.SourcePaths, env.Scanner); err != nil {
		return err
	}

	if p.Tree.HasConflicts() {
		if !checkQuiet {
			printConflicts(p.Tree, color.RedString("Error:"))
		}
		os.Exit(1)
	}

	useSudo, _ := cmd.Flags().GetBool("sudo")
	result, err := p.Changes(useSudo)
	if err != nil {
		return err
	}
	changes := result.Changes

	if len(changes) == 0 {
		if !checkQuiet {
			fmt.Println(color.GreenString("OK:"), "configuration is in sync")
			if len(result.Skipped) > 0 {
				fmt.Fprintf(os.Stderr, "%s %d file(s) skipped (permission denied, use --sudo to check)\n", color.YellowString("Warning:"), len(result.Skipped))
			}
		}
		return nil
	}

	if !checkQuiet {
		fmt.Printf("%s %d file(s) would change\n", color.YellowString("DRIFT:"), len(changes))
		for _, change := range changes {
			fmt.Printf("  %s %s\n", change.Status.Marker(), util.ShortenPath(change.Entry.TargetPath))
		}
	}

	os.Exit(1)
	return nil
}
