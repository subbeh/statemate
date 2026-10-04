package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBW puts a bw on PATH that records its arguments and the value of every
// --passwordenv variable it is pointed at, then prints a session key.
func fakeBW(t *testing.T) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "bw.log")
	script := `#!/bin/sh
echo "args: $*" >> "` + logPath + `"
while [ $# -gt 0 ]; do
  if [ "$1" = "--passwordenv" ]; then
    eval "echo \"env: \$$2\"" >> "` + logPath + `"
  fi
  shift
done
echo fake-session
`
	if err := os.WriteFile(filepath.Join(dir, "bw"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BW_SESSION", "")
	return logPath
}

func TestUnlockWithPassword_KeepsPasswordOffCommandLine(t *testing.T) {
	logPath := fakeBW(t)
	const password = "correct-horse-battery"

	if err := NewBitwardenProvider().unlockWithPassword([]byte(password)); err != nil {
		t.Fatalf("unlockWithPassword: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)

	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "args:") && strings.Contains(line, password) {
			t.Errorf("password passed as an argument, visible in the process list: %q", line)
		}
	}
	if !strings.Contains(log, "env: "+password) {
		t.Errorf("bw was not given the password via --passwordenv; log:\n%s", log)
	}
	// Later children (bw list, scripts) must not inherit the password.
	if got := os.Getenv(bwPasswordEnv); got != "" {
		t.Errorf("%s leaked into mate's own environment", bwPasswordEnv)
	}
	if got := os.Getenv("BW_SESSION"); got != "fake-session" {
		t.Errorf("BW_SESSION = %q, want fake-session", got)
	}
}
