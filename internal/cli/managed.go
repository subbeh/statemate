package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

var managedCmd = &cobra.Command{
	Use:   "managed [path]",
	Short: "List all managed files",
	Long: `List all files in source directories that are managed by mate.

With no argument, lists every managed file. With an argument, filters the list:

  - A path to an existing file (absolute, or relative to the current directory)
    matches only that file, whether you give its target or its source path.
  - Anything else is treated as a name fragment, so 'mate managed nvim' lists
    every file in the nvim source.

Examples:
  mate managed                    # all managed files
  mate managed ~/.ssh/config      # just that file
  mate managed config             # that file if it exists here, else all matches
  mate managed nvim               # everything in the nvim source`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runManaged,
	ValidArgsFunction: completeSourceDirs,
}

func init() {
	rootCmd.AddCommand(managedCmd)
}

func runManaged(cmd *cobra.Command, args []string) error {
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

	allSources := profile.AllSources(cfg)
	allSourcePaths := cfg.ResolveSourcePaths(allSources)

	activeSources := profile.ResolveSources(cfg, profileName)
	activeSourceSet := make(map[string]bool)
	for _, s := range activeSources {
		activeSourceSet[s] = true
	}

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}
	tree, err := scanner.Scan(allSourcePaths)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	var filterPath string
	if len(args) > 0 {
		filterPath = args[0]
	}

	var data [][]string
	for _, e := range tree.Files() {
		srcDir := strings.TrimSuffix(e.SourcePath, "/"+e.RelPath)
		srcPath := filepath.Join(filepath.Base(srcDir), e.RelPath)

		if filterPath != "" && !matchesManagedFilter(e, srcPath, filterPath) {
			continue
		}

		active := isActiveForProfile(e, profileName, activeSourceSet, cfg.SourceDir())
		destPath := util.ShortenPath(e.TargetPath)
		attrs := formatAttrs(e.Attrs)

		status := ""
		if active {
			status = "*"
		}

		data = append(data, []string{destPath, srcPath, status, attrs})
	}

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{"TARGET", "SOURCE", "ACTIVE", "ATTRIBUTES"}),
		tablewriter.WithAlignment(tw.Alignment{tw.AlignLeft, tw.AlignLeft, tw.AlignCenter, tw.AlignLeft}),
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

// matchesManagedFilter reports whether an entry matches the user's filter.
//
// A filter that resolves to a real path (absolute, or relative to the current
// directory) matches only the entry with that exact target or source path. This
// makes a path an unambiguous lookup -- "mate managed ~/.ssh/config" from any
// directory returns exactly that file rather than every target ending in
// "/config".
//
// Anything else is treated as a name fragment and matched loosely, which keeps
// "mate managed nvim" listing a whole source.
func matchesManagedFilter(e *source.Entry, srcPath, filter string) bool {
	if abs, ok := resolveFilterPath(filter); ok {
		return resolveSymlinks(e.TargetPath) == abs || resolveSymlinks(e.SourcePath) == abs
	}

	// Match against source relative path
	if strings.HasPrefix(srcPath, filter) || strings.HasSuffix(srcPath, "/"+filter) {
		return true
	}
	// Match against target path (absolute or basename)
	if e.TargetPath == filter || strings.HasSuffix(e.TargetPath, "/"+filter) {
		return true
	}
	// Match against relative path within source
	if e.RelPath == filter || strings.HasPrefix(e.RelPath, filter+"/") || strings.HasSuffix(e.RelPath, "/"+filter) {
		return true
	}
	return false
}

// resolveFilterPath resolves a filter to an absolute path when it names an
// existing file, reporting false when it should be treated as a name fragment.
//
// A bare name like "config" is only treated as a path when it exists in the
// current directory; otherwise it stays a fragment so loose matching still works
// from unrelated directories.
func resolveFilterPath(filter string) (string, bool) {
	abs, err := expandToAbs(filter)
	if err != nil {
		return "", false
	}
	if _, err := os.Lstat(abs); err != nil {
		return "", false
	}
	return resolveSymlinks(abs), true
}

func isActiveForProfile(e *source.Entry, profileName string, activeSources map[string]bool, sourceDir string) bool {
	if e.Attrs.Profile != "" && e.Attrs.Profile != profileName {
		return false
	}

	for src := range activeSources {
		fullPath := src
		if !filepath.IsAbs(src) {
			fullPath = filepath.Join(sourceDir, src)
		}
		if strings.HasPrefix(e.SourcePath, fullPath+"/") || e.SourcePath == fullPath {
			return true
		}
	}

	return false
}

func formatAttrs(a source.Attrs) string {
	var parts []string

	if a.Profile != "" {
		parts = append(parts, "profile:"+a.Profile)
	}
	if a.Perm != 0 {
		parts = append(parts, fmt.Sprintf("perm:%04o", a.Perm))
	}
	if a.Owner != "" {
		parts = append(parts, "owner:"+a.Owner)
	}
	if a.Group != "" {
		parts = append(parts, "group:"+a.Group)
	}
	if a.Encrypted {
		parts = append(parts, "encrypted")
	}
	if a.Template {
		parts = append(parts, "template")
	}
	if a.Symlink {
		parts = append(parts, "symlink")
	}

	return strings.Join(parts, ", ")
}
