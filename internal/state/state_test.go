package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDBOpenClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestFileOperations(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	entry := &FileEntry{
		SourcePath:  "/source/config.yaml",
		TargetPath:  "/target/config.yaml",
		SourceHash:  "abc123",
		AppliedHash: "abc123",
		Mode:        0644,
	}

	if err := db.SaveFile(entry); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	got, err := db.GetFile("/target/config.yaml")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetFile returned nil")
	}

	if got.SourcePath != entry.SourcePath {
		t.Errorf("SourcePath: got %q, want %q", got.SourcePath, entry.SourcePath)
	}
	if got.SourceHash != entry.SourceHash {
		t.Errorf("SourceHash: got %q, want %q", got.SourceHash, entry.SourceHash)
	}
	if got.Mode != entry.Mode {
		t.Errorf("Mode: got %o, want %o", got.Mode, entry.Mode)
	}

	entry.SourceHash = "def456"
	if err := db.SaveFile(entry); err != nil {
		t.Fatalf("SaveFile (update) failed: %v", err)
	}

	got, err = db.GetFile("/target/config.yaml")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if got.SourceHash != "def456" {
		t.Errorf("SourceHash after update: got %q, want %q", got.SourceHash, "def456")
	}

	files, err := db.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("ListFiles: got %d files, want 1", len(files))
	}

	if err := db.DeleteFile("/target/config.yaml"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	got, err = db.GetFile("/target/config.yaml")
	if err != nil {
		t.Fatalf("GetFile after delete failed: %v", err)
	}
	if got != nil {
		t.Error("file still exists after delete")
	}
}

func TestScriptOperations(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	has, err := db.HasScriptRun("/scripts/setup.sh")
	if err != nil {
		t.Fatalf("HasScriptRun failed: %v", err)
	}
	if has {
		t.Error("expected script not run yet")
	}

	if err := db.RecordScriptRun("/scripts/setup.sh", "hash123"); err != nil {
		t.Fatalf("RecordScriptRun failed: %v", err)
	}

	has, err = db.HasScriptRun("/scripts/setup.sh")
	if err != nil {
		t.Fatalf("HasScriptRun failed: %v", err)
	}
	if !has {
		t.Error("expected script to have run")
	}

	hasWithHash, err := db.HasScriptRunWithHash("/scripts/setup.sh", "hash123")
	if err != nil {
		t.Fatalf("HasScriptRunWithHash failed: %v", err)
	}
	if !hasWithHash {
		t.Error("expected script run with matching hash")
	}

	hasWithHash, err = db.HasScriptRunWithHash("/scripts/setup.sh", "differenthash")
	if err != nil {
		t.Fatalf("HasScriptRunWithHash failed: %v", err)
	}
	if hasWithHash {
		t.Error("expected no match with different hash")
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	if hash == "" {
		t.Error("hash is empty")
	}

	hash2, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}
	if hash != hash2 {
		t.Error("same file should produce same hash")
	}

	if err := os.WriteFile(testFile, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	hash3, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}
	if hash == hash3 {
		t.Error("different content should produce different hash")
	}
}

func TestHashBytes(t *testing.T) {
	h1 := HashBytes([]byte("hello"))
	h2 := HashBytes([]byte("hello"))
	h3 := HashBytes([]byte("world"))

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
}
