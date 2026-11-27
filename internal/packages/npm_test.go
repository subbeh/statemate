package packages

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

// A trimmed `npm ls -g --depth=0 --json` from an Arch machine, where npm itself
// is installed in the global root.
const npmLsJSON = `{
  "name": "lib",
  "dependencies": {
    "pyright": {"version": "1.1.412", "overridden": false},
    "@anthropic-ai/claude-code": {"version": "2.1.0", "overridden": false},
    "npm": {"version": "12.2.3", "overridden": false},
    "corepack": {"version": "0.34.0", "overridden": false}
  }
}`

// fakeNPM puts a stub npm on PATH whose global root is root, and logs every
// invocation's arguments to the returned file.
func fakeNPM(t *testing.T, root string) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	script := `#!/bin/sh
echo "$@" >> "` + log + `"
case "$1" in
ls) printf '%s\n' '` + npmLsJSON + `' ;;
root) echo '` + root + `' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "npm"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return log
}

func TestNPMManager(t *testing.T) {
	log := fakeNPM(t, t.TempDir())
	m := NewNPMManager()
	if !m.IsAvailable() {
		t.Fatal("npm on PATH should be available")
	}

	installed, err := m.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	// npm and corepack ship with npm: never listed, so never pruned.
	want := []Package{{Name: "@anthropic-ai/claude-code", Version: "2.1.0"}, {Name: "pyright", Version: "1.1.412"}}
	if !reflect.DeepEqual(installed, want) {
		t.Errorf("ListInstalled() = %v, want %v", installed, want)
	}

	queried, err := m.QueryInstalled([]string{"@anthropic-ai/claude-code", "npm", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	wantQ := []Package{{Name: "@anthropic-ai/claude-code", Version: "2.1.0"}, {Name: "npm", Version: "12.2.3"}}
	if !reflect.DeepEqual(queried, wantQ) {
		t.Errorf("QueryInstalled() = %v, want %v", queried, wantQ)
	}

	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := m.Install([]string{"@anthropic-ai/claude-code", "pyright"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall([]string{"pyright"}); err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"install -g @anthropic-ai/claude-code pyright", "uninstall -g pyright"}
	if got := calls(t, log); !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("calls = %q, want %q", got, wantCalls)
	}
}

func TestNPMManagerDescribe(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name, "package.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("@anthropic-ai/claude-code", `{"name": "@anthropic-ai/claude-code", "description": "Claude Code"}`)
	write("pyright", `{"name": "pyright"}`)
	fakeNPM(t, root)

	descs, err := NewNPMManager().Describe([]string{"@anthropic-ai/claude-code", "pyright", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"@anthropic-ai/claude-code": "Claude Code"}; !reflect.DeepEqual(descs.ByName, want) {
		t.Errorf("ByName = %q, want %q", descs.ByName, want)
	}
	if _, resolved := descs.Lookup("missing"); !resolved {
		t.Error("a package that is not installed must not be reported unknown")
	}
}

// A declared @a/x must not hide an installed @b/x (or a bare x) from --prune.
func TestComputeSync_NPMScopedExtras(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "@a/x"}, {Name: "@b/x"}, {Name: "x"}}}
	withFakeManager(t, f)
	cfg := &config.Config{Packages: config.PackageList{{Name: "x", Managers: map[string]string{"brew": "@a/x"}}}}

	results, err := ComputeSync(cfg, "", nil, WithExtras(true))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := results[0].Extra(), []string{"@b/x", "x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Extra() = %v, want %v", got, want)
	}
}
