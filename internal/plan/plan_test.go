package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/target"
)

// newRepo writes a repository with the given files, relative to it, and points
// HOME, the XDG directories and STATEMATE_DIR into the test's temp dir. It
// returns the home directory, where targets land.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	repo, home := filepath.Join(root, "repo"), filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("STATEMATE_DIR", repo)
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func newPlan(t *testing.T) *Plan {
	t.Helper()
	env, err := NewEnv("", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := env.Plan()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func apply(t *testing.T, p *Plan) {
	t.Helper()
	db, err := p.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.NewApplier(db, p.env.TmplCtx, p.env.Enc, false, true, false).Apply(p.Tree); err != nil {
		t.Fatal(err)
	}
}

func changes(t *testing.T, p *Plan) map[string]target.ChangeStatus {
	t.Helper()
	res, err := p.Changes(false)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]target.ChangeStatus)
	for _, c := range res.Changes {
		got[filepath.Base(c.Entry.TargetPath)] = c.Status
	}
	return got
}

func TestChangesUntilApplied(t *testing.T) {
	newRepo(t, map[string]string{
		"mate.yaml":          "sources: [app]\n",
		"app/plain.conf":     "x\n",
		"app/host.conf#tmpl": "host={{ .Hostname }}\n",
	})

	p := newPlan(t)
	if got := changes(t, p); got["plain.conf"] != target.StatusNew || got["host.conf"] != target.StatusNew {
		t.Fatalf("before apply: got %v, want both new", got)
	}
	apply(t, p)

	// A #tmpl target holds the rendered text, so this only holds when the
	// template context is used for the comparison.
	if got := changes(t, newPlan(t)); len(got) != 0 {
		t.Errorf("after apply: got %v, want no changes", got)
	}
}

func TestOrphans(t *testing.T) {
	home := newRepo(t, map[string]string{
		"mate.yaml":     "sources: [app]\n",
		"app/keep.conf": "x\n",
		"app/gone.conf": "y\n",
	})
	p := newPlan(t)
	apply(t, p)

	if err := os.Remove(filepath.Join(os.Getenv("STATEMATE_DIR"), "app", "gone.conf")); err != nil {
		t.Fatal(err)
	}
	orphans, err := newPlan(t).Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "gone.conf"); len(orphans) != 1 || orphans[0] != want {
		t.Errorf("got %v, want [%s]", orphans, want)
	}
}

func TestConflicts(t *testing.T) {
	newRepo(t, map[string]string{
		"mate.yaml":   "sources: [a, b]\n",
		"a/same.conf": "a\n",
		"b/same.conf": "b\n",
	})
	if !newPlan(t).Tree.HasConflicts() {
		t.Error("two sources deploying one target: want a conflict")
	}
}

func TestIdentityThatFailsToLoad(t *testing.T) {
	files := map[string]string{
		"mate.yaml":      "sources: [app]\nage:\n  identity: ~/missing-key.txt\n",
		"app/plain.conf": "x\n",
	}
	newRepo(t, files)
	if _, err := newPlan(t).Changes(false); err != nil {
		t.Fatalf("no #enc entry needs the identity: %v", err)
	}

	files["app/secret.conf#enc"] = "ciphertext\n"
	newRepo(t, files)
	_, err := newPlan(t).Changes(false)
	if err == nil || !strings.Contains(err.Error(), "secret.conf#enc is #enc, but the age identity failed to load") ||
		!strings.Contains(err.Error(), "missing-key.txt") {
		t.Errorf("got %v, want the identity's load error naming the #enc file", err)
	}
}
