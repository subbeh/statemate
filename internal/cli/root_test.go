package cli

import (
	"bytes"
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
