package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/util"
)

var diffCmd = &cobra.Command{
	Use:               "diff [path]",
	Short:             "Show pending changes",
	Long: `Show full unified diff of pending changes.

The positional argument filters by file or path; use --source to limit the diff
to a single source.

Use --tool to specify an external diff tool (e.g., delta, difft, vimdiff), or
set 'diff_tool' in config. The tool is run as '<tool> <old> <new>' and its
output is captured, so it may include arguments but cannot be interactive.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runDiff,
	ValidArgsFunction: completePendingFiles,
}

func init() {
	diffCmd.Flags().StringP("tool", "t", "", "external diff tool to use")
	diffCmd.Flags().Bool("sudo", false, "use sudo to check files requiring elevated access")
	addScopeFlag(diffCmd)
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg, enc, tmplCtx := env.Config, env.Enc, env.TmplCtx

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()
	tree := p.Tree

	if tree.HasConflicts() {
		printConflicts(tree, color.RedString("Error:"))
		return fmt.Errorf("resolve conflicts before diffing")
	}

	useSudo, _ := cmd.Flags().GetBool("sudo")
	result, err := p.Changes(useSudo)
	if err != nil {
		return err
	}
	changes := result.Changes

	scope, err := scopeFrom(cmd, args)
	if err != nil {
		return err
	}
	if err := scope.Validate(env, tree.Files()); err != nil {
		return err
	}

	if !scope.IsZero() {
		var filtered []*target.Change
		for _, c := range changes {
			if scope.Matches(c.Entry, cfg.SourceDir()) {
				filtered = append(filtered, c)
			}
		}
		changes = filtered
	}

	if len(changes) == 0 {
		if len(result.Skipped) > 0 {
			fmt.Fprintf(os.Stderr, "%s %d file(s) skipped (permission denied, use --sudo to check)\n", color.YellowString("Warning:"), len(result.Skipped))
		}
		fmt.Println(color.GreenString("No changes"))
		return nil
	}

	diffTool, _ := cmd.Flags().GetString("tool")
	if diffTool == "" {
		diffTool = cfg.DiffTool
	}

	for _, change := range changes {
		fmt.Println(bold("=== " + util.ShortenPath(change.Entry.TargetPath) + " ==="))
		if change.Entry.IsDir {
			// Directories have no content to compare; status reports them.
			fmt.Println("(directory)")
			continue
		}

		diff, err := target.Diff(change, tmplCtx, enc, diffTool)
		if err != nil {
			return fmt.Errorf("generating diff for %s: %w", util.ShortenPath(change.Entry.TargetPath), err)
		}
		if diff == "" {
			fmt.Println("(no content difference, state mismatch only)")
		} else {
			fmt.Println(diff)
		}
	}

	return nil
}
