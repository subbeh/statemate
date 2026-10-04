package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runProfileCapture runs `mate profile` against cfgPath with the given
// --profile value and returns what it printed.
func runProfileCapture(t *testing.T, cfgPath, profileFlag string) string {
	t.Helper()

	cmd := &cobra.Command{Use: "profile", RunE: runProfile}
	cmd.Flags().String("config", cfgPath, "")
	cmd.Flags().String("profile", profileFlag, "")

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := runProfile(cmd, nil)
	_ = w.Close()
	os.Stdout = origStdout

	out, _ := io.ReadAll(r)
	_ = r.Close()
	if runErr != nil {
		t.Fatalf("mate profile: %v", runErr)
	}
	return string(out)
}

// `mate profile` must report the profile every other command uses. Those give
// the config's profile: precedence over $STATEMATE_PROFILE; `mate profile`
// checked the variable first and so named a profile nothing else applied.
func TestProfileMatchesOtherCommandsPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, "mate.yaml")
	cfg := "profile: work\nprofiles:\n  work: {}\n  home: {}\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STATEMATE_PROFILE", "home")

	out := runProfileCapture(t, cfgPath, "")
	if !strings.Contains(out, "Profile: work\n") || !strings.Contains(out, "config file") {
		t.Errorf("expected the config's profile to win over STATEMATE_PROFILE, got:\n%s", out)
	}

	// The flag still overrides both.
	out = runProfileCapture(t, cfgPath, "home")
	if !strings.Contains(out, "Profile: home\n") || !strings.Contains(out, "--profile flag") {
		t.Errorf("expected --profile to win, got:\n%s", out)
	}
}

func TestProfileReportsEnvironmentVariable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte("profiles:\n  home: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STATEMATE_PROFILE", "home")

	out := runProfileCapture(t, cfgPath, "")
	if !strings.Contains(out, "Profile: home\n") || !strings.Contains(out, "STATEMATE_PROFILE") {
		t.Errorf("expected the profile from STATEMATE_PROFILE, got:\n%s", out)
	}
}
