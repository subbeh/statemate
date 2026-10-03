package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/state"
)

func TestMatch(t *testing.T) {
	home := "/home/u"
	tests := []struct {
		pattern, target string
		want            bool
	}{
		// No slash: a name at any depth, in any target root.
		{"*.service", "/etc/systemd/system/keyd.service", true},
		{"*.service", "/home/u/.config/systemd/user/a.service", true},
		{"*.service", "/etc/systemd/system/keyd.timer", false},
		{"profile.d", "/etc/profile.d/x.env", true},
		// Relative with a slash: anchored at home.
		{".config/tmux/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/*.conf", "/home/u/.config/tmux/sub/a.conf", false},
		{".config/tmux/*.conf", "/etc/.config/tmux/tmux.conf", false},
		{"~/.config/tmux/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		// ** spans segments, including none.
		{".config/tmux/**/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/**/*.conf", "/home/u/.config/tmux/a/b/c.conf", true},
		{"/etc/**", "/etc/keyd/default.conf", true},
		// Absolute.
		{"/etc/keyd/*.conf", "/etc/keyd/default.conf", true},
		{"/etc/keyd/*.conf", "/home/u/etc/keyd/default.conf", false},
		// A directory matches what is under it; a trailing slash only that.
		{".config/tmux", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/", "/home/u/.config/tmux/tmux.conf", true},
		{"tmux.conf/", "/home/u/.config/tmux/tmux.conf", false},
		{"~", "/home/u/.zshrc", true},
	}
	for _, tc := range tests {
		if got := match(tc.pattern, tc.target, home); got != tc.want {
			t.Errorf("match(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
		}
	}
}

func hook(match string, do ...config.HookStep) *config.Hook {
	return &config.Hook{Match: config.StringList{match}, Do: do}
}

func TestTrigger(t *testing.T) {
	disabled := false
	cfg := &config.Config{Hooks: map[string]*config.Hook{
		"b-units": hook("/etc/**/*.service", config.HookStep{Run: "true"}),
		"a-all":   hook("/etc", config.HookStep{Run: "true"}),
		"off":     {Match: config.StringList{"/etc"}, Enabled: &disabled},
		"arch":    {Match: config.StringList{"/etc"}, Do: []config.HookStep{{Run: "true"}}, Profile: "arch"},
	}}
	dirCfgs := map[string]*config.DirConfig{
		"/src/arch": {Hooks: map[string]*config.Hook{"own": hook("*.service", config.HookStep{Run: "true"})}},
	}

	set, err := Collect(cfg, []string{"/src/arch", "/src/other"}, func(d string) *config.DirConfig { return dirCfgs[d] }, nil)
	if err != nil {
		t.Fatal(err)
	}

	changes := []Change{
		{Path: "/etc/systemd/system/b.service", SourceDir: "/src/other"},
		{Path: "/etc/systemd/system/a.service", SourceDir: "/src/other"},
		{Path: "/etc/systemd/system/a.service", SourceDir: "/src/other"},
		{Path: "/etc/keyd.conf", SourceDir: "/src/arch"},
	}

	got := set.Trigger(changes, []string{"base"})
	var names []string
	for _, tr := range got {
		names = append(names, tr.Name)
	}
	// Alphabetical; disabled and profile-inactive hooks never trigger, and the
	// source hook ignores .service files from another source.
	if strings.Join(names, ",") != "a-all,b-units" {
		t.Fatalf("triggered %v", names)
	}
	if files := strings.Join(got[1].Files, ","); files != "/etc/systemd/system/a.service,/etc/systemd/system/b.service" {
		t.Errorf("b-units files = %s, want deduplicated and sorted", files)
	}

	got = set.Trigger([]Change{{Path: "/etc/x.service", SourceDir: "/src/arch"}}, []string{"arch", "base"})
	names = nil
	for _, tr := range got {
		names = append(names, tr.Name)
	}
	if strings.Join(names, ",") != "a-all,arch,arch/own,b-units" {
		t.Errorf("triggered %v", names)
	}
}

func TestCollectValidation(t *testing.T) {
	tests := map[string]*config.Hook{
		"no match":       {Do: []config.HookStep{{Run: "true"}}},
		"no steps":       {Match: config.StringList{"*"}},
		"both in a step": hook("*", config.HookStep{Run: "true", Script: "x"}),
		"bad glob":       hook("[", config.HookStep{Run: "true"}),
		"bad template":   hook("*", config.HookStep{Run: "{{ .Files"}),
		"unknown script": hook("*", config.HookStep{Script: "nope.sh"}),
	}
	for name, h := range tests {
		cfg := &config.Config{Hooks: map[string]*config.Hook{"h": h}}
		if _, err := Collect(cfg, nil, func(string) *config.DirConfig { return nil }, nil); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	cfg := &config.Config{Hooks: map[string]*config.Hook{"h": nil}}
	if _, err := Collect(cfg, nil, nil, nil); err == nil {
		t.Error("a hook with no settings should be rejected")
	}
}

func TestResolveScript(t *testing.T) {
	all := scripts.Scripts{
		{Name: "reload.sh", Path: "/repo/.matescripts/reload.sh"},
		{Name: "reload.sh", Path: "/src/a/.matescripts/reload.sh", SourceDir: "/src/a"},
		{Name: "dup.sh", Path: "/src/a/.matescripts/dup.sh", SourceDir: "/src/a"},
		{Name: "dup.sh", Path: "/src/b/.matescripts/dup.sh", SourceDir: "/src/b"},
		{Name: "only.sh", Path: "/src/b/.matescripts/10-only.sh#onchange", SourceDir: "/src/b"},
	}

	s, err := resolveScript("reload.sh", "/src/a", all)
	if err != nil || s.SourceDir != "/src/a" {
		t.Errorf("source hook should prefer its own source, got %v, %v", s, err)
	}
	s, err = resolveScript("reload.sh", "", all)
	if err != nil || s.SourceDir != "" {
		t.Errorf("global hook should prefer the repository root, got %v, %v", s, err)
	}
	s, err = resolveScript("dup.sh", "/src/a", all)
	if err != nil || s.SourceDir != "/src/a" {
		t.Errorf("own source wins over an ambiguous name, got %v, %v", s, err)
	}
	if _, err := resolveScript("dup.sh", "", all); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguity error, got %v", err)
	}
	if s, err := resolveScript("only.sh", "", all); err != nil || s.Name != "only.sh" {
		t.Errorf("unique name in another source should resolve, got %v, %v", s, err)
	}
}

func TestOwningSource(t *testing.T) {
	dirs := []string{"/src/a", "/src/ab", "/src/a/nested"}
	cases := map[string]string{
		"/src/a/x":        "/src/a",
		"/src/ab/x":       "/src/ab",
		"/src/a/nested/x": "/src/a/nested",
		"/src/a":          "/src/a",
		"/elsewhere/file": "",
	}
	for path, want := range cases {
		if got := OwningSource(path, dirs); got != want {
			t.Errorf("OwningSource(%q) = %q, want %q", path, got, want)
		}
	}
}

func newRunner(t *testing.T, opts Options) (*Runner, *scripts.Executor) {
	t.Helper()
	db, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	exec := scripts.NewExecutor(db, nil, opts.DryRun, false).WithConfirmation(opts.Force, opts.NoScripts)
	return NewRunner(exec, nil, opts), exec
}

func TestRunStepsEnvAndFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")

	scriptDir := filepath.Join(dir, ".matescripts")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptDir, "note.sh#onchange#after")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho \"script $STATEMATE_HOOK_NAME\" >> "+out+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	note := &scripts.Script{Name: "note.sh", Path: scriptPath, Frequency: scripts.FreqOnchange, Timing: scripts.TimingAfter}

	cfg := &config.Config{Hooks: map[string]*config.Hook{
		"a-fails": hook("*", config.HookStep{Run: "exit 3"}, config.HookStep{Run: "echo unreachable >> " + out}),
		"b-works": hook("*",
			config.HookStep{Run: `echo "{{ len .Files }} $STATEMATE_HOOK_FILES $(pwd)" >> ` + out},
			config.HookStep{Script: "note.sh"},
		),
	}}
	for _, h := range cfg.Hooks {
		h.SetDir(dir)
	}
	set, err := Collect(cfg, nil, nil, scripts.Scripts{note})
	if err != nil {
		t.Fatal(err)
	}

	runner, exec := newRunner(t, Options{Force: true})
	res, err := runner.Run(set.Trigger([]Change{{Path: "/etc/x"}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || res.Ran != 1 || res.Err() == nil {
		t.Fatalf("want one failure and one run, got %+v", res)
	}

	data, _ := os.ReadFile(out)
	realDir, _ := filepath.EvalSymlinks(dir)
	want := "1 /etc/x " + realDir + "\nscript b-works\n"
	if string(data) != want {
		t.Errorf("output = %q, want %q", data, want)
	}

	// The script's own #after trigger must not run it again in this invocation.
	res2, err := exec.Execute(scripts.Scripts{note})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Executed != 0 {
		t.Errorf("script ran again after its hook ran it")
	}
}

func TestRunNoScriptsAndNoTTY(t *testing.T) {
	cfg := &config.Config{Hooks: map[string]*config.Hook{"h": hook("*", config.HookStep{Run: "exit 1"})}}
	set, err := Collect(cfg, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	triggered := set.Trigger([]Change{{Path: "/x"}}, nil)

	runner, _ := newRunner(t, Options{NoScripts: true})
	res, err := runner.Run(triggered)
	if err != nil || res.Ran != 0 || len(res.Failed) != 0 {
		t.Errorf("--no-scripts should skip hooks silently, got %+v, %v", res, err)
	}

	orig := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = orig })
	runner, _ = newRunner(t, Options{})
	res, err = runner.Run(triggered)
	if err != nil || len(res.SkippedNoTTY) != 1 || len(res.Failed) != 0 {
		t.Errorf("without a terminal hooks should be skipped, got %+v, %v", res, err)
	}

	runner, _ = newRunner(t, Options{DryRun: true})
	res, err = runner.Run(triggered)
	if err != nil || res.Ran != 1 || len(res.Failed) != 0 {
		t.Errorf("dry run should report without running, got %+v, %v", res, err)
	}
}
