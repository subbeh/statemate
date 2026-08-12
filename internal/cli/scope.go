package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/source"
)

// Scope narrows a command to a single file/path or a single source.
//
// The positional argument is always a file/path filter and --source is the only
// way to select a source, because the two mean different amounts of work: a file
// scope touches files only, while a source scope also runs that source's scripts
// and packages. Inferring which was meant from a bare word would silently do the
// wrong thing.
type Scope struct {
	// Path is the positional file/path filter, empty when not given.
	Path string
	// Source is the --source value, empty when not given.
	Source string
}

// IsZero reports whether no narrowing was requested.
func (s Scope) IsZero() bool { return s.Path == "" && s.Source == "" }

// scopeFlagName is the long flag shared by apply, status, and diff. It matches
// the flag mate add already uses for the same concept.
const scopeFlagName = "source"

// addScopeFlag registers --source/-s on a command.
func addScopeFlag(cmd *cobra.Command) {
	cmd.Flags().StringP(scopeFlagName, "s", "", "limit to a single source")
	_ = cmd.RegisterFlagCompletionFunc(scopeFlagName, completeSources)
}

// scopeFrom reads the scope from a command's positional args and --source flag,
// rejecting the contradictory combination of both.
func scopeFrom(cmd *cobra.Command, args []string) (Scope, error) {
	var s Scope
	if len(args) > 0 {
		s.Path = args[0]
	}
	s.Source, _ = cmd.Flags().GetString(scopeFlagName)

	if s.Path != "" && s.Source != "" {
		return Scope{}, fmt.Errorf("cannot combine a path (%s) with --source (%s); use one or the other", s.Path, s.Source)
	}
	return s, nil
}

// Matches reports whether an entry falls inside the scope.
func (s Scope) Matches(entry *source.Entry, sourceDir string) bool {
	switch {
	case s.Source != "":
		return entrySourceName(entry) == s.Source
	case s.Path != "":
		// A bare source name must not match here: selecting a source is what
		// --source is for, and it does more work (scripts, packages) than a path
		// filter. Without this guard "mate apply nvim" would quietly apply the
		// source's files while skipping its scripts.
		if s.Path == entrySourceName(entry) {
			return false
		}
		return matchesPath(entry, s.Path, sourceDir)
	default:
		return true
	}
}

// entrySourceName returns the base name of the source directory an entry came
// from, e.g. "nvim" for nvim/.config/nvim/init.lua.
func entrySourceName(entry *source.Entry) string {
	srcDir := strings.TrimSuffix(entry.SourcePath, "/"+entry.RelPath)
	return filepath.Base(srcDir)
}

// validate checks the scope against what the tree actually contains, so a typo
// fails loudly rather than silently applying nothing.
//
// When a positional path matches nothing but does name a configured source, the
// error points at --source: that is exactly the moment someone trips over the
// positional/--source split.
func (s Scope) validate(cfg *config.Config, profileName string, entries []*source.Entry) error {
	if s.IsZero() {
		return nil
	}

	for _, e := range entries {
		if s.Matches(e, cfg.SourceDir()) {
			return nil
		}
	}

	if s.Source != "" {
		return fmt.Errorf("source %q has no files (configured sources: %s)",
			s.Source, strings.Join(profile.ResolveSources(cfg, profileName), ", "))
	}

	for _, name := range profile.ResolveSources(cfg, profileName) {
		if name == s.Path {
			return fmt.Errorf("no files match %q; did you mean --source %s?", s.Path, s.Path)
		}
	}
	return fmt.Errorf("no files match %q", s.Path)
}

// scopedScripts narrows which scripts an apply may run.
//
//   - No scope: everything, as before.
//   - File scope: nothing. Deploying one file should not trigger lifecycle hooks.
//   - Source scope: only that source's scripts. Repo-root scripts are excluded
//     because they apply to the whole repository, so running them for a single
//     source would overreach.
func scopedScripts(all scripts.Scripts, scope Scope) scripts.Scripts {
	switch {
	case scope.Path != "":
		return nil
	case scope.Source != "":
		var kept scripts.Scripts
		for _, s := range all {
			if s.SourceDir != "" && filepath.Base(s.SourceDir) == scope.Source {
				kept = append(kept, s)
			}
		}
		return kept
	default:
		return all
	}
}

// scopedPackages narrows which missing packages an apply may install.
//
// File scope installs nothing; source scope installs only packages contributed
// by that source's .mate.yaml. PackageStatus.Sources records the contributing
// source, so no restructuring is needed.
func scopedPackages(statuses []packages.PackageStatus, scope Scope) []packages.PackageStatus {
	switch {
	case scope.Path != "":
		return nil
	case scope.Source != "":
		var kept []packages.PackageStatus
		for _, st := range statuses {
			for _, src := range st.Sources {
				if src == scope.Source {
					kept = append(kept, st)
					break
				}
			}
		}
		return kept
	default:
		return statuses
	}
}

// missingNames returns the names of statuses that are not installed.
func missingNames(statuses []packages.PackageStatus) []string {
	var names []string
	for _, st := range statuses {
		if st.Status == packages.StatusMissing {
			names = append(names, st.Name)
		}
	}
	return names
}

// FilterTree returns a tree containing only the entries inside the scope.
func (s Scope) FilterTree(tree *source.Tree, sourceDir string) *source.Tree {
	if s.IsZero() {
		return tree
	}

	filtered := &source.Tree{Conflicts: tree.Conflicts}
	for _, e := range tree.Entries {
		if s.Matches(e, sourceDir) {
			filtered.Entries = append(filtered.Entries, e)
		}
	}
	return filtered
}
