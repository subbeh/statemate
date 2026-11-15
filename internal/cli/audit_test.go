package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/state"
)

// execute runs mate through Execute, which records the run for `mate audit`,
// and returns what it printed to stdout.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	rootCmd.SetArgs(args)
	runErr := Execute()
	rootCmd.SetArgs(nil)
	auditLog = nil
	_ = w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

// A run that changes something is recorded in the state database without a log
// file; a run that only fails is not; `mate audit` lists the recorded runs.
func TestAudit(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))

	db, err := state.Open("")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := db.SaveFile(&state.FileEntry{SourcePath: "/src/.zshrc", TargetPath: target, SourceHash: "h", AppliedHash: "h"}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	if out, err := execute(t, "audit"); err != nil || !strings.Contains(out, "No changes recorded yet.") {
		t.Fatalf("mate audit on an empty log = %q, %v", out, err)
	}
	if _, err := execute(t, "forget", target); err != nil {
		t.Fatalf("mate forget: %v", err)
	}
	if _, err := execute(t, "forget", target); err == nil {
		t.Fatal("forgetting an untracked file succeeded")
	}

	out, err := execute(t, "audit")
	if err != nil {
		t.Fatalf("mate audit: %v", err)
	}
	if n := strings.Count(out, "  mate "); n != 1 {
		t.Errorf("audit lists %d runs, want 1 (the failed run changed nothing):\n%s", n, out)
	}
	if !strings.Contains(out, "  forgot target=~/.zshrc\n") {
		t.Errorf("audit does not list the forget:\n%s", out)
	}
}
