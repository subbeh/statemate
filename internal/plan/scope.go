package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subbeh/statemate/internal/profile"
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
	return filepath.Base(entry.SourceDir)
}

// Validate checks the scope against what the tree actually contains, so a typo
// fails loudly rather than silently applying nothing.
//
// When a positional path matches nothing but does name a configured source, the
// error points at --source: that is exactly the moment someone trips over the
// positional/--source split.
func (s Scope) Validate(env *Env, entries []*source.Entry) error {
	if s.IsZero() {
		return nil
	}

	cfg, profileName := env.Config, env.Profile
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

// FilterSourcePaths narrows a list of absolute source directories to the scope.
//
// Secret discovery and script discovery walk these paths directly rather than
// going through the tree, so they must be filtered too -- otherwise a scoped run
// discovers templates from other sources and tries to fetch their secrets.
//
// A file scope returns nothing: such a run deploys files and performs no
// discovery at all.
func (s Scope) FilterSourcePaths(paths []string) []string {
	switch {
	case s.Path != "":
		return nil
	case s.Source != "":
		for _, p := range paths {
			if filepath.Base(p) == s.Source {
				return []string{p}
			}
		}
		return nil
	default:
		return paths
	}
}

// Narrow limits the plan to the entries inside the scope, and fails when the
// scope matches none of them.
func (p *Plan) Narrow(s Scope) error {
	if err := s.Validate(p.env, p.Tree.Files()); err != nil {
		return err
	}
	if s.IsZero() {
		return nil
	}

	sourceDir := p.env.Config.SourceDir()
	filtered := &source.Tree{Conflicts: p.Tree.Conflicts}
	for _, e := range p.Tree.Entries {
		if s.Matches(e, sourceDir) {
			filtered.Entries = append(filtered.Entries, e)
		}
	}
	p.Tree = filtered
	return nil
}

func matchesPath(entry *source.Entry, pattern, sourceDir string) bool {
	relPath := filepath.Join(filepath.Base(entry.SourceDir), entry.RelPath)

	if relPath == pattern || entry.SourcePath == pattern || entry.TargetPath == pattern {
		return true
	}
	if strings.HasSuffix(relPath, "/"+pattern) ||
		strings.HasSuffix(entry.SourcePath, "/"+pattern) ||
		strings.HasSuffix(entry.TargetPath, "/"+pattern) {
		return true
	}
	if strings.HasPrefix(relPath, pattern+"/") ||
		strings.HasPrefix(entry.TargetPath, pattern+"/") {
		return true
	}
	// A relative directory is also taken from the current directory, so
	// `mate apply .config/nvim` run in ~ covers everything under ~/.config/nvim,
	// and `.config` run inside a source covers that source's .config.
	if !filepath.IsAbs(pattern) {
		if cwd, err := os.Getwd(); err == nil {
			abs := filepath.Join(cwd, pattern)
			if strings.HasPrefix(entry.TargetPath, abs+"/") || strings.HasPrefix(entry.SourcePath, abs+"/") {
				return true
			}
		}
	}
	return false
}
