package packages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/template"
)

// fakeManager records whether the expensive full-inventory call was made.
//
// installed holds names as `brew leaves` reports them, which is fully qualified
// for a tap formula. queryReportsBare models the asymmetry that caused the tap
// bug: `brew list --formula` (without --full-name) prints the bare name, so a
// package declared by its qualified name never matched.
type fakeManager struct {
	installed        []Package
	listInstalledHit int
	queryReportsBare bool

	// descriptions is what Describe reports; names in descUnknown are reported as
	// not recognised by the manager at all.
	descriptions map[string]string
	descUnknown  map[string]bool
}

func (f *fakeManager) Name() string      { return "brew" }
func (f *fakeManager) IsAvailable() bool { return true }

func (f *fakeManager) ListInstalled() ([]Package, error) {
	f.listInstalledHit++
	return f.installed, nil
}

func (f *fakeManager) QueryInstalled(pkgs []string) ([]Package, error) {
	// Mirror the real BrewManager, which matches a declared name against both the
	// qualified and unqualified spellings.
	names := make([]string, 0, len(f.installed))
	for _, inst := range f.installed {
		name := inst.Name
		if f.queryReportsBare {
			name = unqualifiedName(name)
		}
		names = append(names, name)
	}
	idx := newBrewIndex(names)

	var out []Package
	for _, want := range pkgs {
		if !idx.has(want) {
			continue
		}
		out = append(out, Package{Name: want})
	}
	return out, nil
}

func (f *fakeManager) Describe([]string) (Descriptions, error) {
	return Descriptions{ByName: f.descriptions, Unknown: f.descUnknown}, nil
}
func (f *fakeManager) Install([]string) error   { return nil }
func (f *fakeManager) Uninstall([]string) error { return nil }

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

// A package declared by its fully-qualified tap name must be recognised as
// installed. brew reports such a formula bare from `brew list --formula`, so
// comparing the exact string reported it missing forever: every apply offered to
// install it, `brew install` said it was already there, and nothing converged.
func TestComputeSync_TapQualifiedPackageIsInstalled(t *testing.T) {
	// What brew leaves prints: qualified for tap formulae, bare for casks. The
	// query path reports the bare name, as `brew list --formula` does -- this
	// asymmetry is the bug.
	f := &fakeManager{
		installed: []Package{
			{Name: "jamf/internal-tap/hermes"},
			{Name: "slack"},
		},
		queryReportsBare: true,
	}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{
		"jamf/internal-tap/hermes",
		"slack",
	}}}

	results, err := ComputeSync(cfg, "", nil, WithExtras(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if missing := results[0].Missing(); len(missing) != 0 {
		t.Errorf("nothing should be missing, got %v", missing)
	}
	// The same mismatch would also list the declared package as an extra.
	if extra := results[0].Extra(); len(extra) != 0 {
		t.Errorf("nothing should be extra, got %v", extra)
	}
}

// The inverse spelling: declared bare, installed from a tap.
func TestComputeSync_BareNameMatchesTapInstall(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "jamf/internal-tap/hermes"}}}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{"hermes"}}}

	results, err := ComputeSync(cfg, "", nil, WithExtras(true))
	if err != nil {
		t.Fatal(err)
	}

	if missing := results[0].Missing(); len(missing) != 0 {
		t.Errorf("nothing should be missing, got %v", missing)
	}
	if extra := results[0].Extra(); len(extra) != 0 {
		t.Errorf("nothing should be extra, got %v", extra)
	}
}

// Guard the fix against over-matching: a genuinely absent package must still be
// reported missing, and a genuinely undeclared one still reported extra.
func TestComputeSync_UnrelatedPackagesStillClassified(t *testing.T) {
	f := &fakeManager{installed: []Package{
		{Name: "jamf/internal-tap/hermes"},
		{Name: "some/tap/undeclared"},
	}}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{
		"jamf/internal-tap/hermes",
		"jamf/internal-tap/absent",
	}}}

	results, err := ComputeSync(cfg, "", nil, WithExtras(true))
	if err != nil {
		t.Fatal(err)
	}

	missing := results[0].Missing()
	if len(missing) != 1 || missing[0] != "jamf/internal-tap/absent" {
		t.Errorf("missing: got %v, want [jamf/internal-tap/absent]", missing)
	}
	extra := results[0].Extra()
	if len(extra) != 1 || extra[0] != "some/tap/undeclared" {
		t.Errorf("extra: got %v, want [some/tap/undeclared]", extra)
	}
}

// A name the manager cannot resolve has to be distinguishable from one it resolved
// but has no description for. The first is a config error worth surfacing (a typo,
// or a Linux-only package declared for every platform); the second is normal.
func TestComputeSync_VerboseMarksUnresolvableNames(t *testing.T) {
	f := &fakeManager{
		installed:    []Package{{Name: "git"}, {Name: "ripgrep"}},
		descriptions: map[string]string{"git": "Distributed revision control system", "ripgrep": ""},
		descUnknown:  map[string]bool{"man": true},
	}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{"git", "ripgrep", "man"}}}
	results, err := ComputeSync(cfg, "", nil, WithVerbose(true))
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]PackageStatus)
	for _, s := range results[0].Statuses {
		got[s.Name] = s
	}

	for _, tc := range []struct {
		name        string
		wantDesc    string
		wantUnknown bool
		why         string
	}{
		{"git", "Distributed revision control system", false, "resolved with a description"},
		{"ripgrep", "", false, "resolved, but the manager has no description"},
		{"man", "", true, "no such package"},
	} {
		s, ok := got[tc.name]
		if !ok {
			t.Errorf("%s missing from statuses", tc.name)
			continue
		}
		if s.Description != tc.wantDesc {
			t.Errorf("%s description = %q, want %q (%s)", tc.name, s.Description, tc.wantDesc, tc.why)
		}
		if s.DescriptionUnknown != tc.wantUnknown {
			t.Errorf("%s DescriptionUnknown = %v, want %v (%s)", tc.name, s.DescriptionUnknown, tc.wantUnknown, tc.why)
		}
	}
}

// Without --verbose no descriptions are looked up, so nothing may be flagged as
// unresolvable -- otherwise every package would render as <unknown>.
func TestComputeSync_NonVerboseLeavesDescriptionsUnflagged(t *testing.T) {
	f := &fakeManager{
		installed:   []Package{{Name: "git"}},
		descUnknown: map[string]bool{"man": true},
	}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{"git", "man"}}}
	results, err := ComputeSync(cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range results[0].Statuses {
		if s.DescriptionUnknown {
			t.Errorf("%s flagged unknown without WithVerbose", s.Name)
		}
	}
}

// A profile inherits the packages of every profile it extends, as it already
// inherits their sources and variables. Each package is attributed to the
// profile that declared it, so `mate packages status` shows where it came from.
func TestComputeSync_InheritsPackagesAlongExtendsChain(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "git"}}}
	withFakeManager(t, f)

	cfg := &config.Config{Profiles: map[string]*config.Profile{
		"base":  {Packages: &config.PackageList{Brew: []string{"git"}}},
		"mac":   {Extends: "base", Packages: &config.PackageList{Brew: []string{"ripgrep"}}},
		"work":  {Extends: "mac", Packages: &config.PackageList{Brew: []string{"kubectl"}}},
		"other": {Packages: &config.PackageList{Brew: []string{"unrelated"}}},
	}}

	results, err := ComputeSync(cfg, "work", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := make(map[string][]string)
	for _, s := range results[0].Statuses {
		got[s.Name] = s.Sources
	}
	for name, source := range map[string]string{
		"git":     "profile:base",
		"ripgrep": "profile:mac",
		"kubectl": "profile:work",
	} {
		if len(got[name]) != 1 || got[name][0] != source {
			t.Errorf("%s sources = %v, want [%s]", name, got[name], source)
		}
	}
	if _, ok := got["unrelated"]; ok {
		t.Error("packages of a profile outside the chain must not be included")
	}
}

// writeDirConfig creates a source directory holding the given .mate.yaml.
func writeDirConfig(t *testing.T, content string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mate.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A source's .mate.yaml is a template wherever else it is read, so its packages
// must be rendered the same way. Read raw, a range loop yielded no packages and
// an interpolated name was taken literally as "{{ .Vars.editor }}".
func TestComputeSync_RendersTemplatedDirConfig(t *testing.T) {
	withFakeManager(t, &fakeManager{})

	source := writeDirConfig(t, `packages:
  brew:
    - "{{ .Vars.editor }}"
{{- range .Vars.extras }}
    - {{ . }}
{{- end }}
`)

	cfg := &config.Config{Variables: map[string]any{
		"editor": "neovim",
		"extras": []any{"ripgrep", "fd"},
	}}
	tmplCtx, err := template.NewContext(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	render := func(data []byte) ([]byte, error) { return template.Render(data, tmplCtx) }

	results, err := ComputeSync(cfg, "", []string{source}, WithDirConfigRenderer(render))
	if err != nil {
		t.Fatal(err)
	}

	got := results[0].Missing()
	if strings.Join(got, ",") != "fd,neovim,ripgrep" {
		t.Errorf("missing = %v, want [fd neovim ripgrep]", got)
	}
}

// A .mate.yaml that cannot be loaded is an error, not a source with no packages.
// Ignoring it made every package of that source vanish without a word.
func TestComputeSync_ReportsBrokenDirConfig(t *testing.T) {
	withFakeManager(t, &fakeManager{})

	source := writeDirConfig(t, "packages:\n  brew: [git\n")

	_, err := ComputeSync(syncConfig(), "", []string{source})
	if err == nil {
		t.Fatal("expected an error for an unparseable .mate.yaml")
	}
	if !strings.Contains(err.Error(), "tools") {
		t.Errorf("error should name the source, got: %v", err)
	}
}

// Homebrew's versioned formulae (node@20, python@3.12, openssl@3) are distinct
// packages whose names contain '@'. Splitting the name there checked for and
// installed plain node instead, then reported a permanent version mismatch.
func TestComputeSync_VersionedFormulaIsItsOwnPackage(t *testing.T) {
	f := &fakeManager{installed: []Package{{Name: "node"}}}
	withFakeManager(t, f)

	cfg := &config.Config{Packages: &config.PackageList{Brew: []string{"node@20"}}}
	results, err := ComputeSync(cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if missing := results[0].Missing(); len(missing) != 1 || missing[0] != "node@20" {
		t.Errorf("missing packages: got %v, want [node@20]", missing)
	}
}
