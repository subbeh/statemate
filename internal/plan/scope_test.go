package plan

import (
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/source"
)

func entry(sourceName, rel string) *source.Entry {
	return &source.Entry{
		SourcePath: filepath.Join("/repo", sourceName, rel),
		TargetPath: filepath.Join("/home/u", rel),
		RelPath:    rel,
	}
}

func TestScope_SourceMatchesOnlyThatSource(t *testing.T) {
	nvim := entry("nvim", ".config/init.lua")
	zsh := entry("zsh", ".config/zshrc")

	s := Scope{Source: "nvim"}
	if !s.Matches(nvim, "/repo") {
		t.Error("expected nvim entry to match --source nvim")
	}
	if s.Matches(zsh, "/repo") {
		t.Error("zsh entry must not match --source nvim")
	}
}

// A bare source name given positionally must not match, so it can be reported
// with a --source suggestion rather than quietly applying files while skipping
// the source's scripts and packages.
func TestScope_PathDoesNotMatchBareSourceName(t *testing.T) {
	nvim := entry("nvim", ".config/init.lua")

	if (Scope{Path: "nvim"}).Matches(nvim, "/repo") {
		t.Error("a bare source name must not match as a path filter")
	}
}

func TestScope_PathMatchesRealPaths(t *testing.T) {
	e := entry("nvim", ".config/init.lua")

	for _, p := range []string{
		".config/init.lua",      // relative path within the source
		"init.lua",              // basename
		e.TargetPath,            // absolute target
		e.SourcePath,            // absolute source
		"nvim/.config/init.lua", // source-qualified relative path
	} {
		if !(Scope{Path: p}).Matches(e, "/repo") {
			t.Errorf("expected path %q to match", p)
		}
	}
}

func TestScope_ZeroMatchesEverything(t *testing.T) {
	var s Scope
	if !s.IsZero() {
		t.Error("expected zero scope")
	}
	if !s.Matches(entry("nvim", "a"), "/repo") || !s.Matches(entry("zsh", "b"), "/repo") {
		t.Error("a zero scope must match every entry")
	}
}

// Secret and script discovery walk the source paths directly rather than going
// through the tree, so those paths must be scoped too. Leaving them unfiltered
// made `mate apply -s env` try to fetch secrets for another source's templates.
func TestScope_FilterSourcePaths(t *testing.T) {
	paths := []string{"/repo/env", "/repo/restic", "/repo/nvim"}

	t.Run("no scope keeps everything", func(t *testing.T) {
		if got := (Scope{}).FilterSourcePaths(paths); len(got) != 3 {
			t.Errorf("expected all 3 paths, got %v", got)
		}
	})

	t.Run("source scope keeps only that source", func(t *testing.T) {
		got := (Scope{Source: "env"}).FilterSourcePaths(paths)
		if len(got) != 1 || got[0] != "/repo/env" {
			t.Errorf("expected only /repo/env, got %v", got)
		}
	})

	t.Run("file scope keeps none", func(t *testing.T) {
		if got := (Scope{Path: "some/file"}).FilterSourcePaths(paths); len(got) != 0 {
			t.Errorf("a file-scoped run performs no discovery, got %v", got)
		}
	})

	t.Run("unknown source keeps none", func(t *testing.T) {
		if got := (Scope{Source: "nope"}).FilterSourcePaths(paths); len(got) != 0 {
			t.Errorf("expected no paths for an unknown source, got %v", got)
		}
	})
}

// A directory given relative to the current directory scopes to the files under
// it, which is what completing a directory offers.
func TestMatchesPathRelativeDirectory(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(home)
	e := &source.Entry{
		SourcePath: "/repo/app/.config/app/app.conf",
		RelPath:    ".config/app/app.conf",
		TargetPath: filepath.Join(home, ".config", "app", "app.conf"),
	}
	for pattern, want := range map[string]bool{
		".config/app": true,
		".config":     true,
		".config/ap":  false,
		"elsewhere":   false,
	} {
		if got := matchesPath(e, pattern, "/repo"); got != want {
			t.Errorf("matchesPath(%q) = %v, want %v", pattern, got, want)
		}
	}
}
