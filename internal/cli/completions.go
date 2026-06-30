package cli

import (
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

func completeTrackedFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	db, err := state.Open("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer db.Close()

	files, err := db.ListFiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, f := range files {
		completions = append(completions, f.TargetPath)
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

	scanner := source.NewScanner(cfg.TargetBase, cfg.SourceDir())
	tree, err := scanner.Scan(sourcePaths)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, e := range tree.Files() {
		completions = append(completions, e.TargetPath)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeSources(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return cfg.Sources, cobra.ShellCompDirectiveNoFileComp
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

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), cfg.AbsoluteSources())
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
