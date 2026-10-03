package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

// withStdin replaces os.Stdin with a pipe holding input for the duration of
// the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
}

// initSandbox runs the test in a fresh repository directory on a machine with
// no local config, returning the repository directory as init sees it.
func initSandbox(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "dotfiles")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("STATEMATE_DIR", "")

	origDir, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

func registeredSourceDir(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(config.LocalConfigPath())
	if err != nil {
		t.Fatalf("local config not written: %v", err)
	}
	t.Logf("local config:\n%s", data)
	return config.LocalSourceDir()
}

// On a fresh machine nothing is registered yet, so init must offer to register
// the new repository. It used to compare against the source dir resolved with
// the current-directory fallback, which always matched, and skip the prompt.
func TestInitRegistersRepoOnFreshMachine(t *testing.T) {
	repo := initSandbox(t)

	initFormat = "yaml"
	t.Cleanup(func() { initFormat = "" })
	withStdin(t, "\n") // Enter accepts the [Y/n] default

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if got := registeredSourceDir(t); got != repo {
		t.Errorf("registered source_dir = %q, want %q", got, repo)
	}
}

// Piped answers to both prompts -- format, then registration -- must each be
// consumed by their own prompt.
func TestInitPipedAnswers(t *testing.T) {
	repo := initSandbox(t)
	withStdin(t, "yaml\ny\n")

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "mate.yaml")); err != nil {
		t.Errorf("mate.yaml not created: %v", err)
	}
	if got := registeredSourceDir(t); got != repo {
		t.Errorf("registered source_dir = %q, want %q", got, repo)
	}
}

// mate.yml is a config name every other command accepts, so init must treat it
// as an existing repository rather than create a second config next to it.
func TestInitDetectsExistingMateYml(t *testing.T) {
	repo := initSandbox(t)
	if err := os.WriteFile(filepath.Join(repo, "mate.yml"), []byte("sources: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	withStdin(t, "")

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "mate.yaml")); err == nil {
		t.Error("init created mate.yaml next to an existing mate.yml")
	}
	if got := registeredSourceDir(t); got != repo {
		t.Errorf("registered source_dir = %q, want %q", got, repo)
	}
}

// A local config that exists but holds no source_dir (only, say, a profile)
// does not register anything, so an existing repository must still be
// registered rather than reported as already registered.
func TestInitExistingRepoWithLocalConfigWithoutSourceDir(t *testing.T) {
	repo := initSandbox(t)
	if err := os.WriteFile(filepath.Join(repo, "mate.yaml"), []byte("sources: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(config.LocalConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalConfigPath(), []byte("profile: work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	withStdin(t, "")

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if got := registeredSourceDir(t); got != repo {
		t.Errorf("registered source_dir = %q, want %q", got, repo)
	}
}
