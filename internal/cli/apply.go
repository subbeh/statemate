package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/state"
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
                           packages, no secret fetch
  mate apply -s <source>   apply that source's files, run its scripts, and
                           prompt for its packages

The positional argument is always a file or path filter; --source is the only
way to select a source. Repo-root scripts are not run under --source, since
they apply to the whole repository.

Scripts due to run are confirmed individually before executing:

  [y]es    run the script
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

A file marked '#import' is not prompted about when only its target changed: the
target is treated as authoritative and copied back into the source. Use it for
files an application rewrites, such as ~/.claude/settings.json. If the source
changed too, the conflict prompt still appears.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runApply,
	ValidArgsFunction: completeManagedFiles,
}

var (
	dryRun    bool
	force     bool
	noScripts bool
	verbose   int
)

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	applyCmd.Flags().BoolVar(&force, "force", false, "overwrite modified targets and auto-confirm scripts")
	applyCmd.Flags().BoolVar(&noScripts, "no-scripts", false, "skip all scripts")
	applyCmd.Flags().CountVarP(&verbose, "verbose", "V", "increase verbosity (can be repeated)")
	addScopeFlag(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	if tree.HasConflicts() {
		fmt.Fprintln(os.Stderr, "Error: conflicting targets detected")
		for _, c := range tree.Conflicts {
			fmt.Fprintf(os.Stderr, "  %s defined in:\n", util.ShortenPath(c.TargetPath))
			for _, s := range c.Sources {
				fmt.Fprintf(os.Stderr, "    - %s\n", util.ShortenPath(s))
			}
		}
		return fmt.Errorf("resolve conflicts before applying")
	}

	profileChain := profile.InheritanceChain(cfg, profileName)
	if profileName != "" {
		tree = tree.FilterByProfile(profileChain)
	}

	// Narrow the run before anything is applied, so every later phase (files,
	// scripts, packages, orphans) sees the same scope.
	scope, err := scopeFrom(cmd, args)
	if err != nil {
		return err
	}
	if err := scope.validate(cfg, profileName, tree.Files()); err != nil {
		return err
	}
	tree = scope.FilterTree(tree, cfg.SourceDir())

	// Narrow the source paths too. Secret discovery and script discovery both
	// walk these directly, so leaving them unfiltered would make a scoped run
	// fetch secrets for templates it is never going to render.
	sourcePaths = scope.FilterSourcePaths(sourcePaths)

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	var enc *encrypt.AgeEncryptor
	if cfg.Age != nil {
		enc, err = encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
		if err != nil {
			return fmt.Errorf("setting up encryption: %w", err)
		}
	}

	var ctxOpts []template.ContextOption
	if enc != nil && enc.CanDecrypt() {
		ctxOpts = append(ctxOpts, template.WithDecrypt(enc.Decrypt))
	}

	tmplCtx, err := template.NewContext(cfg, profileName, ctxOpts...)
	if err != nil {
		return fmt.Errorf("creating template context: %w", err)
	}

	identitySource := ""
	if cfg.Age != nil {
		identitySource = cfg.Age.Identity
	}
	mgr, mgrErr := secrets.NewManager(enc, identitySource, cfg.SecretsCache)
	if mgrErr == nil {
		tmplCtx.SecretLookup = func(item, typ, field string) (string, error) {
			key := secrets.CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field}
			return mgr.Get(key)
		}

		// A file-scoped run deploys files and nothing else, so it does not reach
		// out to fetch secrets. Source scope still does, since its templates may
		// need them.
		if scope.Path == "" {
			if err := fetchMissingSecrets(cfg, mgr, enc, profileName, sourcePaths, dryRun, verbose); err != nil {
				return err
			}
		}
	}

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths)
	allScripts, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering scripts: %w", err)
	}
	allScripts = scopedScripts(allScripts, scope)

	// Compute pending changes before applying anything, so #onchange scripts see
	// the same set whether they run #before or #after -- once apply has written
	// the files there are no pending changes left to detect.
	pending, err := target.ComputeChanges(tree, db, target.ComputeOpts{TmplCtx: tmplCtx, Enc: enc})
	if err != nil {
		return fmt.Errorf("computing changes: %w", err)
	}

	executor := scripts.NewExecutor(db, tmplCtx, dryRun, verbose > 0).
		WithConfirmation(force, noScripts).
		WithChangedSources(changedSources(pending.Changes))

	beforeScripts := allScripts.Automatic().ByProfile(profileChain).ByTiming(scripts.TimingBefore)
	beforeScripts.Sort()

	if len(beforeScripts) > 0 {
		if verbose > 0 || dryRun {
			fmt.Println("Running before scripts...")
		}
		res, err := executor.Execute(beforeScripts)
		if err != nil {
			return err
		}
		warnSkippedScripts(res)

		// Reload config and template context after before scripts
		// (scripts may generate var_files like secrets)
		if !dryRun {
			cfg, err = config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("reloading config: %w", err)
			}
			tmplCtx, err = template.NewContext(cfg, profileName, ctxOpts...)
			if err != nil {
				return fmt.Errorf("reloading template context: %w", err)
			}
			if mgrErr == nil {
				tmplCtx.SecretLookup = func(item, typ, field string) (string, error) {
					key := secrets.CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field}
					return mgr.Get(key)
				}
			}
		}
	}

	applier := target.NewApplier(db, tmplCtx, enc, dryRun, force, verbose)
	result, err := applier.Apply(tree)
	if err != nil {
		return err
	}

	if err := promptMissingPackages(cfg, profileName, sourcePaths, dryRun, force, scope); err != nil {
		return err
	}

	afterScripts := allScripts.Automatic().ByProfile(profileChain).ByTiming(scripts.TimingAfter)
	afterScripts.Sort()

	if len(afterScripts) > 0 {
		if verbose > 0 || dryRun {
			fmt.Println("Running after scripts...")
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
			fmt.Printf("%s\n", strings.Join(parts, ", "))
		}
	}

	return nil
}

func fetchMissingSecrets(cfg *config.Config, mgr *secrets.Manager, enc *encrypt.AgeEncryptor, profileName string, sourcePaths []string, dryRun bool, verbose int) error {
	templateFiles := discoverTemplateFiles(cfg, sourcePaths)
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

	if verbose > 0 {
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
	fmt.Fprintf(os.Stderr, "\nWarning: %d script(s) skipped (no terminal to confirm on):\n", len(res.SkippedNoTTY))
	for _, s := range res.SkippedNoTTY {
		fmt.Fprintf(os.Stderr, "  - %s\n", s)
	}
	fmt.Fprintln(os.Stderr, "Use --force to run them, or --no-scripts to silence this warning.")
}

func promptMissingPackages(cfg *config.Config, profileName string, sourcePaths []string, dryRun bool, autoConfirm bool, scope Scope) error {
	// A file-scoped run touches files only.
	if scope.Path != "" {
		return nil
	}

	results, err := packages.ComputeSync(cfg, profileName, sourcePaths)
	if err != nil {
		return nil
	}

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

		fmt.Printf("\nMissing %s packages: %s\n", result.Manager, strings.Join(missing, ", "))
		if !autoConfirm {
			fmt.Print("Install? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return nil
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				continue
			}
		}

		if err := manager.Install(missing); err != nil {
			return fmt.Errorf("installing packages: %w", err)
		}
	}

	return nil
}
