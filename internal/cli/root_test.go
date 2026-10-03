package cli

import (
	"bytes"
	"strings"
	"testing"
)

// The Homebrew formula's test block runs `mate --version`, so the flag must
// exist and print the same line as `mate version`.
func TestVersionFlag(t *testing.T) {
	orig := version
	SetVersion("1.2.3")
	t.Cleanup(func() { SetVersion(orig) })

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("mate --version: %v", err)
	}
	if got, want := out.String(), "mate version 1.2.3\n"; got != want {
		t.Errorf("mate --version printed %q, want %q", got, want)
	}
}

// The --config default is not just the current directory: $STATEMATE_DIR and a
// registered source_dir come first, which the help must say.
func TestConfigFlagDescribesResolution(t *testing.T) {
	usage := rootCmd.PersistentFlags().Lookup("config").Usage
	for _, want := range []string{"$STATEMATE_DIR", "source_dir", "current directory"} {
		if !strings.Contains(usage, want) {
			t.Errorf("--config help %q does not mention %s", usage, want)
		}
	}
}

// A command that fails at runtime (no config found, a conflict, a broken hook)
// should print only its error. The usage block buried that error under a screen
// of flags, as if the command line itself had been wrong. A bad flag still gets
// the usage, since there it is the help that is needed.
func TestRuntimeErrorOmitsUsage(t *testing.T) {
	t.Setenv("STATEMATE_DIR", t.TempDir()) // no mate.yaml there
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	run := func(args ...string) string {
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs(args)
		t.Cleanup(func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
			rootCmd.SetArgs(nil)
		})
		// The commands are package globals, and the flag is set on the one that
		// ran, so it would otherwise carry over into the next Execute.
		defer func() { statusCmd.SilenceUsage = false }()
		if err := rootCmd.Execute(); err == nil {
			t.Fatalf("mate %v: expected an error", args)
		}
		return out.String()
	}

	if out := run("status"); strings.Contains(out, "Usage:") {
		t.Errorf("runtime error printed usage:\n%s", out)
	}
	if out := run("status", "--no-such-flag"); !strings.Contains(out, "Usage:") {
		t.Errorf("flag error did not print usage:\n%s", out)
	}
}
