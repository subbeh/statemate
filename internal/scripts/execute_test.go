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

	executor := NewExecutor(db, nil, false, false).WithConfirmation(true, false)

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
	executor := NewExecutor(db, nil, false, false).WithConfirmation(true, false)
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
	executor = NewExecutor(db, nil, false, false).
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
	executor = NewExecutor(db, nil, false, false).
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

	executor := NewExecutor(db, nil, false, false).
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
	executor = NewExecutor(db, nil, false, false).WithConfirmation(true, false)
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

	executor := NewExecutor(db, nil, false, false).WithConfirmation(true, false)
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

	executor := NewExecutor(db, nil, true, false)

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

	executor := NewExecutor(db, nil, false, false)

	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("manual script should be skipped in batch execute, got %d skipped", result.Skipped)
	}
}
