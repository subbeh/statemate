package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
)

var unmanagedCmd = &cobra.Command{
	Use:   "unmanaged [path]",
	Short: "List files that are not managed",
	Long: `List files under a directory that no source deploys.

With no argument, lists the current directory; otherwise the given directory.
A file counts as managed when any source deploys it, for any profile, matching
what 'mate managed' lists.

Only directories that contain managed files are descended into. A directory
holding nothing managed is listed once, with a trailing slash, rather than
file by file. Paths are relative to the current directory, so they can be passed
to 'mate add'.`,
	Example: `  mate unmanaged                  # unmanaged files here
  mate unmanaged ~/.config        # unmanaged files in ~/.config`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUnmanaged,
}

func init() {
	rootCmd.AddCommand(unmanagedCmd)
}

func runUnmanaged(cmd *cobra.Command, args []string) error {
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

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}
	tree, err := scanner.Scan(cfg.ResolveSourcePaths(profile.AllSources(cfg)))
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	root, err = expandToAbs(root)
	if err != nil {
		return err
	}

	targets := make([]string, 0, len(tree.Entries))
	for _, e := range tree.Entries {
		targets = append(targets, e.TargetPath)
	}

	paths, err := findUnmanaged(root, targets)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	for _, p := range paths {
		// Rel drops the trailing slash that marks a directory.
		if rel, err := filepath.Rel(cwd, p); err == nil {
			if strings.HasSuffix(p, string(filepath.Separator)) {
				rel += string(filepath.Separator)
			}
			p = rel
		}
		fmt.Println(p)
	}
	return nil
}

// findUnmanaged walks root and returns the absolute paths of everything no
// target accounts for. It only descends into directories that are a target or
// hold one, so an unmanaged directory is returned once, with a trailing slash.
//
// ponytail: symlinked directories are not followed, so managed files reached
// through one (a symlinked ~/.config) are neither listed nor descended into.
func findUnmanaged(root string, targets []string) ([]string, error) {
	// A target and all its ancestors are kept: files are managed, directories
	// are descended into.
	keep := make(map[string]bool)
	for _, t := range targets {
		for p := t; !keep[p]; p = filepath.Dir(p) {
			keep[p] = true
		}
	}

	var unmanaged []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			// An unreadable directory below root is skipped, not fatal.
			return nil
		}
		if path == root || keep[path] {
			return nil
		}
		if d.IsDir() {
			unmanaged = append(unmanaged, path+string(filepath.Separator))
			return filepath.SkipDir
		}
		unmanaged = append(unmanaged, path)
		return nil
	})
	return unmanaged, err
}
