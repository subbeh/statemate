package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

func TestApplier_Apply(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app", ".config", "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	configContent := []byte("setting = true\n")
	srcFile := filepath.Join(sourceDir, "app", ".config", "app", "config.txt")
	if err := os.WriteFile(srcFile, configContent, 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	applier := NewApplier(db, nil, nil, false, false, 0)
	result, err := applier.Apply(tree)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("expected 1 applied file, got %d", result.Applied)
	}

	targetFile := filepath.Join(targetDir, ".config", "app", "config.txt")
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}

	if string(content) != string(configContent) {
		t.Errorf("content mismatch: got %q, want %q", content, configContent)
	}

	result2, err := applier.Apply(tree)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}

	if result2.Skipped != 1 {
		t.Errorf("expected 1 skipped file on second apply, got %d skipped, %d applied", result2.Skipped, result2.Applied)
	}
}

func TestApplier_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	srcFile := filepath.Join(sourceDir, "app", "test.txt")
	if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	applier := NewApplier(db, nil, nil, true, false, 0)
	result, err := applier.Apply(tree)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("expected 1 would-apply file, got %d", result.Applied)
	}

	targetFile := filepath.Join(targetDir, "test.txt")
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("target file should not exist after dry-run")
	}
}

func TestComputeChanges(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	srcFile := filepath.Join(sourceDir, "app", "new.txt")
	if err := os.WriteFile(srcFile, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	result, err := ComputeChanges(tree, db)
	if err != nil {
		t.Fatalf("ComputeChanges: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(result.Changes))
	}

	if result.Changes[0].Status != StatusNew {
		t.Errorf("expected StatusNew, got %v", result.Changes[0].Status)
	}
}

func TestComputeChanges_PermMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	content := []byte("script content")
	srcFile := filepath.Join(sourceDir, "app", "run.sh#perm:755")
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
		t.Fatal(err)
	}

	// Target exists with matching content but wrong permissions
	targetFile := filepath.Join(targetDir, "run.sh")
	if err := os.WriteFile(targetFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	result, err := ComputeChanges(tree, db)
	if err != nil {
		t.Fatalf("ComputeChanges: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	if result.Changes[0].Status != StatusModified {
		t.Errorf("expected StatusModified for perm mismatch, got %v", result.Changes[0].Status)
	}
}

func TestComputeChanges_PermMatch(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	content := []byte("script content")
	srcFile := filepath.Join(sourceDir, "app", "run.sh#perm:755")
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
		t.Fatal(err)
	}

	// Target with correct permissions — should be unchanged
	targetFile := filepath.Join(targetDir, "run.sh")
	if err := os.WriteFile(targetFile, content, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	result, err := ComputeChanges(tree, db)
	if err != nil {
		t.Fatalf("ComputeChanges: %v", err)
	}

	if len(result.Changes) != 0 {
		t.Errorf("expected 0 changes (perms match), got %d with status %v", len(result.Changes), result.Changes[0].Status)
	}
}

func TestApplier_PermFixOnApply(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	dbPath := filepath.Join(tmpDir, "state.db")

	_ = os.MkdirAll(filepath.Join(sourceDir, "app"), 0755)
	_ = os.MkdirAll(targetDir, 0755)

	content := []byte("#!/bin/sh\necho hi")
	srcFile := filepath.Join(sourceDir, "app", "run.sh#perm:755")
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
		t.Fatal(err)
	}

	// Target exists with same content but 644
	targetFile := filepath.Join(targetDir, "run.sh")
	if err := os.WriteFile(targetFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	applier := NewApplier(db, nil, nil, false, false, 0)
	result, err := applier.Apply(tree)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	if result.Applied != 1 {
		t.Errorf("expected 1 applied (perm fix), got %d applied, %d skipped", result.Applied, result.Skipped)
	}

	info, err := os.Stat(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}
}

// Applying a source that maps a directory outside the user's writable tree
// (e.g. targets: { etc: /etc }) must not fail with "permission denied" -- the
// directory loop needs the same sudo fallback applyFile already has.
func TestApplier_CreatesMappedDirectoriesWithAttrs(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")

	// A directory carrying a recursive perm attribute, as etc#perm-r:750 would.
	nested := filepath.Join(sourceDir, "app", "etc#perm-r:750", "restic")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "excludes"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	applier := NewApplier(db, nil, nil, false, false, 0)
	if _, err := applier.Apply(tree); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// The attribute must reach directories, not just files.
	dir := filepath.Join(targetDir, "etc", "restic")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected %s to be created: %v", dir, err)
	}
	if got := info.Mode().Perm(); got != 0750 {
		t.Errorf("directory mode: got %o, want 750", got)
	}
}

// An existing directory with no attributes must be left completely alone.
// Otherwise a mapped root like /etc would get chmodded on every apply.
func TestApplier_LeavesExistingUnattributedDirectoriesAlone(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")

	if err := os.MkdirAll(filepath.Join(sourceDir, "app", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "app", "sub", "f"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create the target subdirectory with a deliberately unusual mode.
	existing := filepath.Join(targetDir, "sub")
	if err := os.MkdirAll(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0705); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{filepath.Join(sourceDir, "app")})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	applier := NewApplier(db, nil, nil, false, false, 0)
	if _, err := applier.Apply(tree); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	info, err := os.Stat(existing)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0705 {
		t.Errorf("existing directory mode changed: got %o, want 705 (untouched)", got)
	}
}
