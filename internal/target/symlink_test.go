package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

// symlinkFixture is one #symlink source in a scanned tree. Change detection for
// these compares link text, so the destination may be anything -- a directory,
// or nothing at all.
type symlinkFixture struct {
	t          *testing.T
	db         *state.DB
	tree       *source.Tree
	sourcePath string
	targetPath string
}

func newSymlinkFixture(t *testing.T, dest string) *symlinkFixture {
	t.Helper()
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source", "app")
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "link#symlink")
	if err := os.Symlink(dest, sourcePath); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tree, err := source.NewScanner(targetDir, "").Scan([]string{sourceDir})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(tree.Files()) != 1 {
		t.Fatalf("expected 1 file in tree, got %d", len(tree.Files()))
	}
	return &symlinkFixture{t: t, db: db, tree: tree, sourcePath: sourcePath, targetPath: tree.Files()[0].TargetPath}
}

func (f *symlinkFixture) status() ChangeStatus {
	f.t.Helper()
	res, err := ComputeChanges(f.tree, f.db)
	if err != nil {
		f.t.Fatalf("ComputeChanges: %v", err)
	}
	if len(res.Changes) == 0 {
		return StatusUnchanged
	}
	return res.Changes[0].Status
}

func (f *symlinkFixture) apply() {
	f.t.Helper()
	if _, err := NewApplier(f.db, nil, nil, false, true, 0).Apply(f.tree); err != nil {
		f.t.Fatalf("apply: %v", err)
	}
}

func (f *symlinkFixture) relink(path, dest string) {
	f.t.Helper()
	if err := os.Remove(path); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(dest, path); err != nil {
		f.t.Fatal(err)
	}
}

// Hashing followed the link, so a link to a directory or to a path that does not
// exist made status and apply fail outright.
func TestSymlink_DirectoryAndDanglingDestinations(t *testing.T) {
	for name, dest := range map[string]string{
		"directory": t.TempDir(),
		"dangling":  "/nonexistent/statemate/test/path",
	} {
		t.Run(name, func(t *testing.T) {
			f := newSymlinkFixture(t, dest)
			if got := f.status(); got != StatusNew {
				t.Fatalf("before apply: got %v, want new", got)
			}
			f.apply()
			if got, err := os.Readlink(f.targetPath); err != nil || got != dest {
				t.Fatalf("target link = %q (%v), want %q", got, err, dest)
			}
			if got := f.status(); got != StatusUnchanged {
				t.Errorf("after apply: got %v, want unchanged", got)
			}

			// The recorded hashes describe the link itself, on both sides.
			want, err := state.HashLink(f.sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			rec, err := f.db.GetFile(f.targetPath)
			if err != nil || rec == nil {
				t.Fatalf("no state recorded: %v", err)
			}
			if rec.SourceHash != want || rec.AppliedHash != want {
				t.Errorf("recorded hashes %q/%q, want link hash %q", rec.SourceHash, rec.AppliedHash, want)
			}
		})
	}
}

// An unrecorded target that already is the right link only needs its state
// recorded; one saying something else, or a regular file, is in the way.
func TestSymlink_UnrecordedTarget(t *testing.T) {
	dest := t.TempDir()

	f := newSymlinkFixture(t, dest)
	if err := os.Symlink(dest, f.targetPath); err != nil {
		t.Fatal(err)
	}
	res, err := ComputeChanges(f.tree, f.db)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("matching link: expected no pending changes, got %v", res.Changes[0].Status)
	}
	f.apply()
	if rec, _ := f.db.GetFile(f.targetPath); rec == nil {
		t.Error("matching link: apply did not record state")
	}

	f = newSymlinkFixture(t, dest)
	if err := os.Symlink("/somewhere/else", f.targetPath); err != nil {
		t.Fatal(err)
	}
	if got := f.status(); got != StatusConflict {
		t.Errorf("different link: got %v, want conflict", got)
	}

	f = newSymlinkFixture(t, dest)
	if err := os.WriteFile(f.targetPath, []byte(dest), 0644); err != nil {
		t.Fatal(err)
	}
	if got := f.status(); got != StatusConflict {
		t.Errorf("regular file: got %v, want conflict", got)
	}
}

func TestSymlink_ChangesAfterApply(t *testing.T) {
	f := newSymlinkFixture(t, "/opt/old")
	f.apply()

	// The source now points elsewhere and the target is untouched: deploy it.
	f.relink(f.sourcePath, "/opt/new")
	if got := f.status(); got != StatusModified {
		t.Errorf("source relinked: got %v, want modified", got)
	}
	f.apply()
	if got, _ := os.Readlink(f.targetPath); got != "/opt/new" {
		t.Errorf("target link = %q, want /opt/new", got)
	}

	// Someone repointed the target: that is drift, not ours to overwrite.
	f.relink(f.targetPath, "/opt/theirs")
	if got := f.status(); got != StatusConflict {
		t.Errorf("target relinked: got %v, want conflict", got)
	}

	// Replaced by a regular file: likewise.
	if err := os.Remove(f.targetPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.targetPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := f.status(); got != StatusConflict {
		t.Errorf("target replaced by file: got %v, want conflict", got)
	}

	// Deleted: redeploy.
	if err := os.Remove(f.targetPath); err != nil {
		t.Fatal(err)
	}
	if got := f.status(); got != StatusModified {
		t.Errorf("target removed: got %v, want modified", got)
	}
}

// State recorded before link-text hashing holds content hashes. A link that is
// still correct must not turn into a conflict because of that.
func TestSymlink_LegacyStateWithCorrectLink(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	f := newSymlinkFixture(t, dest)
	if err := os.Symlink(dest, f.targetPath); err != nil {
		t.Fatal(err)
	}
	legacy := state.HashBytes([]byte("content"))
	if err := f.db.SaveFile(&state.FileEntry{
		SourcePath: f.sourcePath, TargetPath: f.targetPath,
		SourceHash: legacy, AppliedHash: legacy, Mode: 0777,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := ComputeChanges(f.tree, f.db)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("expected no pending changes, got %v", res.Changes[0].Status)
	}
	f.apply()
	want, _ := state.HashLink(f.sourcePath)
	if rec, _ := f.db.GetFile(f.targetPath); rec == nil || rec.AppliedHash != want {
		t.Errorf("apply should have re-recorded the link hash, got %+v", rec)
	}
}
