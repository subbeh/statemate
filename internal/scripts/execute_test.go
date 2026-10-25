package scripts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/state"
)

func TestExecutor_RunOnce(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	markerFile := filepath.Join(tmpDir, "marker")

	scriptPath := filepath.Join(tmpDir, "01-test#once#before.sh")
	scriptContent := "#!/bin/bash\ntouch " + markerFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	contentHash, _ := state.HashFile(scriptPath)
	script := &Script{
		Path:        scriptPath,
		Name:        "test",
		Frequency:   FreqOnce,
		Timing:      TimingBefore,
		Order:       1,
		ContentHash: contentHash,
	}

	executor := NewExecutor(db, nil, false).WithConfirmation(true, false)

	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}

	if result.Executed != 1 {
		t.Errorf("expected 1 executed, got %d", result.Executed)
	}

	if _, err := os.Stat(markerFile); err != nil {
		t.Error("marker file should exist after script execution")
	}

	_ = os.Remove(markerFile)

	result2, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	if result2.Skipped != 1 {
		t.Errorf("expected 1 skipped on second run, got %d", result2.Skipped)
	}

	if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
		t.Error("marker file should not exist after skipped run")
	}
}

func TestExecutor_RunOnchange(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	markerFile := filepath.Join(tmpDir, "marker")

	scriptPath := filepath.Join(tmpDir, "01-test#onchange#before.sh")
	scriptContent := "#!/bin/bash\ntouch " + markerFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	contentHash, _ := state.HashFile(scriptPath)
	script := &Script{
		Path:        scriptPath,
		Name:        "test",
		Frequency:   FreqOnchange,
		Timing:      TimingBefore,
		Order:       1,
		ContentHash: contentHash,
	}

	// #onchange is driven by pending changes to the script's own source.
	script.SourceDir = filepath.Join(tmpDir, "mysrc")

	// No pending changes -> does not run.
	executor := NewExecutor(db, nil, false).WithConfirmation(true, false)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute with no changes failed: %v", err)
	}
	if result.Executed != 0 {
		t.Errorf("expected no run without source changes, got %d executed", result.Executed)
	}
	if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
		t.Error("script must not run when its source has no pending changes")
	}

	// Its own source changed -> runs.
	executor = NewExecutor(db, nil, false).
		WithConfirmation(true, false).
		WithChangedSources(NewChangedSources("mysrc"))
	result, err = executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute with source changes failed: %v", err)
	}
	if result.Executed != 1 {
		t.Errorf("expected 1 executed when source changed, got %d", result.Executed)
	}
	if _, err := os.Stat(markerFile); err != nil {
		t.Error("script should have run when its source changed")
	}

	// A different source changing must not trigger it.
	_ = os.Remove(markerFile)
	executor = NewExecutor(db, nil, false).
		WithConfirmation(true, false).
		WithChangedSources(NewChangedSources("othersrc"))
	result, err = executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute with unrelated changes failed: %v", err)
	}
	if result.Executed != 0 {
		t.Errorf("an unrelated source must not trigger the script, got %d executed", result.Executed)
	}
}

// A script in the repo-root .matescripts has no owning source, so any change
// triggers it.
func TestExecutor_OnchangeRootScriptWatchesAllSources(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker")
	scriptPath := filepath.Join(tmpDir, "01-root#onchange#before.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\ntouch "+marker+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// SourceDir is empty for a root script.
	script := &Script{Path: scriptPath, Name: "root", Frequency: FreqOnchange, Timing: TimingBefore}

	executor := NewExecutor(db, nil, false).
		WithConfirmation(true, false).
		WithChangedSources(NewChangedSources("anything"))
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Executed != 1 {
		t.Errorf("root script should run when any source changed, got %d", result.Executed)
	}

	// With nothing changed it must not run.
	_ = os.Remove(marker)
	executor = NewExecutor(db, nil, false).WithConfirmation(true, false)
	result, err = executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Executed != 0 {
		t.Errorf("root script should not run with no changes, got %d", result.Executed)
	}
}

// Editing the script itself is no longer a trigger.
func TestExecutor_OnchangeIgnoresScriptContentChange(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker")
	scriptPath := filepath.Join(tmpDir, "01-x#onchange#before.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\ntouch "+marker+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	hash, _ := state.HashFile(scriptPath)
	script := &Script{
		Path:        scriptPath,
		Name:        "x",
		Frequency:   FreqOnchange,
		Timing:      TimingBefore,
		SourceDir:   filepath.Join(tmpDir, "mysrc"),
		ContentHash: hash,
	}

	// Rewrite the script so its content hash differs, with no source changes.
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\ntouch "+marker+"\necho edited\n"), 0755); err != nil {
		t.Fatal(err)
	}
	newHash, _ := state.HashFile(scriptPath)
	if newHash == hash {
		t.Fatal("expected the script hash to change")
	}
	script.ContentHash = newHash

	executor := NewExecutor(db, nil, false).WithConfirmation(true, false)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Executed != 0 {
		t.Errorf("editing the script must not trigger it, got %d executed", result.Executed)
	}
}

func TestExecutor_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	markerFile := filepath.Join(tmpDir, "marker")

	scriptPath := filepath.Join(tmpDir, "01-test#always#before.sh")
	scriptContent := "#!/bin/bash\ntouch " + markerFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	contentHash, _ := state.HashFile(scriptPath)
	script := &Script{
		Path:        scriptPath,
		Name:        "test",
		Frequency:   FreqAlways,
		Timing:      TimingBefore,
		Order:       1,
		ContentHash: contentHash,
	}

	executor := NewExecutor(db, nil, true)

	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	if result.Executed != 1 {
		t.Errorf("expected 1 would-execute, got %d", result.Executed)
	}

	if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
		t.Error("marker file should not exist after dry-run")
	}
}

func TestExecutor_ManualScript(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	scriptPath := filepath.Join(tmpDir, "manual-script.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	script := &Script{
		Path:      scriptPath,
		Name:      "manual-script",
		Frequency: FreqManual,
	}

	executor := NewExecutor(db, nil, false)

	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("manual script should be skipped in batch execute, got %d skipped", result.Skipped)
	}
}

// The changes that trigger an #onchange script are applied before an #after
// script runs, so a failed run must stay due without them.
func TestExecutor_OnchangeRetriesAfterFailure(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "01-sync#onchange#after.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	script := &Script{Path: scriptPath, Name: "sync", Frequency: FreqOnchange, Timing: TimingAfter, SourceDir: tmpDir}

	changed := NewExecutor(db, nil, false).WithConfirmation(true, false).WithChangedSources(NewChangedSources(tmpDir))
	if _, err := changed.Execute(Scripts{script}); err == nil {
		t.Fatal("expected the failing script to fail")
	}

	unchanged := NewExecutor(db, nil, false).WithConfirmation(true, false)
	if due, _, err := ShouldRun(script, db, ChangedSources{}); err != nil || !due {
		t.Fatalf("failed script should stay due without changes: due=%v err=%v", due, err)
	}

	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := unchanged.Execute(Scripts{script})
	if err != nil || result.Executed != 1 {
		t.Fatalf("expected the retry to run: executed=%d err=%v", result.Executed, err)
	}
	if due, _, _ := ShouldRun(script, db, ChangedSources{}); due {
		t.Error("script should not be due again after succeeding")
	}
}

// "# Onchange: self" runs when the script differs from its last recorded run,
// and only then: changes elsewhere in its source no longer trigger it.
func TestShouldRun_OnchangeSelf(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	script := &Script{
		Path:        filepath.Join(tmpDir, "01-x#onchange#before.sh"),
		Frequency:   FreqOnchange,
		SourceDir:   filepath.Join(tmpDir, "mysrc"),
		ContentHash: "v1",
		Onchange:    []string{OnchangeSelf},
	}

	if due, _, err := ShouldRun(script, db, ChangedSources{}); err != nil || !due {
		t.Fatalf("a script that never ran should be due, got due=%v err=%v", due, err)
	}
	if err := db.RecordScriptRun(script.Path, "v1"); err != nil {
		t.Fatal(err)
	}
	if due, _, _ := ShouldRun(script, db, NewChangedSources("mysrc")); due {
		t.Error("an unchanged script must not run on source changes")
	}
	script.ContentHash = "v2"
	if due, _, _ := ShouldRun(script, db, ChangedSources{}); !due {
		t.Error("an edited script should be due")
	}
}

// Patterns match the target paths of pending changes in the script's own source,
// or in every source for a repo-root script.
func TestShouldRun_OnchangePatterns(t *testing.T) {
	db, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	var changed ChangedSources
	changed.Add("/repo/arch", "/etc/keyd/default.conf")
	changed.Add("/repo/base", "/etc/systemd/system/a.service")

	tests := []struct {
		sourceDir, pattern string
		want               bool
	}{
		{"/repo/arch", "/etc/keyd/*.conf", true},
		{"/repo/arch", "*.service", false}, // only in another source
		{"/repo/base", "/etc/keyd/*.conf", false},
		{"", "*.service", true}, // a repo-root script sees every source
		{"", "*.timer", false},
	}
	for _, tc := range tests {
		script := &Script{Path: "/s", Frequency: FreqOnchange, SourceDir: tc.sourceDir, Onchange: []string{tc.pattern}}
		if due, _, err := ShouldRun(script, db, changed); err != nil || due != tc.want {
			t.Errorf("source %q pattern %q: due=%v err=%v, want %v", tc.sourceDir, tc.pattern, due, err, tc.want)
		}
	}
}

func TestReadOnchange(t *testing.T) {
	dir := t.TempDir()
	write := func(header string) string {
		p := filepath.Join(dir, "s.sh")
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+header+"\necho hi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	got, err := ReadOnchange(write("#  onchange:  self /etc/keyd/*.conf "), FreqOnchange)
	if err != nil || len(got) != 2 || got[0] != "self" || got[1] != "/etc/keyd/*.conf" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := ReadOnchange(write("# Description: none"), FreqOnchange); got != nil || err != nil {
		t.Errorf("no comment: got %q, %v", got, err)
	}
	for header, freq := range map[string]Frequency{
		"# Onchange:":      FreqOnchange,
		"# Onchange: [bad": FreqOnchange,
		"# Onchange: self": FreqDaily,
	} {
		if _, err := ReadOnchange(write(header), freq); err == nil {
			t.Errorf("%q (%s): expected an error", header, freq)
		}
	}
}
