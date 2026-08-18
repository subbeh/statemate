package cli

import (
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/scripts"
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

func TestScopedScripts(t *testing.T) {
	all := scripts.Scripts{
		{Name: "n.sh", SourceDir: "/repo/nvim"},
		{Name: "z.sh", SourceDir: "/repo/zsh"},
		{Name: "r.sh", SourceDir: ""}, // repo-root script
	}

	t.Run("no scope keeps everything", func(t *testing.T) {
		if got := scopedScripts(all, Scope{}); len(got) != 3 {
			t.Errorf("expected 3 scripts, got %d", len(got))
		}
	})

	t.Run("file scope runs none", func(t *testing.T) {
		if got := scopedScripts(all, Scope{Path: "some/file"}); len(got) != 0 {
			t.Errorf("a file-scoped apply must run no scripts, got %d", len(got))
		}
	})

	t.Run("source scope excludes other sources and root", func(t *testing.T) {
		got := scopedScripts(all, Scope{Source: "nvim"})
		if len(got) != 1 || got[0].Name != "n.sh" {
			t.Fatalf("expected only n.sh, got %v", names(got))
		}
	})
}

func names(ss scripts.Scripts) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}

func TestScopedPackages(t *testing.T) {
	all := []packages.PackageStatus{
		{Name: "nvim-pkg", Status: packages.StatusMissing, Sources: []string{"nvim"}},
		{Name: "zsh-pkg", Status: packages.StatusMissing, Sources: []string{"zsh"}},
		{Name: "shared", Status: packages.StatusMissing, Sources: []string{"nvim", "zsh"}},
		{Name: "installed", Status: packages.StatusInstalled, Sources: []string{"nvim"}},
	}

	t.Run("no scope keeps everything", func(t *testing.T) {
		if got := scopedPackages(all, Scope{}); len(got) != 4 {
			t.Errorf("expected 4, got %d", len(got))
		}
	})

	t.Run("file scope keeps none", func(t *testing.T) {
		if got := scopedPackages(all, Scope{Path: "f"}); len(got) != 0 {
			t.Errorf("a file-scoped apply must install no packages, got %d", len(got))
		}
	})

	t.Run("source scope keeps that source's packages", func(t *testing.T) {
		got := scopedPackages(all, Scope{Source: "nvim"})
		// nvim-pkg, shared (also nvim), and installed (filtered later by status)
		if len(got) != 3 {
			t.Fatalf("expected 3 nvim packages, got %d", len(got))
		}
		missing := missingNames(got)
		if len(missing) != 2 {
			t.Errorf("expected 2 missing nvim packages, got %v", missing)
		}
		for _, m := range missing {
			if m == "zsh-pkg" {
				t.Error("zsh package leaked into nvim scope")
			}
		}
	})
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

func TestMissingNamesOnlyReportsMissing(t *testing.T) {
	got := missingNames([]packages.PackageStatus{
		{Name: "a", Status: packages.StatusMissing},
		{Name: "b", Status: packages.StatusInstalled},
		{Name: "c", Status: packages.StatusMissing},
	})
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("got %v, want [a c]", got)
	}
}
