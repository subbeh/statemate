package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/util"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/target"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if configuration is in sync",
	Long:  "Exit 0 if in sync, 1 if changes pending. Useful for CI.",
	RunE:  runCheck,
}

var checkQuiet bool

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().BoolVarP(&checkQuiet, "quiet", "q", false, "suppress output, only use exit code")
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	scanner := source.NewScanner(cfg.TargetBase)
	tree, err := scanner.Scan(cfg.AbsoluteSources())
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	if tree.HasConflicts() {
		if !checkQuiet {
			fmt.Fprintln(os.Stderr, "Error: conflicting targets detected")
			for _, c := range tree.Conflicts {
				fmt.Fprintf(os.Stderr, "  %s defined in:\n", util.ShortenPath(c.TargetPath))
				for _, s := range c.Sources {
					fmt.Fprintf(os.Stderr, "    - %s\n", util.ShortenPath(s))
				}
			}
		}
		os.Exit(1)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	if profileName != "" {
		tree = tree.FilterByProfile(profileName)
	}

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer db.Close()

	changes, err := target.ComputeChanges(tree, db)
	if err != nil {
		return fmt.Errorf("computing changes: %w", err)
	}

	if len(changes) == 0 {
		if !checkQuiet {
			fmt.Println("OK: configuration is in sync")
		}
		return nil
	}

	if !checkQuiet {
		fmt.Printf("DRIFT: %d file(s) would change\n", len(changes))
		for _, change := range changes {
			var prefix string
			switch change.Status {
			case target.StatusNew:
				prefix = "+"
			case target.StatusModified:
				prefix = "~"
			case target.StatusConflict:
				prefix = "!"
			}
			fmt.Printf("  %s %s\n", prefix, util.ShortenPath(change.Entry.TargetPath))
		}
	}

	os.Exit(1)
	return nil
}
