package hooks

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Match reports whether a hook pattern matches an absolute target path.
//
// Patterns follow gitignore conventions, applied to target paths:
//
//   - No slash (`*.service`): matches a file or directory name at any depth.
//   - Relative with a slash (`.config/tmux/*.conf`): anchored at the home
//     directory, as is a leading `~/`.
//   - Absolute (`/etc/keyd/*.conf`): anchored at the root.
//   - `**` matches any number of path segments, and a trailing `/` matches
//     only what is inside a directory.
//
// A pattern that matches a directory matches everything under it, so
// `.config/tmux` covers `.config/tmux/tmux.conf`.
func Match(pattern, target string) bool {
	home, _ := os.UserHomeDir()
	return match(pattern, target, home)
}

func match(pattern, target, home string) bool {
	p := filepath.ToSlash(pattern)
	target = filepath.ToSlash(target)

	dirOnly := strings.HasSuffix(p, "/")
	p = strings.TrimRight(p, "/")
	if p == "" {
		return false
	}

	// Candidates are the target itself and each directory above it; a trailing
	// slash leaves out the target, which is a file.
	segs := strings.Split(strings.TrimPrefix(target, "/"), "/")
	last := len(segs)
	if dirOnly {
		last--
	}

	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.Contains(p, "/") {
		for i := 0; i < last; i++ {
			if ok, _ := path.Match(p, segs[i]); ok {
				return true
			}
		}
		return false
	}

	switch {
	case p == "~":
		p = filepath.ToSlash(home)
	case strings.HasPrefix(p, "~/"):
		p = filepath.ToSlash(home) + p[1:]
	case !strings.HasPrefix(p, "/"):
		p = filepath.ToSlash(home) + "/" + p
	}
	pSegs := strings.Split(strings.TrimPrefix(p, "/"), "/")

	for i := 1; i <= last; i++ {
		if matchSegments(pSegs, segs[:i]) {
			return true
		}
	}
	return false
}

// matchSegments matches path segments against pattern segments, where `**`
// stands for zero or more segments.
func matchSegments(pattern, segs []string) bool {
	if len(pattern) == 0 {
		return len(segs) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pattern[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, _ := path.Match(pattern[0], segs[0]); !ok {
		return false
	}
	return matchSegments(pattern[1:], segs[1:])
}
