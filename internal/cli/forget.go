package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/state"
)

var forgetCmd = &cobra.Command{
	Use:   "forget <path>...",
	Short: "Remove files from tracking",
	Long: `Remove files from statemate's tracking database.

The files at target remain untouched. Only the tracking entries are removed.
This is useful when you want statemate to stop managing files without
deleting them.

Paths are matched against the target, and may be absolute, start with ~, or be
relative to the current directory.

Supports wildcards (glob patterns) to forget multiple files at once. Quote a
pattern so the shell does not expand it first.

Example:
  mate forget ~/.config/nvim/init.lua
  mate forget '~/.config/nvim/*.lua'
  mate forget .config/nvim/init.lua
  mate forget ~/.config/app/file1.conf ~/.config/app/file2.conf`,
	Args:              cobra.MinimumNArgs(1),
	RunE:              runForget,
	ValidArgsFunction: completeTrackedFiles,
}

func init() {
	rootCmd.AddCommand(forgetCmd)
}

func runForget(cmd *cobra.Command, args []string) error {
	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	tracked, err := db.ListFiles()
	if err != nil {
		return fmt.Errorf("listing tracked files: %w", err)
	}

	var toForget []string
	for _, pattern := range args {
		pattern = forgetPattern(pattern)

		matches := matchTrackedFiles(tracked, pattern)
		if len(matches) == 0 {
			return fmt.Errorf("no tracked files match: %s", pattern)
		}
		toForget = append(toForget, matches...)
	}

	seen := make(map[string]bool)
	for _, path := range toForget {
		if seen[path] {
			continue
		}
		seen[path] = true

		if err := db.DeleteFile(path); err != nil {
			return fmt.Errorf("removing %s from database: %w", path, err)
		}
		fmt.Printf("Forgot %s (target file unchanged)\n", path)
	}

	return nil
}

// forgetPattern resolves an argument to the absolute form the database stores.
// A leading ~ becomes the home directory; anything else still relative is taken
// as relative to the current directory, since a target path typed from the
// directory holding it would otherwise match nothing.
func forgetPattern(pattern string) string {
	pattern = expandPath(pattern)
	if filepath.IsAbs(pattern) {
		return pattern
	}

	abs, err := filepath.Abs(pattern)
	if err != nil {
		return pattern
	}
	return abs
}

func matchTrackedFiles(tracked []*state.FileEntry, pattern string) []string {
	var matches []string

	for _, entry := range tracked {
		if entry.TargetPath == pattern {
			return []string{pattern}
		}
	}

	for _, entry := range tracked {
		matched, err := filepath.Match(pattern, entry.TargetPath)
		if err != nil {
			continue
		}
		if matched {
			matches = append(matches, entry.TargetPath)
		}
	}

	return matches
}
