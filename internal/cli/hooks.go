package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/hooks"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage hooks",
	Long: `List and run hooks.

A hook runs commands or scripts after files matching its patterns are written
by 'mate apply' or removed by 'mate clean', 'mate delete', or 'mate rename':

  hooks:
    systemd-reload:
      match: ["*.service", "*.timer"]
      do:
        - run: sudo systemctl daemon-reload

Each triggered hook runs once per command, however many files matched, after
packages and before #after scripts. Hooks are confirmed like scripts; --force
auto-confirms them and --no-scripts skips them.

Hooks declared in a source's .mate.yaml are named <source>/<name> and match
only that source's files.`,
}

var hooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all hooks",
	Long: `List every hook with where it is declared, its patterns, and its status.

SCOPE is 'repo' for mate.yaml, 'local' for the machine-local config, or the
source name. STATUS shows 'disabled' for a hook switched off with
'enabled: false', and 'n/a' when its profile is not active.`,
	Args: cobra.NoArgs,
	RunE: runHooksList,
}

var hooksRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a hook",
	Long: `Run a hook manually, ignoring its patterns.

There is no confirmation prompt, and no files triggered the run, so
STATEMATE_HOOK_FILES and .Files are empty.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runHooksRun,
	ValidArgsFunction: completeHooks,
}

func init() {
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksListCmd)
	hooksCmd.AddCommand(hooksRunCmd)

	hooksRunCmd.Flags().Bool("dry-run", false, "show what would be done without running")
	hooksRunCmd.Flags().BoolP("verbose", "v", false, "verbose output")
}

// hookEnv is what the hooks subcommands share: the config, the active profile,
// every hook, and the files the profile manages.
type hookEnv struct {
	cfg          *config.Config
	profileName  string
	profileChain []string
	sourcePaths  []string
	set          hooks.Set
	files        []*source.Entry
}

func loadHookEnv(cmd *cobra.Command) (*hookEnv, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}
	sourcePaths := cfg.ResolveSourcePaths(profile.ResolveSources(cfg, profileName))

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return nil, fmt.Errorf("creating scanner: %w", err)
	}
	// Scanning is what loads each source's .mate.yaml.
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return nil, fmt.Errorf("scanning sources: %w", err)
	}
	profileChain := profile.InheritanceChain(cfg, profileName)
	if profileName != "" {
		tree = tree.FilterByProfile(profileChain)
	}

	all, err := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths).Discover()
	if err != nil {
		return nil, fmt.Errorf("discovering scripts: %w", err)
	}
	set, err := hooks.Collect(cfg, sourcePaths, scanner.DirConfig, all)
	if err != nil {
		return nil, fmt.Errorf("invalid hooks: %w", err)
	}

	return &hookEnv{
		cfg:          cfg,
		profileName:  profileName,
		profileChain: profileChain,
		sourcePaths:  sourcePaths,
		set:          set,
		files:        tree.Files(),
	}, nil
}

func runHooksList(cmd *cobra.Command, args []string) error {
	env, err := loadHookEnv(cmd)
	if err != nil {
		return err
	}
	if len(env.set) == 0 {
		fmt.Println("No hooks configured")
		return nil
	}

	var data [][]string
	for _, h := range env.set {
		status := ""
		switch {
		case !h.IsEnabled():
			status = "disabled"
		case !h.ActiveFor(env.profileChain):
			status = "n/a"
		}
		data = append(data, []string{
			h.Name,
			h.Scope(),
			strings.Join(h.Match, ", "),
			h.Profile,
			strconv.Itoa(len(h.Do)),
			status,
			h.Description,
		})
	}

	// Same layout rules as 'mate scripts list': the description is trimmed to
	// the terminal and printed in full when piped.
	headers := []string{"NAME", "SCOPE", "MATCH", "PROFILE", "STEPS", "STATUS", "DESCRIPTION"}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			truncateDescriptions(data, descriptionBudget(data, headers, w))
		}
	}

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader(headers),
		tablewriter.WithRowAutoWrap(tw.WrapTruncate),
		tablewriter.WithAlignment(tw.Alignment{
			tw.AlignLeft, tw.AlignLeft, tw.AlignLeft, tw.AlignLeft,
			tw.AlignRight, tw.AlignLeft, tw.AlignLeft,
		}),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Separators: tw.SeparatorsNone,
				Lines:      tw.LinesNone,
			},
		}),
	)
	_ = table.Bulk(data)
	_ = table.Render()
	return nil
}

func runHooksRun(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")

	env, err := loadHookEnv(cmd)
	if err != nil {
		return err
	}

	h := env.set.Get(args[0])
	switch {
	case h == nil:
		return fmt.Errorf("hook not found: %s (see 'mate hooks list')", args[0])
	case !h.IsEnabled():
		return fmt.Errorf("hook %s is disabled", h.Name)
	case !h.ActiveFor(env.profileChain):
		return fmt.Errorf("hook %s requires profile %q", h.Name, h.Profile)
	}

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	tmplCtx, err := newTemplateContext(env.cfg, env.profileName)
	if err != nil {
		return fmt.Errorf("creating template context: %w", err)
	}

	executor := scripts.NewExecutor(db, tmplCtx, dryRun, verbose).WithConfirmation(true, false)
	runner := hooks.NewRunner(executor, tmplCtx, hooks.Options{
		DryRun:       dryRun,
		Verbose:      verbose,
		Force:        true,
		ProfileChain: env.profileChain,
	})
	if err := runner.RunOne(h); err != nil {
		return fmt.Errorf("hook %s failed: %w", h.Name, err)
	}
	return nil
}

// hookChanges turns the entries a command wrote into hook changes, noting which
// source each came from so source hooks only see their own files.
func hookChanges(entries []*source.Entry, sourcePaths []string) []hooks.Change {
	changes := make([]hooks.Change, 0, len(entries))
	for _, e := range entries {
		changes = append(changes, hooks.Change{
			Path:      e.TargetPath,
			SourceDir: hooks.OwningSource(e.SourcePath, sourcePaths),
		})
	}
	return changes
}

// runTriggeredHooks runs the hooks a set of changes triggers and reports hooks
// that were skipped. The returned error is set only when the user quits; failed
// hooks are in the result, so the caller can finish its work before failing.
func runTriggeredHooks(runner *hooks.Runner, set hooks.Set, changes []hooks.Change, chain []string, announce bool) (*hooks.Result, error) {
	triggered := set.Trigger(changes, chain)
	if len(triggered) == 0 {
		return nil, nil
	}
	if announce {
		fmt.Println("Running hooks...")
	}

	// Report what was skipped even when the user quit part-way, since those
	// hooks will not trigger again on their own.
	res, err := runner.Run(triggered)
	for _, name := range res.Declined {
		fmt.Printf("  skipped hook %s; run it later with 'mate hooks run %s'\n", name, name)
	}
	if len(res.SkippedNoTTY) > 0 {
		fmt.Fprintf(os.Stderr, "\nWarning: %d hook(s) skipped (no terminal to confirm on):\n", len(res.SkippedNoTTY))
		for _, s := range res.SkippedNoTTY {
			fmt.Fprintf(os.Stderr, "  - %s\n", s)
		}
		fmt.Fprintln(os.Stderr, "The files are already in place, so these will not trigger again.")
		fmt.Fprintln(os.Stderr, "Run them with 'mate hooks run <name>', or pass --force next time.")
	}
	return res, err
}

// runRemovalHooks runs the hooks triggered by files that clean, delete, or
// rename changed on disk. Those commands have no --no-scripts, so --force alone
// decides whether hooks are confirmed.
func runRemovalHooks(cfg *config.Config, profileName string, sourcePaths []string, scanner *source.Scanner, db *state.DB, changes []hooks.Change, force bool) error {
	if len(changes) == 0 {
		return nil
	}

	all, err := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths).Discover()
	if err != nil {
		return fmt.Errorf("discovering scripts: %w", err)
	}
	set, err := hooks.Collect(cfg, sourcePaths, scanner.DirConfig, all)
	if err != nil {
		return fmt.Errorf("invalid hooks: %w", err)
	}

	tmplCtx, err := newTemplateContext(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating template context: %w", err)
	}

	chain := profile.InheritanceChain(cfg, profileName)
	executor := scripts.NewExecutor(db, tmplCtx, false, false).WithConfirmation(force, false)
	runner := hooks.NewRunner(executor, tmplCtx, hooks.Options{Force: force, ProfileChain: chain})

	res, err := runTriggeredHooks(runner, set, changes, chain, false)
	if err != nil {
		return err
	}
	return res.Err()
}

func completeHooks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	env, err := loadHookEnv(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, h := range env.set {
		if strings.HasPrefix(h.Name, toComplete) {
			names = append(names, h.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
