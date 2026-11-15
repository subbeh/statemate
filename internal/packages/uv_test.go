package packages

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeUV puts a stub uv on PATH that prints a real `uv tool list` output and
// logs every invocation's arguments, one call per line, to the returned file.
func fakeUV(t *testing.T) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	script := `#!/bin/sh
echo "$@" >> "` + log + `"
if [ "$1 $2" = "tool list" ]; then
	printf 'pre-commit v3.8.0\n- pre-commit\nruff v0.6.9\n- ruff\n'
fi
`
	if err := os.WriteFile(filepath.Join(dir, "uv"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return log
}

func calls(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestUVManager(t *testing.T) {
	log := fakeUV(t)
	m := NewUVManager()
	if !m.IsAvailable() {
		t.Fatal("uv on PATH should be available")
	}

	installed, err := m.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	want := []Package{{Name: "pre-commit", Version: "3.8.0"}, {Name: "ruff", Version: "0.6.9"}}
	if !reflect.DeepEqual(installed, want) {
		t.Errorf("ListInstalled() = %v, want %v", installed, want)
	}

	queried, err := m.QueryInstalled([]string{"ruff", "black"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Package{{Name: "ruff", Version: "0.6.9"}}; !reflect.DeepEqual(queried, want) {
		t.Errorf("QueryInstalled() = %v, want %v", queried, want)
	}

	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	// uv tool install takes one package, so each gets its own call.
	if err := m.Install([]string{"ruff", "black"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall([]string{"ruff", "black"}); err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"tool install ruff", "tool install black", "tool uninstall ruff black"}
	if got := calls(t, log); !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("calls = %q, want %q", got, wantCalls)
	}
}

func TestUVManagerUnavailable(t *testing.T) {
	fakePath(t)
	if NewUVManager().IsAvailable() {
		t.Error("uv not on PATH should be unavailable")
	}
}
