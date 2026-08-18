package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
)

// These tests guard the hand-written guides under docs/ against the code drifting
// away from them. They caught nothing when written -- they exist because #import
// shipped with the generated docs never noticing, and the next attribute would
// have done the same.
//
// They check only that a name is *mentioned* somewhere in the guides. That is a
// low bar deliberately: a stricter test would fail on wording changes and get
// disabled. Mentioning a feature is not documenting it well, but never mentioning
// it is certainly documenting it badly.

// docsDir resolves docs/ relative to this package, so the test works regardless of
// the working directory.
func docsDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "docs"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("docs directory not found at %s: %v", dir, err)
	}
	return dir
}

// readGuides concatenates the hand-written guides. docs/commands/ is excluded: it
// is generated from the same help text a missing feature would also be absent
// from, so counting it would let a feature "document" itself.
func readGuides(t *testing.T) string {
	t.Helper()

	dir := docsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(content)
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		t.Fatal("no guides found in docs/")
	}
	return sb.String()
}

func TestFileAttributesAreDocumented(t *testing.T) {
	guides := readGuides(t)

	for _, attr := range source.AttrNames {
		if !strings.Contains(guides, "#"+attr) {
			t.Errorf("file attribute #%s is not mentioned in docs/ -- add it to docs/attributes.md", attr)
		}
	}
}

func TestTemplateFunctionsAreDocumented(t *testing.T) {
	guides := readGuides(t)

	for _, fn := range template.MateFuncNames() {
		if !strings.Contains(guides, fn) {
			t.Errorf("template function %q is not mentioned in docs/ -- add it to docs/templates.md", fn)
		}
	}
}

func TestScriptFrequenciesAreDocumented(t *testing.T) {
	guides := readGuides(t)

	// Every frequency the parser accepts, by the name it is written as in a
	// filename. "manual" is the absence of an attribute rather than a spelling, so
	// it is checked as a word.
	freqs := []scripts.Frequency{
		scripts.FreqOnce, scripts.FreqOnchange, scripts.FreqAlways,
		scripts.FreqDaily, scripts.FreqWeekly, scripts.FreqMonthly,
		scripts.FreqManual,
	}

	for _, f := range freqs {
		name := f.String()
		if name == "unknown" {
			t.Errorf("frequency %d has no string form", f)
			continue
		}
		if !strings.Contains(guides, name) {
			t.Errorf("script frequency %q is not mentioned in docs/ -- add it to docs/scripts.md", name)
		}
	}
}

func TestScriptEnvVarsAreDocumented(t *testing.T) {
	guides := readGuides(t)

	// Set for every script run by internal/scripts.runScript.
	vars := []string{
		"STATEMATE_SCRIPT",
		"STATEMATE_SCRIPT_NAME",
		"STATEMATE_SCRIPT_FREQUENCY",
		"STATEMATE_SCRIPT_TIMING",
		"STATEMATE_SOURCE_DIR",
	}

	for _, v := range vars {
		if !strings.Contains(guides, v) {
			t.Errorf("script environment variable %s is not mentioned in docs/ -- add it to docs/scripts.md", v)
		}
	}
}

func TestEnvVarsAreDocumented(t *testing.T) {
	guides := readGuides(t)

	for _, v := range []string{"STATEMATE_DIR", "STATEMATE_PROFILE"} {
		if !strings.Contains(guides, v) {
			t.Errorf("environment variable %s is not mentioned in docs/ -- add it to docs/configuration.md", v)
		}
	}
}

// configKeys reads the yaml tags off a struct, which is exactly the set of keys a
// user can write. Deriving them by reflection means adding a field to the struct
// is enough to fail this test.
func configKeys(t *testing.T, v any) []string {
	t.Helper()

	typ := reflect.TypeOf(v)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var keys []string
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		keys = append(keys, strings.Split(tag, ",")[0])
	}
	return keys
}

func TestConfigKeysAreDocumented(t *testing.T) {
	guides := readGuides(t)

	cases := []struct {
		name string
		val  any
		file string
	}{
		{"mate.yaml", config.Config{}, "docs/configuration.md"},
		{"profile", config.Profile{}, "docs/configuration.md"},
		{"detection", config.Detection{}, "docs/configuration.md"},
		{"age", config.AgeConfig{}, "docs/configuration.md"},
		{"packages", config.PackageList{}, "docs/packages.md"},
		{".mate.yaml", config.DirConfig{}, "docs/configuration.md"},
		{"generate", config.GenerateConfig{}, "docs/configuration.md"},
		{"scripts", config.DirScripts{}, "docs/scripts.md"},
	}

	for _, c := range cases {
		for _, key := range configKeys(t, c.val) {
			if !strings.Contains(guides, key) {
				t.Errorf("%s key %q is not mentioned in docs/ -- add it to %s", c.name, key, c.file)
			}
		}
	}
}

// Every guide the index links to must exist, and every guide must be linked from
// the index -- an unlinked guide is one nobody finds.
func TestGuidesAreLinkedFromIndex(t *testing.T) {
	dir := docsDir(t)

	index, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		if !strings.Contains(string(index), "("+e.Name()+")") {
			t.Errorf("docs/%s is not linked from docs/README.md", e.Name())
		}
	}
}

// A broken relative link renders as a dead link on GitHub, which is where these
// docs are read. Checks docs/*.md and the top-level README.md, following links
// into docs/commands/ as well.
func TestDocsLinksResolve(t *testing.T) {
	dir := docsDir(t)
	repoRoot := filepath.Dir(dir)

	files := []string{filepath.Join(repoRoot, "README.md")}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	linkPattern := regexp.MustCompile(`\]\(([^)]+)\)`)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range linkPattern.FindAllStringSubmatch(string(content), -1) {
			link := m[1]

			// External links and same-page anchors are out of scope.
			if strings.HasPrefix(link, "http") || strings.HasPrefix(link, "#") {
				continue
			}

			// Strip any anchor; the file is what must exist.
			path, _, _ := strings.Cut(link, "#")
			if path == "" {
				continue
			}

			target := filepath.Join(filepath.Dir(file), path)
			if _, err := os.Stat(target); err != nil {
				rel, _ := filepath.Rel(repoRoot, file)
				t.Errorf("%s: link %q points at a missing file", rel, link)
			}
		}
	}
}

// Every command needs a Short description: it is what `mate help` lists and what
// the generated reference uses as a page title.
func TestEveryCommandHasHelpText(t *testing.T) {
	var walk func(cmd *cobra.Command, path string)
	walk = func(cmd *cobra.Command, path string) {
		for _, sub := range cmd.Commands() {
			// Both are generated by cobra itself.
			if sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			name := strings.TrimSpace(path + " " + sub.Name())
			if sub.Short == "" {
				t.Errorf("command %q has no Short description", name)
			}
			walk(sub, name)
		}
	}
	walk(RootCmd(), "")
}
