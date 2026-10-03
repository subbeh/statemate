package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// hookRepo is a throwaway repository for running mate apply end to end. Every
// hook and script in it touches a file named after itself in marks, so a test
// can tell what ran.
type hookRepo struct {
	dir, home, marks string
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// newHookRepo builds a repository with two sources. app's one file triggers a
// repo hook whose script: step lives in the repository root, a repo hook whose
// script: step lives in the other source, and a hook from app's own .mate.yaml.
// A repo-root #always#after script stands in for the lifecycle scripts a scoped
// run must not run.
//
// HOME, the XDG directories and STATEMATE_DIR all point into the test's temp
// dir, so nothing touches the real state database or config.
func newHookRepo(t *testing.T, mateYAML string) *hookRepo {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	r := &hookRepo{
		dir:   filepath.Join(root, "repo"),
		home:  filepath.Join(root, "home"),
		marks: filepath.Join(root, "marks"),
	}
	for _, d := range []string{r.home, r.marks} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", r.home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("STATEMATE_DIR", r.dir)
	t.Setenv("MARKS", r.marks)

	touch := func(name string) string { return "#!/bin/sh\ntouch \"$MARKS/" + name + "\"\n" }

	writeFile(t, filepath.Join(r.dir, "mate.yaml"), mateYAML, 0644)
	writeFile(t, filepath.Join(r.dir, ".matescripts", "hello.sh"), touch("hello.sh"), 0755)
	writeFile(t, filepath.Join(r.dir, ".matescripts", "lifecycle.sh#always#after"), touch("lifecycle"), 0755)
	writeFile(t, filepath.Join(r.dir, "app", ".config", "app", "app.conf"), "x\n", 0644)
	writeFile(t, filepath.Join(r.dir, "app", ".mate.yaml"), `hooks:
  own:
    match: "*.conf"
    do:
      - run: touch "$MARKS/app-own"
`, 0644)
	writeFile(t, filepath.Join(r.dir, "other", ".config", "other", "other.txt"), "y\n", 0644)
	writeFile(t, filepath.Join(r.dir, "other", ".matescripts", "elsewhere.sh"), touch("elsewhere.sh"), 0755)
	return r
}

// repoHooks is a mate.yaml whose hooks call scripts outside the app source.
const repoHooks = `sources: [app, other]
hooks:
  root-script:
    match: "*.conf"
    do:
      - script: hello.sh
  other-script:
    match: "*.conf"
    do:
      - script: elsewhere.sh
`

// apply runs mate apply with --force, so hooks run without a terminal.
func (r *hookRepo) apply(t *testing.T, args []string, sourceFlag string) error {
	t.Helper()
	cmd := &cobra.Command{Use: "apply", RunE: runApply}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("profile", "", "")
	addScopeFlag(cmd)
	if sourceFlag != "" {
		if err := cmd.Flags().Set(scopeFlagName, sourceFlag); err != nil {
			t.Fatal(err)
		}
	}

	origForce := force
	force = true
	defer func() { force = origForce }()

	return runApply(cmd, args)
}

func (r *hookRepo) ran(name string) bool {
	_, err := os.Stat(filepath.Join(r.marks, name))
	return err == nil
}

// A scoped apply used to drop every source and script before collecting hooks,
// so source hooks were never loaded and any script: step failed as "not found"
// after the files had already been written.
func TestApplyScoped_RunsHooksForWrittenFiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		source string
		// lifecycle is whether the repo-root #after script should run.
		lifecycle bool
	}{
		{name: "no scope", lifecycle: true},
		{name: "path scope", args: []string{"app.conf"}},
		{name: "source scope", source: "app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newHookRepo(t, repoHooks)

			if err := r.apply(t, tc.args, tc.source); err != nil {
				t.Fatalf("apply failed: %v", err)
			}

			if _, err := os.Stat(filepath.Join(r.home, ".config", "app", "app.conf")); err != nil {
				t.Errorf("app.conf was not written: %v", err)
			}
			for _, mark := range []string{"hello.sh", "elsewhere.sh", "app-own"} {
				if !r.ran(mark) {
					t.Errorf("hook step %s did not run", mark)
				}
			}
			// Scoping still keeps lifecycle scripts out of the run.
			if got := r.ran("lifecycle"); got != tc.lifecycle {
				t.Errorf("repo-root #after script ran = %v, want %v", got, tc.lifecycle)
			}
		})
	}
}

// A broken hook used to be noticed only after the files were written and the
// packages prompted for, so the apply failed with its work half done.
func TestApply_BrokenHookFailsBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mateYAML string
		appYAML  string
	}{
		{
			name: "unknown script",
			mateYAML: `sources: [app]
hooks:
  missing:
    match: "*.conf"
    do:
      - script: nope.sh
`,
		},
		{
			name: "run template does not parse",
			mateYAML: `sources: [app]
hooks:
  bad:
    match: "*.conf"
    do:
      - run: echo {{ .Files
`,
		},
		{
			name:     "source hook without match",
			mateYAML: "sources: [app]\n",
			appYAML: `hooks:
  bad:
    do:
      - run: "true"
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newHookRepo(t, tc.mateYAML)
			if tc.appYAML != "" {
				writeFile(t, filepath.Join(r.dir, "app", ".mate.yaml"), tc.appYAML, 0644)
			}

			err := r.apply(t, nil, "")
			if err == nil || !strings.Contains(err.Error(), "invalid hooks") {
				t.Fatalf("want an invalid hooks error, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(r.home, ".config", "app", "app.conf")); err == nil {
				t.Error("app.conf was written before the broken hook was reported")
			}
			if r.ran("lifecycle") {
				t.Error("#after script ran despite the broken hook")
			}
		})
	}
}
