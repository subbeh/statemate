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

// executeAdd runs `mate add` through the root command, as the binary does,
// resetting the flags it may set afterwards.
func executeAdd(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		for _, f := range []string{"config", "profile"} {
			_ = rootCmd.PersistentFlags().Set(f, "")
			rootCmd.PersistentFlags().Lookup(f).Changed = false
		}
		addForProfile, addSource, addEncrypt, addTemplate = "", "", false, false
	})
	rootCmd.SetArgs(append([]string{"add"}, args...))
	return rootCmd.Execute()
}

// addRepo creates a repository whose "extra" source is contributed only by the
// work profile, which nothing auto-detects, and a home file to add.
func addRepo(t *testing.T) (cfgPath, file string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STATEMATE_PROFILE", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := t.TempDir()
	for _, d := range []string{"base", "extra"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfgPath = filepath.Join(repo, "mate.yaml")
	cfg := "target_base: " + home + "\nsources: [base]\nprofiles:\n  work:\n    sources: [extra]\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	file = filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(file, []byte("[user]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, file
}

// The global -p/--profile must work on add like on every other command, and
// pick the sources add can choose from. A local --profile used to shadow it, so
// -p was an unknown flag and profile-only sources could not be added to.
func TestAddHonoursGlobalProfileFlag(t *testing.T) {
	cfgPath, file := addRepo(t)

	if err := executeAdd(t, "-c", cfgPath, "-p", "work", "-s", "extra", file); err != nil {
		t.Fatalf("mate add -p work -s extra: %v", err)
	}

	want := filepath.Join(filepath.Dir(cfgPath), "extra", ".gitconfig")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to be added without a profile suffix: %v", want, err)
	}
}

func TestAddForProfileAddsSuffix(t *testing.T) {
	cfgPath, file := addRepo(t)

	if err := executeAdd(t, "-c", cfgPath, "--for-profile", "work", "-s", "base", file); err != nil {
		t.Fatalf("mate add --for-profile work: %v", err)
	}

	want := filepath.Join(filepath.Dir(cfgPath), "base", ".gitconfig#profile:work")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s: %v", want, err)
	}
}

func osName() string {
	return runtime.GOOS
}
