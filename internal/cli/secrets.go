package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets",
	Long:  "Fetch and inspect secrets referenced in templates",
}

var secretsFetchCmd = &cobra.Command{
	Use:   "fetch [pattern]",
	Short: "Fetch secrets from providers",
	Long: `Find every secret reference in templates, fetch the values from Bitwarden, and
store them in the encrypted cache.

Every reference is fetched again, not only missing ones. With a pattern, only
the item with exactly that name is fetched, or every item whose name starts
with a prefix ending in '*'.

Needs the bw CLI, logged in, and an age identity to encrypt the cache with. A
locked vault is unlocked for you.`,
	Example: `  mate secrets fetch
  mate secrets fetch 'github*'`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSecretsFetch,
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secrets referenced in templates and cache status",
	Long:  "List every secret reference found in templates, with whether the cache holds it.",
	RunE:  runSecretsList,
}

var secretsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show secrets that need fetching",
	Long:  "List the secret references the cache does not hold yet. 'mate apply' fetches these before deploying.",
	RunE:  runSecretsStatus,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
	secretsCmd.AddCommand(secretsFetchCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsStatusCmd)
}

func runSecretsFetch(cmd *cobra.Command, args []string) error {
	mgr, items, err := setupSecrets(cmd)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No secrets referenced in templates")
		return nil
	}

	var pattern string
	if len(args) > 0 {
		pattern = args[0]
	}

	if pattern != "" {
		var filtered []secrets.FetchItem
		for _, item := range items {
			if matchSecretsPattern(item.Item, pattern) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	fmt.Printf("Fetching %d secrets...\n", len(items))

	green := color.New(color.FgGreen).SprintFunc()
	dim := color.New(color.Faint).SprintFunc()
	mgr.SetProgress(func(key secrets.CacheKey, changed bool) {
		label := fmt.Sprintf("%s/%s/%s", key.Item, key.Type, key.Field)
		if changed {
			fmt.Printf("  %s %s\n", green("✓"), label)
		} else {
			fmt.Printf("  %s %s\n", dim("·"), label)
		}
	})

	result, err := mgr.Fetch(items)
	if err != nil {
		// Without an identity there is no cache to fall back on either.
		if errors.Is(err, secrets.ErrNoIdentity) {
			return err
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if ok, _ := util.Confirm("Continue with cached secrets? [y/n]: ", false); !ok {
			return err
		}
		return nil
	}

	fmt.Printf("Fetched %d secrets (%d changed, %d unchanged)\n",
		result.Total, result.Changed, result.Unchanged)
	return nil
}

func runSecretsList(cmd *cobra.Command, args []string) error {
	mgr, items, err := setupSecrets(cmd)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No secrets referenced in templates")
		return nil
	}

	cached := mgr.ListCached()

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	maxItem := len("ITEM")
	maxType := len("TYPE")
	maxField := len("FIELD")
	for _, item := range items {
		if len(item.Item) > maxItem {
			maxItem = len(item.Item)
		}
		if len(item.Type) > maxType {
			maxType = len(item.Type)
		}
		if len(item.Field) > maxField {
			maxField = len(item.Field)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %-16s  %s\n", maxItem, "ITEM", maxType, "TYPE", maxField, "FIELD", "LAST FETCHED", "STATUS")
	for _, item := range items {
		fetched := "-"
		var status string
		if cached != nil {
			if cv, ok := cached[item.Key.String()]; ok {
				fetched = cv.FetchedAt.Format("2006-01-02 15:04")
				status = green("cached")
			} else {
				status = yellow("missing")
			}
		} else {
			status = cyan("no cache")
		}

		fmt.Printf("%-*s  %-*s  %-*s  %-16s  %s\n", maxItem, item.Item, maxType, item.Type, maxField, item.Field, fetched, status)
	}

	return nil
}

func runSecretsStatus(cmd *cobra.Command, args []string) error {
	mgr, items, err := setupSecrets(cmd)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No secrets referenced in templates")
		return nil
	}

	cached := mgr.ListCached()

	var missing []secrets.FetchItem
	for _, item := range items {
		if cached == nil {
			missing = append(missing, item)
			continue
		}
		if _, ok := cached[item.Key.String()]; !ok {
			missing = append(missing, item)
		}
	}

	if len(missing) == 0 {
		fmt.Println("All secrets are cached")
		return nil
	}

	fmt.Println("Secrets needing fetch:")
	for _, item := range missing {
		fmt.Printf("  %s/%s/%s\n", item.Item, item.Type, item.Field)
	}
	fmt.Printf("\nRun 'mate secrets fetch' to fetch %d secrets\n", len(missing))

	return nil
}

func setupSecrets(cmd *cobra.Command) (*secrets.Manager, []secrets.FetchItem, error) {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	var enc *encrypt.AgeEncryptor
	if cfg.Age != nil {
		enc, err = encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
		if err != nil {
			return nil, nil, fmt.Errorf("setting up encryption: %w", err)
		}
	}

	mgr, err := secrets.NewManager(enc, cfg.SecretsCache)
	if err != nil {
		return nil, nil, fmt.Errorf("setting up secrets: %w", err)
	}

	// Discover all bitwarden() calls by rendering templates
	templateFiles := discoverTemplateFiles(cfg, profileName, sourcePaths)

	var decryptFn func([]byte) ([]byte, error)
	var ctxOpts []template.ContextOption
	if enc != nil && enc.CanDecrypt() {
		decryptFn = enc.Decrypt
		ctxOpts = append(ctxOpts, template.WithDecrypt(enc.Decrypt))
	}

	tmplCtx, err := template.NewContext(cfg, profileName, ctxOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("creating template context: %w", err)
	}

	items := secrets.DiscoverByRendering(templateFiles, tmplCtx, decryptFn)

	return mgr, items, nil
}

// discoverTemplateFiles lists the templates whose secrets mate apply would need:
// the files and scripts it would actually render, filtered by profile the same
// way apply filters them, so secrets are never fetched for another profile's
// files.
func discoverTemplateFiles(cfg *config.Config, profileName string, sourcePaths []string) []string {
	var files []string

	scanner := source.NewScannerWithIgnore(cfg.TargetBase, cfg.SourceDir(), nil, cfg.Ignore)
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return files
	}

	profileChain := profile.InheritanceChain(cfg, profileName)
	if profileName != "" {
		tree = tree.FilterByProfile(profileChain)
	}

	for _, entry := range tree.Files() {
		if entry.Attrs.Template {
			files = append(files, entry.SourcePath)
		}
	}

	// Template scripts, from the repo root and from every source's
	// .matescripts/. Discover them the way apply does rather than listing a
	// directory here, so the two cannot disagree on which scripts exist.
	// Manual scripts are kept: 'mate scripts run' renders them with the cache.
	if allScripts, err := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths).Discover(); err == nil {
		for _, s := range allScripts.ByProfile(profileChain) {
			if s.Template {
				files = append(files, s.Path)
			}
		}
	}

	// Scan source directory configs for generate directives
	for _, sourcePath := range sourcePaths {
		for _, name := range []string{".mate.yaml", ".mate.yml", ".mate.toml"} {
			path := filepath.Join(sourcePath, name)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}
	}

	return files
}

func matchSecretsPattern(item, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(item, prefix)
	}
	return item == pattern
}
