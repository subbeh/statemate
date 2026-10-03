package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// $EDITOR and diff_tool are commonly set with arguments -- "code --wait",
// "delta --side-by-side" -- and occasionally with a quoted path. Both have to
// reach the program as the user wrote them, followed by mate's own arguments.
func TestUserCommand_PassesArgumentsThrough(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "my tool")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s|' \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := UserCommand(`"`+script+`" --wait`, "a file", "b").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "--wait|a file|b|"; got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}
}

func TestUserCommand_PlainProgramName(t *testing.T) {
	out, err := UserCommand("echo", "x").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "x" {
		t.Errorf("got %q", out)
	}
}
