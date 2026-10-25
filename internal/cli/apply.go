package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/hooks"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

var applyCmd = &cobra.Command{
	Use:   "apply [path]",
	Short: "Apply configuration to target",
	Long: `Apply files from source directories to their targets.

With no argument, applies everything. Otherwise the run is narrowed:

  mate apply <path>        apply matching files only -- no scripts, no
                           packages, no secret fetch; hooks still run
  mate apply -s <source>   apply that source's files, run its scripts, and
                           prompt for its packages

The positional argument is always a file or path filter, and a directory
covers every file under it, relative to the current directory or not; --source
is the only way to select a source. Repo-root scripts are not run under --source, since
they apply to the whole repository.

Before anything runs, the files about to change are listed (with the markers
of 'mate status') and the apply asks to proceed. --noconfirm skips the list and
the question; --force and --dry-run do too. Without a terminal to ask on, the
apply stops: pass --noconfirm for automated runs.

Scripts due to run are confirmed individually before executing:

  [Y]es    run the script (Enter also runs it)
  [n]o     skip this time; ask again on the next apply
  [s]kip   mark as done without running, so it is not offered again
           (not available for 'always' scripts, whose runs are never recorded)
  [a]ll    run this and auto-confirm the rest
  [q]uit   abort the apply

A script marked as done still appears in 'mate scripts list' and can be run
manually with 'mate scripts run'.

Use --force to auto-confirm all scripts, or --no-scripts to skip them entirely
(useful for automated runs). Without a terminal to prompt on, scripts are
skipped with a warning.

Hooks run for the files this apply wrote, after packages and before #after
scripts, and are confirmed the same way (without [s]kip). A failed hook does
not stop the others, but the apply exits non-zero. A hook that did not run, or
failed, stays pending and is offered again on the next apply; [n]o settles it.
See 'mate hooks'.

Missing secrets are fetched before anything is written, and missing packages
are offered for install after the files are written.

A target changed outside mate is a conflict, and is confirmed on its own:
[o]verwrite, [i]mport into the source, [s]kip, [d]iff, or [a]bort. --force
overwrites without asking; --dry-run still asks.

A file marked '#import' is not prompted about when only its target changed: the
target is treated as authoritative and copied back into the source. Use it for
files an application rewrites, such as ~/.claude/settings.json. If the source
changed too, the conflict prompt still appears.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runApply,
	ValidArgsFunction: completePendingFiles,
}

var (
	dryRun    bool
	force     bool
	noScripts bool
	noConfirm bool
	list      bool
)

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	applyCmd.Flags().BoolVar(&force, "force", false, "overwrite conflicting targets and auto-confirm scripts, hooks and package installs")
	applyCmd.Flags().BoolVar(&noScripts, "no-scripts", false, "skip all scripts")
	applyCmd.Flags().BoolVar(&noConfirm, "noconfirm", false, "apply without listing the changes and asking first")
	applyCmd.Flags().BoolVarP(&list, "list", "l", false, "list every file written, and with --dry-run the files and steps of each hook")
	addScopeFlag(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	// apply deploys #enc files and fetches secrets, so an identity that fails to
	// load stops it up front rather than at the first file that needs it.
	if err := env.CheckDecrypt(); err != nil {
		return fmt.Errorf("setting up encryption: %w", err)
	}
	cfg, profileName, profileChain, enc, tmplCtx := env.Config, env.Profile, env.Chain, env.Enc, env.TmplCtx
	sourcePaths := env.SourcePaths

	// Narrow the run before anything is applied, so every later phase (secrets,
	// files, scripts, packages, orphans) sees the same scope.
	scope, err := scopeFrom(cmd, args)
	if err != nil {
		return err
	}

	// Fetch missing secrets before scanning: a source's .mate.yaml is rendered
	// during the scan, and one that generates files from secrets fails to
	// render on a fresh machine whose cache does not have them yet. Discovery
	// reads .mate.yaml unrendered, so it does not depend on the cache itself.
	//
	// A file-scoped run deploys files and nothing else, so it does not reach
	// out to fetch secrets. Source scope still does, since its templates may
	// need them.
	if env.Secrets != nil && scope.Path == "" {
		if err := fetchMissingSecrets(cfg, env.Secrets, enc, profileName, scope.FilterSourcePaths(sourcePaths), dryRun, list); err != nil {
			return err
		}
	}

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()

	if p.Tree.HasConflicts() {
		printConflicts(p.Tree, color.RedString("Error:"))
		return fmt.Errorf("resolve conflicts before applying")
	}

	if err := p.Narrow(scope); err != nil {
		return err
	}
	tree := p.Tree

	// Narrow the source paths too. Secret discovery walks these directly, so
	// leaving them unfiltered would make a scoped run fetch secrets for
	// templates it is never going to render.
	//
	// Hooks are the exception: they are collected from every active source,
	// since a script: step may name a script anywhere in the repository and a
	// source's own hooks live in its .mate.yaml. Only the files the scoped run
	// writes decide which of them trigger.
	allSourcePaths := sourcePaths
	sourcePaths = scope.FilterSourcePaths(sourcePaths)

	// Collect hooks before fetching secrets, running scripts, or writing files,
	// so a broken hook stops the apply before it has done anything. Every
	// script is discovered, so hooks can resolve their script: steps, but only
	// those the scope allows run as #before/#after scripts.
	hookSet, discovered, err := collectHooks(cfg, allSourcePaths, env.Scanner)
	if err != nil {
		return err
	}
	allScripts := scopedScripts(discovered, scope)

	db, err := p.DB()
	if err != nil {
		return err
	}

	// Compute pending changes before applying anything, so #onchange scripts see
	// the same set whether they run #before or #after -- once apply has written
	// the files there are no pending changes left to detect.
	pending, err := p.Changes(false)
	if err != nil {
		return err
	}

	if !dryRun && !noConfirm && !force && len(pending.Changes) > 0 {
		printChanges(pending.Changes)
		ok, err := util.Confirm("\nApply these changes? [Y/n] ", true)
		if errors.Is(err, util.ErrInterrupted) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("no terminal to confirm on; use --noconfirm to apply without asking")
		}
		if !ok {
			return nil
		}
	}

	executor := scripts.NewExecutor(db, tmplCtx, dryRun).
		WithConfirmation(force, noScripts).
		WithChangedSources(changedSources(pending.Changes))

	beforeScripts := allScripts.Automatic().ByProfile(profileChain).ByTiming(scripts.TimingBefore)
	beforeScripts.Sort()

	if len(beforeScripts) > 0 {
		if dryRun {
			fmt.Println(bold("Running before scripts..."))
		}
		res, err := executor.Execute(beforeScripts)
		if err != nil {
			return err
		}
		warnSkippedScripts(res)

		// Reload config and template context after before scripts
		// (scripts may generate var_files like secrets)
		if !dryRun {
			if err := env.Reload(); err != nil {
				return err
			}
			cfg, tmplCtx = env.Config, env.TmplCtx
		}
	}

	applier := target.NewApplier(db, tmplCtx, enc, dryRun, force, list)
	applier.DiffTool = cfg.DiffTool
	result, err := applier.Apply(tree)
	if err != nil {
		return err
	}

	// The files are written now, so the hooks they trigger will not trigger
	// again. Record them as pending before anything else can stop the apply --
	// a package prompt, a hook prompt, Ctrl-C -- and offer the hooks an earlier
	// apply left pending alongside them.
	pendingHooks, err := db.PendingHooks()
	if err != nil {
		return fmt.Errorf("reading pending hooks: %w", err)
	}
	triggered := hookSet.WithPending(hookSet.Trigger(hookChanges(result.Written, allSourcePaths), profileChain), pendingHooks, profileChain)
	if !dryRun {
		for _, t := range triggered {
			if err := db.SetPendingHook(t.Name, t.Files); err != nil {
				return fmt.Errorf("recording pending hook %s: %w", t.Name, err)
			}
		}
	}

	if err := promptMissingPackages(cfg, profileName, sourcePaths, tmplCtx, dryRun, force, scope); err != nil {
		return err
	}

	hookRunner := hooks.NewRunner(executor, tmplCtx, hooks.Options{
		DryRun:       dryRun,
		Verbose:      list,
		Force:        force,
		NoScripts:    noScripts,
		ProfileChain: profileChain,
	})
	hookDB := db
	if dryRun {
		hookDB = nil // a dry run neither records nor clears pending hooks
	}
	hookRes, err := runTriggeredHooks(hookRunner, triggered, hookDB, list || dryRun)
	if err != nil {
		return err
	}

	afterScripts := allScripts.Automatic().ByProfile(profileChain).ByTiming(scripts.TimingAfter)
	afterScripts.Sort()

	if len(afterScripts) > 0 {
		if dryRun {
			fmt.Println(bold("Running after scripts..."))
		}
		res, err := executor.Execute(afterScripts)
		if err != nil {
			return err
		}
		warnSkippedScripts(res)
	}

	if dryRun {
		fmt.Printf("\nDry run: %d files would be applied", result.Applied)
		if result.Imported > 0 {
			fmt.Printf(", %d imported", result.Imported)
		}
		fmt.Printf(", %d unchanged\n", result.Skipped)
	} else {
		parts := []string{}
		if result.Applied > 0 {
			parts = append(parts, fmt.Sprintf("applied %d", result.Applied))
		}
		if result.Imported > 0 {
			parts = append(parts, fmt.Sprintf("imported %d", result.Imported))
		}
		if result.Skipped > 0 {
			parts = append(parts, fmt.Sprintf("%d unchanged", result.Skipped))
		}
		if len(parts) == 0 {
			fmt.Println("Nothing to do")
		} else {
			fmt.Println(color.GreenString(strings.Join(parts, ", ")))
		}
	}

	// A failed hook does not stop the other hooks or the #after scripts, but
	// the run still fails.
	return hookRes.Err()
}

func fetchMissingSecrets(cfg *config.Config, mgr *secrets.Manager, enc *encrypt.AgeEncryptor, profileName string, sourcePaths []string, dryRun, list bool) error {
	templateFiles := discoverTemplateFiles(cfg, profileName, sourcePaths)
	if len(templateFiles) == 0 {
		return nil
	}

	var decryptFn func([]byte) ([]byte, error)
	var ctxOpts []template.ContextOption
	if enc != nil && enc.CanDecrypt() {
		decryptFn = enc.Decrypt
		ctxOpts = append(ctxOpts, template.WithDecrypt(enc.Decrypt))
	}

	tmplCtx, err := template.NewContext(cfg, profileName, ctxOpts...)
	if err != nil {
		return nil
	}

	items := secrets.DiscoverByRendering(templateFiles, tmplCtx, decryptFn)
	if len(items) == 0 {
		return nil
	}

	cached := mgr.ListCached()
	var missing []secrets.FetchItem
	for _, item := range items {
		if cached == nil {
			missing = append(missing, item)
		} else if _, ok := cached[item.Key.String()]; !ok {
			missing = append(missing, item)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if dryRun {
		fmt.Printf("Would fetch %d missing secrets\n", len(missing))
		return nil
	}

	fmt.Printf("Fetching %d missing secrets...\n", len(missing))
	result, err := mgr.Fetch(missing)
	if err != nil {
		return fmt.Errorf("fetching secrets: %w", err)
	}

	if list {
		fmt.Printf("Fetched %d secrets (%d changed, %d unchanged)\n",
			result.Total, result.Changed, result.Unchanged)
	}

	return nil
}

// warnSkippedScripts reports scripts that were due but could not be confirmed
// because there was no terminal to prompt on.
func warnSkippedScripts(res *scripts.ExecuteResult) {
	if res == nil || len(res.SkippedNoTTY) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s %d script(s) skipped (no terminal to confirm on):\n", color.YellowString("Warning:"), len(res.SkippedNoTTY))
	for _, s := range res.SkippedNoTTY {
		fmt.Fprintf(os.Stderr, "  - %s\n", s)
	}
	fmt.Fprintln(os.Stderr, "Use --force to run them, or --no-scripts to silence this warning.")
}

func promptMissingPackages(cfg *config.Config, profileName string, sourcePaths []string, tmplCtx *template.Context, dryRun bool, autoConfirm bool, scope plan.Scope) error {
	// A file-scoped run touches files only.
	if scope.Path != "" {
		return nil
	}

	// Packages never fail an apply whose files are already written, but a
	// broken .mate.yaml must not make them disappear without a word.
	results, err := packages.ComputeSync(cfg, profileName, sourcePaths,
		packages.WithDirConfigRenderer(plan.DirConfigRenderer(tmplCtx)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s skipping packages: %v\n", color.YellowString("Warning:"), err)
		return nil
	}

	return installMissingPackages(results, cfg, dryRun, autoConfirm, scope)
}

func installMissingPackages(results []packages.SyncResult, cfg *config.Config, dryRun bool, autoConfirm bool, scope plan.Scope) error {
	// Installs that could not be confirmed because no answer could be read,
	// as under cron or CI. Once stdin has failed every later prompt would too,
	// so the rest are recorded without asking and reported together.
	var skippedNoTTY []string

	for _, result := range results {
		// Under --source, install only what that source declares. Filter the
		// statuses (which carry the contributing source) rather than the names.
		missing := missingNames(scopedPackages(result.Statuses, scope))
		if len(missing) == 0 {
			continue
		}

		manager, err := packages.GetManager(result.Manager, cfg.AURHelper)
		if err != nil {
			continue
		}

		if dryRun {
			fmt.Printf("\nMissing %s packages: %s\n", result.Manager, strings.Join(missing, ", "))
			continue
		}

		if len(skippedNoTTY) > 0 {
			skippedNoTTY = append(skippedNoTTY, result.Manager+": "+strings.Join(missing, ", "))
			continue
		}

		fmt.Printf("\nMissing %s packages: %s\n", result.Manager, strings.Join(missing, ", "))
		if !autoConfirm {
			ok, err := util.Confirm("Install? [y/N] ", false)
			if errors.Is(err, util.ErrInterrupted) {
				return nil
			}
			if err != nil {
				skippedNoTTY = append(skippedNoTTY, result.Manager+": "+strings.Join(missing, ", "))
				continue
			}
			if !ok {
				continue
			}
		}

		if err := manager.Install(missing); err != nil {
			return fmt.Errorf("installing packages: %w", err)
		}
		slog.Info("installed", "manager", manager.Name(), "packages", missing)
	}

	warnSkippedPackages(skippedNoTTY)
	return nil
}

// warnSkippedPackages reports package installs that could not be confirmed
// because there was no terminal to prompt on, matching warnSkippedScripts.
func warnSkippedPackages(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s %d package install(s) skipped (no terminal to confirm on):\n", color.YellowString("Warning:"), len(skipped))
	for _, s := range skipped {
		fmt.Fprintf(os.Stderr, "  - %s\n", s)
	}
	fmt.Fprintln(os.Stderr, "Use --force to install them.")
}
