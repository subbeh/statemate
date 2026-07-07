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

	executor := NewExecutor(db, nil, false, false)

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

	executor := NewExecutor(db, nil, false, false)

	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	if result.Executed != 1 {
		t.Errorf("expected 1 executed, got %d", result.Executed)
	}

	_ = os.Remove(markerFile)

	result2, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}
	if result2.Skipped != 1 {
		t.Errorf("expected skipped with same hash, got %d executed", result2.Executed)
	}

	newContent := "#!/bin/bash\ntouch " + markerFile + "\necho changed\n"
	if err := os.WriteFile(scriptPath, []byte(newContent), 0755); err != nil {
		t.Fatal(err)
	}
	newHash, _ := state.HashFile(scriptPath)
	script.ContentHash = newHash

	result3, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("third execute failed: %v", err)
	}
	if result3.Executed != 1 {
		t.Errorf("expected 1 executed after change, got %d", result3.Executed)
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
