package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/hooks"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/util"
)

// Note: source import is still needed for source.Entry type

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show what apply would change",
	Long: `Show pending changes that would be made on apply.

Reports files to be created, modified, or in conflict, plus orphaned files,
missing packages, pending scripts, the hooks the changes would trigger or that
an earlier apply left pending, and secrets that need fetching.

An empty directory in a source -- one with no files under it -- is reported until
it exists on the target. Directories that hold files are not listed separately;
they arrive with those files.

Markers: '+' new, '~' modified, '!' conflict, '<' will be imported into the
source (an '#import' file whose target changed).

The positional argument filters by file or path; use --source to limit the
file report to a single source. Packages, scripts and secrets are always
reported in full.

--short prints one compact line for status bars, and nothing when there is
nothing to do. It covers files, orphans, scripts and secrets, not packages or
hooks.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runStatus,
	ValidArgsFunction: completePendingFiles,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("short", false, "compact output for statuslines (format: +N ~N !N <N ?N *N sN)")
	statusCmd.Flags().Bool("sudo", false, "use sudo to check files requiring elevated access")
	addScopeFlag(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg, profileName, sourcePaths, enc, tmplCtx := env.Config, env.Profile, env.SourcePaths, env.Enc, env.TmplCtx

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()
	tree := p.Tree

	if tree.HasConflicts() {
		printConflicts(tree, color.YellowString("Warning:"))
		fmt.Fprintln(os.Stderr)
	}

	db, err := p.DB()
	if err != nil {
		return err
	}

	useSudo, _ := cmd.Flags().GetBool("sudo")
	result, err := p.Changes(useSudo)
	if err != nil {
		return err
	}
	changes := result.Changes

	orphans, err := p.Orphans()
	if err != nil {
		return err
	}

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths)
	allScripts, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering scripts: %w", err)
	}

	profileChain := env.Chain
	pendingScripts, err := scripts.PendingScripts(
		allScripts.Automatic().ByProfile(profileChain), db, changedSources(result.Changes))
	if err != nil {
		return fmt.Errorf("checking pending scripts: %w", err)
	}

	var pendingSecrets int
	{
		if mgr := env.Secrets; mgr != nil {
			templateFiles := discoverTemplateFiles(cfg, profileName, sourcePaths)
			var decryptFn func([]byte) ([]byte, error)
			if enc != nil && enc.CanDecrypt() {
				decryptFn = enc.Decrypt
			}
			items := secrets.DiscoverByRendering(templateFiles, tmplCtx, decryptFn)
			cached := mgr.ListCached()
			for _, item := range items {
				if cached == nil {
					pendingSecrets++
				} else if _, ok := cached[item.Key.String()]; !ok {
					pendingSecrets++
				}
			}
		}
	}

	// Missing packages are informational only -- a package manager being
	// unavailable should never fail status. A failure is still reported, since
	// a broken .mate.yaml would otherwise make the packages section vanish.
	type missingPkgs struct {
		manager  string
		packages []string
	}
	var pendingPackages []missingPkgs
	syncResults, err := packages.ComputeSync(cfg, profileName, sourcePaths,
		packages.WithDirConfigRenderer(plan.DirConfigRenderer(tmplCtx)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s not checking packages: %v\n", color.YellowString("Warning:"), err)
	}
	for _, r := range syncResults {
		if missing := r.Missing(); len(missing) > 0 {
			pendingPackages = append(pendingPackages, missingPkgs{manager: r.Manager, packages: missing})
		}
	}

	scope, err := scopeFrom(cmd, args)
	if err != nil {
		return err
	}
	if err := scope.Validate(env, tree.Files()); err != nil {
		return err
	}

	var filteredChanges []*target.Change
	for _, c := range changes {
		if !scope.Matches(c.Entry, cfg.SourceDir()) {
			continue
		}
		filteredChanges = append(filteredChanges, c)
	}

	// Orphans are no longer in any source, so they can only be matched by path.
	// Under --source there is nothing to match against, so report none.
	var filteredOrphans []string
	for _, o := range orphans {
		switch {
		case scope.Source != "":
			continue
		case scope.Path != "" && !strings.Contains(o, scope.Path) && !strings.HasSuffix(o, "/"+scope.Path):
			continue
		}
		filteredOrphans = append(filteredOrphans, o)
	}

	// Hooks follow from the files apply would write: not imports (the target
	// is the source of truth) and not mode-only fixes.
	hookSet, err := hooks.Collect(cfg, sourcePaths, env.Scanner.DirConfig, allScripts)
	if err != nil {
		return fmt.Errorf("invalid hooks: %w", err)
	}
	var written []*source.Entry
	for _, c := range filteredChanges {
		if c.Status != target.StatusImport && !c.PermOnly {
			written = append(written, c.Entry)
		}
	}
	// Plus the hooks an earlier apply triggered but did not run.
	leftPending, err := db.PendingHooks()
	if err != nil {
		return fmt.Errorf("reading pending hooks: %w", err)
	}
	pendingHooks := hookSet.WithPending(hookSet.Trigger(hookChanges(written, sourcePaths), profileChain), leftPending, profileChain)

	short, _ := cmd.Flags().GetBool("short")
	if short {
		return printShortStatus(filteredChanges, filteredOrphans, pendingScripts, pendingSecrets)
	}

	if len(filteredChanges) == 0 && len(filteredOrphans) == 0 && len(pendingScripts) == 0 &&
		len(pendingHooks) == 0 && pendingSecrets == 0 && len(pendingPackages) == 0 {
		fmt.Println(color.GreenString("Everything is up to date"))
		return nil
	}

	printChanges(filteredChanges)

	if len(filteredOrphans) > 0 {
		fmt.Fprintf(os.Stderr, "\n%s orphaned files (no longer in source):\n", color.YellowString("Warning:"))
		for _, o := range filteredOrphans {
			fmt.Fprintf(os.Stderr, "  - %s\n", util.ShortenPath(o))
		}
		fmt.Fprintln(os.Stderr, "Run 'mate forget <path>' to remove from tracking, or delete the target file.")
	}

	if len(pendingScripts) > 0 {
		fmt.Println("\n" + bold("Pending scripts:"))
		for _, s := range pendingScripts {
			timing := "before"
			if s.Timing == scripts.TimingAfter {
				timing = "after"
			}
			noconfirm := ""
			if s.NoConfirm {
				noconfirm = ", noconfirm"
			}
			fmt.Printf("  %s (%s, %s%s)\n", s.Name, s.Frequency, timing, noconfirm)
			if s.Description != "" {
				fmt.Printf("      %s\n", s.Description)
			}
		}
	}

	if len(pendingHooks) > 0 {
		fmt.Println("\n" + bold("Pending hooks:"))
		for _, h := range pendingHooks {
			noconfirm := ""
			if !h.Confirms() {
				noconfirm = ", noconfirm"
			}
			fmt.Printf("  %s (%s%s)\n", h.Name, hooks.FileCount(len(h.Files)), noconfirm)
			if h.Description != "" {
				fmt.Printf("      %s\n", h.Description)
			}
		}
	}

	if len(pendingPackages) > 0 {
		fmt.Println("\n" + bold("Missing packages:"))
		for _, p := range pendingPackages {
			fmt.Printf("  %s: %s\n", p.manager, strings.Join(p.packages, ", "))
		}
	}

	if pendingSecrets > 0 {
		fmt.Printf("\n%s\n  %d secrets need refresh (run 'mate secrets fetch')\n", bold("Secrets:"), pendingSecrets)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintf(os.Stderr, "\n%s %d file(s) skipped (permission denied, use --sudo to check)\n", color.YellowString("Warning:"), len(result.Skipped))
	}

	return nil
}

// printChanges lists file changes with their markers, as status and apply show them.
func printChanges(changes []*target.Change) {
	if len(changes) == 0 {
		return
	}
	maxTargetLen := 0
	for _, c := range changes {
		targetDisplay := util.ShortenPath(c.Entry.TargetPath)
		if len(targetDisplay) > maxTargetLen {
			maxTargetLen = len(targetDisplay)
		}
	}

	fmt.Printf("  %-*s  %s\n", maxTargetLen, "TARGET", "SOURCE")
	for _, c := range changes {
		targetDisplay := util.ShortenPath(c.Entry.TargetPath)
		sourceDir := extractSourceDir(c.Entry)
		fmt.Printf("%s %-*s  %s\n", c.Status.Marker(), maxTargetLen, targetDisplay, sourceDir)
	}
}

func printShortStatus(changes []*target.Change, orphans []string, pending scripts.Scripts, pendingSecrets int) error {
	var added, modified, conflicts, imports int
	for _, c := range changes {
		switch c.Status {
		case target.StatusNew:
			added++
		case target.StatusModified:
			modified++
		case target.StatusConflict:
			conflicts++
		case target.StatusImport:
			imports++
		}
	}

	// No changes = no output (for statusline to hide)
	if added == 0 && modified == 0 && conflicts == 0 && imports == 0 &&
		len(orphans) == 0 && len(pending) == 0 && pendingSecrets == 0 {
		return nil
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("~%d", modified))
	}
	if conflicts > 0 {
		parts = append(parts, fmt.Sprintf("!%d", conflicts))
	}
	if imports > 0 {
		parts = append(parts, fmt.Sprintf("<%d", imports))
	}
	if len(orphans) > 0 {
		parts = append(parts, fmt.Sprintf("?%d", len(orphans)))
	}
	if len(pending) > 0 {
		parts = append(parts, fmt.Sprintf("*%d", len(pending)))
	}
	if pendingSecrets > 0 {
		parts = append(parts, fmt.Sprintf("s%d", pendingSecrets))
	}

	fmt.Println(strings.Join(parts, " "))
	return nil
}

func extractSourceDir(entry *source.Entry) string {
	srcDir := strings.TrimSuffix(entry.SourcePath, "/"+entry.RelPath)
	return filepath.Base(srcDir)
}

// changedSources collects the sources with pending changes, which is what
// schedules #onchange scripts.
func changedSources(changes []*target.Change) scripts.ChangedSources {
	names := make([]string, 0, len(changes))
	for _, c := range changes {
		names = append(names, extractSourceDir(c.Entry))
	}
	return scripts.NewChangedSources(names...)
}
