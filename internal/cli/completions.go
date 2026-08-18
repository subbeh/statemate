package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

// completeFilePaths hands completion to the shell's own filesystem completion.
//
// Commands that resolve their argument as an ordinary path (absolute, or relative
// to the current directory) should use this. Offering a computed list instead
// suppresses shell completion via NoFileComp, so anything the list did not
// anticipate -- a file reached with ../, a var_file outside the source tree, a file
// not yet managed -- became impossible to complete even though the command accepts
// it. `mate edit` has always delegated this way; encrypt, decrypt, eval and rename
// did not, and offered almost nothing.
func completeFilePaths(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// One path argument only; a second is not accepted, so offer nothing.
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func completeTrackedFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	db, err := state.Open("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer func() { _ = db.Close() }()

	files, err := db.ListFiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	seen := make(map[string]bool)
	for _, arg := range args {
		seen[arg] = true
	}

	cwd, _ := os.Getwd()

	var completions []string
	for _, f := range files {
		if seen[f.TargetPath] {
			continue
		}
		if rel := relativeTo(f.TargetPath, cwd); rel != "" {
			completions = append(completions, rel)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeManagedFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	cwd, _ := os.Getwd()

	var completions []string
	for _, e := range tree.Files() {
		if rel := cwdRelativeCompletion(e, cwd, sourcePaths); rel != "" {
			completions = append(completions, rel)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeSources(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	seen := make(map[string]bool)
	var sources []string
	for _, s := range cfg.Sources {
		if !seen[s] {
			seen[s] = true
			sources = append(sources, s)
		}
	}
	for _, p := range cfg.Profiles {
		if p == nil {
			continue
		}
		for _, s := range p.Sources {
			if !seen[s] {
				seen[s] = true
				sources = append(sources, s)
			}
		}
	}
	return sources, cobra.ShellCompDirectiveNoFileComp
}

func completeProfiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for name := range cfg.Profiles {
		completions = append(completions, name)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeScripts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths)
	allScripts, err := discoverer.Discover()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, s := range allScripts {
		completions = append(completions, s.Name)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeOrphanedFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	if profileName != "" {
		tree = tree.FilterByProfile(profile.InheritanceChain(cfg, profileName))
	}

	db, err := state.Open("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer func() { _ = db.Close() }()

	orphans, err := findOrphans(db, tree)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	cwd, _ := os.Getwd()

	seen := make(map[string]bool)
	for _, arg := range args {
		seen[arg] = true
	}

	var completions []string
	for _, o := range orphans {
		if seen[o] {
			continue
		}
		if rel := relativeTo(o, cwd); rel != "" {
			completions = append(completions, rel)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeSourceDirs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	return sources, cobra.ShellCompDirectiveNoFileComp
}

// cwdRelativeCompletion returns a relative path for the entry if cwd is within
// the same source directory or target directory as the entry. Returns "" if no match.
func cwdRelativeCompletion(e *source.Entry, cwd string, sourcePaths []string) string {
	// Check if cwd is within or equal to the entry's source directory
	for _, sp := range sourcePaths {
		if !strings.HasPrefix(e.SourcePath, sp+"/") {
			continue
		}
		if cwd == sp || strings.HasPrefix(cwd, sp+"/") {
			rel, err := filepath.Rel(cwd, e.SourcePath)
			if err == nil {
				return rel
			}
		}
	}

	// Check if the entry's target is under cwd
	if strings.HasPrefix(e.TargetPath, cwd+"/") {
		rel, err := filepath.Rel(cwd, e.TargetPath)
		if err == nil {
			return rel
		}
	}

	return ""
}

// relativeTo returns path relative to base if path is under base, otherwise "".
func relativeTo(path, base string) string {
	if rel, ok := strings.CutPrefix(path, base+"/"); ok {
		return rel
	}
	return ""
}
