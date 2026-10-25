package cli

import (
	"testing"

	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/scripts"
)

func TestScopedScripts(t *testing.T) {
	all := scripts.Scripts{
		{Name: "n.sh", SourceDir: "/repo/nvim"},
		{Name: "z.sh", SourceDir: "/repo/zsh"},
		{Name: "r.sh", SourceDir: ""}, // repo-root script
	}

	t.Run("no scope keeps everything", func(t *testing.T) {
		if got := scopedScripts(all, plan.Scope{}); len(got) != 3 {
			t.Errorf("expected 3 scripts, got %d", len(got))
		}
	})

	t.Run("file scope runs none", func(t *testing.T) {
		if got := scopedScripts(all, plan.Scope{Path: "some/file"}); len(got) != 0 {
			t.Errorf("a file-scoped apply must run no scripts, got %d", len(got))
		}
	})

	t.Run("source scope excludes other sources and root", func(t *testing.T) {
		got := scopedScripts(all, plan.Scope{Source: "nvim"})
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
		if got := scopedPackages(all, plan.Scope{}); len(got) != 4 {
			t.Errorf("expected 4, got %d", len(got))
		}
	})

	t.Run("file scope keeps none", func(t *testing.T) {
		if got := scopedPackages(all, plan.Scope{Path: "f"}); len(got) != 0 {
			t.Errorf("a file-scoped apply must install no packages, got %d", len(got))
		}
	})

	t.Run("source scope keeps that source's packages", func(t *testing.T) {
		got := scopedPackages(all, plan.Scope{Source: "nvim"})
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
