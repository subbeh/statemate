package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
)

// The interactive source picker shows one list and indexes into another. If the
// two ever diverge, a selection silently maps to the wrong source -- so they must
// be derived from the same resolved list, not from cfg.Sources.
func TestSourcePickerListMatchesIndexedList(t *testing.T) {
	repo := t.TempDir()

	for _, d := range []string{"app", "extra"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// "extra" is contributed by a profile, not by the top-level sources list.
	cfgContent := "target_base: " + t.TempDir() + "\n" +
		"sources:\n  - app\n" +
		"profiles:\n" +
		"  here:\n" +
		"    detection:\n" +
		"      os: " + osName() + "\n" +
		"    sources:\n      - extra\n"

	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	profileName := profile.Detect(cfg)
	if profileName == "" {
		t.Skip("profile did not match on this platform")
	}

	sources := profile.ResolveSources(cfg, profileName)
	absSources := cfg.ResolveSourcePaths(sources)

	// This is the invariant: runAdd shows `sources` and indexes `absSources`.
	if len(sources) != len(absSources) {
		t.Fatalf("picker list (%d) and indexed list (%d) differ in length", len(sources), len(absSources))
	}

	// The profile-provided source must be offered at all -- showing cfg.Sources
	// would omit it.
	if len(sources) <= len(cfg.Sources) {
		t.Errorf("expected profile sources to extend cfg.Sources (%v), got %v", cfg.Sources, sources)
	}

	var found bool
	for _, s := range sources {
		if s == "extra" {
			found = true
		}
	}
	if !found {
		t.Errorf("profile-provided source missing from picker list: %v", sources)
	}

	// Every displayed entry must resolve to the matching absolute path, so
	// selecting index i yields the source the user actually saw.
	for i, s := range sources {
		want := filepath.Join(cfg.SourceDir(), s)
		if absSources[i] != want {
			t.Errorf("index %d: showed %q but would use %q, want %q", i, s, absSources[i], want)
		}
	}
}

// Adding a file from outside the target base to a source with no .mate.yaml
// must not give the whole source a new target_base while it already holds home
// files: that would redeploy ~/.zshrc as /etc/.zshrc.
func TestAddOutsideTargetBaseRefusesSourceWithHomeFiles(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "shell")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".zshrc"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tree := &source.Tree{}
	tree.AddEntry(&source.Entry{
		SourcePath: filepath.Join(sourceDir, ".zshrc"),
		TargetPath: filepath.Join(home, ".zshrc"),
		RelPath:    ".zshrc",
	})

	// Confirm the .mate.yaml prompt, should it be (wrongly) shown.
	withStdin(t, "y\n")

	_, err := resolveTargetBaseForAdd(sourceDir, "/etc/hosts", home, tree, nil)
	if err == nil || !strings.Contains(err.Error(), "existing files") {
		t.Errorf("expected an error about the source's existing files, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(sourceDir, ".mate.yaml")); statErr == nil {
		t.Error(".mate.yaml with a new target_base was written for a source holding home files")
	}
}

func osName() string {
	return runtime.GOOS
}
