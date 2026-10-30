package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

// NotManagedError is Resolve's error for a path that no entry deploys or comes
// from. Path is the file the argument names, for commands that also work on
// files mate does not manage, such as var_files.
type NotManagedError struct {
	Arg  string
	Path string
}

func (e *NotManagedError) Error() string {
	return fmt.Sprintf("%s is not managed by mate", e.Arg)
}

// Resolve finds the entry a file argument names, in any source. The argument is
// an ordinary path: absolute, under ~, or relative to the current directory. A
// relative path that does not exist there is also tried against the source
// directory. It names an entry by its source file, with or without the
// attributes, or by its target. There is no fuzzy matching: a bare name
// that is unique today turns ambiguous once another source adds the same name.
//
// When several entries match, as variants of one target for different profiles
// do, the one the current profile deploys wins.
func (e *Env) Resolve(arg string) (*source.Entry, error) {
	candidates, err := candidatePaths(arg, e.Config.SourceDir())
	if err != nil {
		return nil, err
	}
	tree, err := e.Scanner.Scan(e.Config.ResolveSourcePaths(profile.AllSources(e.Config)))
	if err != nil {
		return nil, fmt.Errorf("scanning sources: %w", err)
	}

	for _, p := range candidates {
		p = canonical(p)
		var matches []*source.Entry
		seen := make(map[string]bool)
		for _, en := range tree.Files() {
			// A generated entry has no source file to act on.
			if en.Generated || seen[en.SourcePath] {
				continue
			}
			src := canonical(en.SourcePath)
			if src == p || filepath.Join(filepath.Dir(src), en.Name) == p || canonical(en.TargetPath) == p {
				// A source deployed to several targets is still one file.
				seen[en.SourcePath] = true
				matches = append(matches, en)
			}
		}
		if len(matches) > 1 && e.Profile != "" {
			matches = (&source.Tree{Entries: matches}).FilterByProfile(e.Chain).Entries
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], nil
		default:
			var srcs []string
			for _, m := range matches {
				srcs = append(srcs, util.ShortenPath(m.SourcePath))
			}
			return nil, fmt.Errorf("%s matches several files, give the source file instead: %s", arg, strings.Join(srcs, ", "))
		}
	}

	path := candidates[0]
	for _, p := range candidates {
		if _, err := os.Lstat(p); err == nil {
			path = p
			break
		}
	}
	return nil, &NotManagedError{Arg: arg, Path: path}
}

// candidatePaths are the absolute paths arg can mean, in the order to try them.
func candidatePaths(arg, sourceDir string) ([]string, error) {
	abs, err := util.AbsPath(arg)
	if err != nil {
		return nil, err
	}
	paths := []string{abs}
	if !filepath.IsAbs(arg) && arg != "~" && !strings.HasPrefix(arg, "~/") {
		if _, err := os.Lstat(abs); err != nil {
			paths = append(paths, filepath.Join(sourceDir, arg))
		}
	}
	return paths, nil
}

// canonical resolves symlinks in a path's directory, but not in the file
// itself: a deployed #link target is the link, not what it points at. This
// keeps comparisons reliable where temp and home directories are symlinked.
func canonical(path string) string {
	dir := filepath.Dir(path)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return filepath.Join(dir, filepath.Base(path))
}
