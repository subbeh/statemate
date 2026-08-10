package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runConfigSourceDirCapture invokes the command with the given --config value,
// returning what it printed to stdout.
//
// A fresh cobra command is built per call: reusing the package-level command
// would keep the --config flag value from a previous test, since redefining an
// existing flag is a no-op.
func runConfigSourceDirCapture(t *testing.T, cfgPath string) (string, error) {
	t.Helper()

	cmd := &cobra.Command{Use: "source-dir", RunE: runConfigSourceDir}
	cmd.Flags().String("config", cfgPath, "")

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runConfigSourceDir(cmd, nil)

	_ = w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()

	return strings.TrimSpace(string(buf[:n])), runErr
}

func TestConfigSourceDir_PrintsConfigDirectory(t *testing.T) {
	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}

	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte("sources: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := runConfigSourceDirCapture(t, cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The source dir is the directory holding mate.yaml.
	if resolved, rerr := filepath.EvalSymlinks(got); rerr == nil {
		got = resolved
	}
	if got != repo {
		t.Errorf("got %q, want %q", got, repo)
	}
}

func TestConfigSourceDir_ErrorsWhenConfigMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope", "mate.yaml")

	if _, err := runConfigSourceDirCapture(t, missing); err == nil {
		t.Error("expected an error when the config file does not exist")
	}
}

// The output must be a bare path so it works in command substitution.
func TestConfigSourceDir_OutputIsBarePath(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte("sources: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := runConfigSourceDirCapture(t, cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == "" {
		t.Fatal("expected a path to be printed")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected an absolute path, got %q", got)
	}
	if strings.ContainsAny(got, ":\n") {
		t.Errorf("output should be a bare path with no label or extra lines, got %q", got)
	}
}
