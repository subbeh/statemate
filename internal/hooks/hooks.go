// Package hooks runs commands and scripts after files matching a pattern are
// written or removed -- reloading systemd after a unit file changes, say.
package hooks

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/template"
)

// Hook is a configured hook together with where it was declared.
type Hook struct {
	*config.Hook
	// Name is the configured name, prefixed with the source name for a hook from
	// a source's .mate.yaml (e.g. "arch/keyd").
	Name string
	// SourceDir is the declaring source directory, empty for hooks from
	// mate.yaml or the local config. A source hook matches only that source's
	// files.
	SourceDir string

	steps []step
}

type step struct {
	run    string
	script *scripts.Script
}

// Scope names where the hook was declared: "repo", "local", or the source name.
func (h *Hook) Scope() string {
	switch {
	case h.SourceDir != "":
		return filepath.Base(h.SourceDir)
	case h.IsLocal():
		return "local"
	default:
		return "repo"
	}
}

// ActiveFor reports whether the hook applies under the given profile chain.
func (h *Hook) ActiveFor(profileChain []string) bool {
	if h.Profile == "" {
		return true
	}
	for _, p := range profileChain {
		if p == h.Profile {
			return true
		}
	}
	return false
}

// Matches reports whether a target path matches any of the hook's patterns.
func (h *Hook) Matches(target string) bool {
	for _, p := range h.Match {
		if Match(p, target) {
			return true
		}
	}
	return false
}

// Set is every hook known to a command, sorted by name.
type Set []*Hook

// Get returns the hook with the given name, or nil.
func (s Set) Get(name string) *Hook {
	for _, h := range s {
		if h.Name == name {
			return h
		}
	}
	return nil
}

// Collect gathers the hooks from the config (repo and local, already merged)
// and from each source's .mate.yaml. It validates them and resolves their script
// steps against the discovered scripts, so a broken hook fails the command up
// front rather than after the files have been written.
//
// Disabled hooks are included, for listing, but never trigger.
func Collect(cfg *config.Config, sourceDirs []string, dirConfig func(string) *config.DirConfig, all scripts.Scripts) (Set, error) {
	var set Set

	if err := config.ValidateHooks(cfg.Hooks); err != nil {
		return nil, err
	}
	for name, h := range cfg.Hooks {
		set = append(set, &Hook{Hook: h, Name: name})
	}

	for _, dir := range sourceDirs {
		dc := dirConfig(dir)
		if dc == nil {
			continue
		}
		if err := config.ValidateHooks(dc.Hooks); err != nil {
			return nil, fmt.Errorf("source %s: %w", filepath.Base(dir), err)
		}
		for name, h := range dc.Hooks {
			set = append(set, &Hook{Hook: h, Name: filepath.Base(dir) + "/" + name, SourceDir: dir})
		}
	}

	for _, h := range set {
		if !h.IsEnabled() {
			continue
		}
		for i, s := range h.Do {
			if s.Run != "" {
				if _, err := texttemplate.New("").Funcs(template.FuncMap(nil)).Parse(s.Run); err != nil {
					return nil, fmt.Errorf("hook %q: step %d: %w", h.Name, i+1, err)
				}
				h.steps = append(h.steps, step{run: s.Run})
				continue
			}
			script, err := resolveScript(s.Script, h.SourceDir, all)
			if err != nil {
				return nil, fmt.Errorf("hook %q: step %d: %w", h.Name, i+1, err)
			}
			h.steps = append(h.steps, step{script: script})
		}
	}

	sort.Slice(set, func(i, j int) bool { return set[i].Name < set[j].Name })
	return set, nil
}

// resolveScript finds a script by the name 'mate scripts list' shows. A source
// hook prefers its own source's script, then the repository-root one; anything
// else must be unique across the remaining sources.
func resolveScript(ref, hookSourceDir string, all scripts.Scripts) (*scripts.Script, error) {
	var own, root, other scripts.Scripts
	for _, s := range all {
		if s.Name != ref && filepath.Base(s.Path) != ref {
			continue
		}
		switch {
		case hookSourceDir != "" && s.SourceDir == hookSourceDir:
			own = append(own, s)
		case s.SourceDir == "":
			root = append(root, s)
		default:
			other = append(other, s)
		}
	}

	for _, tier := range []scripts.Scripts{own, root, other} {
		switch len(tier) {
		case 0:
			continue
		case 1:
			return tier[0], nil
		}
		var where []string
		for _, s := range tier {
			if s.SourceDir == "" {
				where = append(where, "repository root")
			} else {
				where = append(where, filepath.Base(s.SourceDir))
			}
		}
		return nil, fmt.Errorf("script %q is ambiguous (found in %s)", ref, strings.Join(where, ", "))
	}
	return nil, fmt.Errorf("script %q not found (see 'mate scripts list')", ref)
}

// Change is a target file that a command wrote or removed.
type Change struct {
	// Path is the absolute target path.
	Path string
	// SourceDir is the source directory the file belongs to, empty when unknown.
	SourceDir string
}

// Triggered is a hook due to run, with the files that triggered it.
type Triggered struct {
	*Hook
	Files []string
}

// Trigger returns the enabled, profile-active hooks that the changes trigger, in
// name order, each with its sorted matching files.
func (s Set) Trigger(changes []Change, profileChain []string) []*Triggered {
	var out []*Triggered
	for _, h := range s {
		if !h.IsEnabled() || !h.ActiveFor(profileChain) {
			continue
		}
		seen := make(map[string]bool)
		var files []string
		for _, c := range changes {
			if h.SourceDir != "" && c.SourceDir != h.SourceDir {
				continue
			}
			if seen[c.Path] || !h.Matches(c.Path) {
				continue
			}
			seen[c.Path] = true
			files = append(files, c.Path)
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		out = append(out, &Triggered{Hook: h, Files: files})
	}
	return out
}

// OwningSource returns the source directory a source path lives in, or "" when
// it is in none of them. The longest match wins, for sources nested in others.
func OwningSource(sourcePath string, sourceDirs []string) string {
	best := ""
	for _, dir := range sourceDirs {
		if (sourcePath == dir || strings.HasPrefix(sourcePath, dir+string(filepath.Separator))) && len(dir) > len(best) {
			best = dir
		}
	}
	return best
}
