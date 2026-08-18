package packages

import (
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

// fakeManager records whether the expensive full-inventory call was made.
type fakeManager struct {
	installed        []Package
	listInstalledHit int
}

func (f *fakeManager) Name() string      { return "brew" }
func (f *fakeManager) IsAvailable() bool { return true }

func (f *fakeManager) ListInstalled() ([]Package, error) {
	f.listInstalledHit++
	return f.installed, nil
}

func (f *fakeManager) QueryInstalled(pkgs []string) ([]Package, error) {
	var out []Package
	for _, want := range pkgs {
		for _, inst := range f.installed {
			if inst.Name == want {
				out = append(out, inst)
			}
		}
	}
	return out, nil
}

func (f *fakeManager) Describe([]string) (map[string]string, error) { return nil, nil }
func (f *fakeManager) Install([]string) error                       { return nil }
func (f *fakeManager) Uninstall([]string) error                     { return nil }

// withFakeManager swaps in a fake for the duration of a test, so no test shells
// out to a real package manager.
func withFakeManager(t *testing.T, f *fakeManager) {
	t.Helper()
	origGet, origAvail := getManager, availableManager
	getManager = func(string, string) (Manager, error) { return f, nil }
	availableManager = func(string) []Manager { return []Manager{f} }
	t.Cleanup(func() { getManager, availableManager = origGet, origAvail })
}

func syncConfig() *config.Config {
	return &config.Config{
		Packages: &config.PackageList{Brew: []string{"git", "ripgrep"}},
	}
}

// Listing every installed package is the slow half of a sync -- about a second
// for brew, which used to dominate the runtime of mate status and mate apply.
// Neither reports extras, so it must not happen unless asked for.
func TestComputeSync_SkipsListInstalledByDefault(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "git"}, {Name: "unrelated"}}}
	withFakeManager(t, f)

	results, err := ComputeSync(syncConfig(), "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if f.listInstalledHit != 0 {
		t.Errorf("ListInstalled called %d times without WithExtras; want 0", f.listInstalledHit)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Missing detection must still work -- that is what status and apply use.
	if missing := results[0].Missing(); len(missing) != 1 || missing[0] != "ripgrep" {
		t.Errorf("missing packages: got %v, want [ripgrep]", missing)
	}
	if extra := results[0].Extra(); len(extra) != 0 {
		t.Errorf("expected no extras without WithExtras, got %v", extra)
	}
	if results[0].ExtrasComputed() {
		t.Error("ExtrasComputed should be false without WithExtras")
	}
}

func TestComputeSync_WithExtrasReportsThem(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "git"}, {Name: "unrelated"}}}
	withFakeManager(t, f)

	results, err := ComputeSync(syncConfig(), "", nil, WithExtras(true))
	if err != nil {
		t.Fatal(err)
	}

	if f.listInstalledHit != 1 {
		t.Errorf("ListInstalled called %d times with WithExtras; want 1", f.listInstalledHit)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if extra := results[0].Extra(); len(extra) != 1 || extra[0] != "unrelated" {
		t.Errorf("extras: got %v, want [unrelated]", extra)
	}
	if !results[0].ExtrasComputed() {
		t.Error("ExtrasComputed should be true with WithExtras")
	}

	// Enabling extras must not disturb the missing/installed classification.
	if missing := results[0].Missing(); len(missing) != 1 || missing[0] != "ripgrep" {
		t.Errorf("missing packages: got %v, want [ripgrep]", missing)
	}
}
