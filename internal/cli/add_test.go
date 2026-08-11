package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
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

func osName() string {
	return runtime.GOOS
}
