package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/source"
)

// mate diff shows a #symlink change as link text. Diffing the destinations
// failed for a link to a directory or to a path that does not exist.
func TestSymlinkDiff(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "link#symlink")
	dst := filepath.Join(dir, "link")
	if err := os.Symlink(dir, src); err != nil {
		t.Fatal(err)
	}
	entry := &source.Entry{SourcePath: src, TargetPath: dst, Attrs: source.Attrs{Symlink: true}}

	if got, want := symlinkDiff(entry), "+ -> "+dir; got != want {
		t.Errorf("missing target: got %q, want %q", got, want)
	}

	if err := os.Symlink("/nonexistent", dst); err != nil {
		t.Fatal(err)
	}
	if got, want := symlinkDiff(entry), "- -> /nonexistent\n+ -> "+dir; got != want {
		t.Errorf("relinked target: got %q, want %q", got, want)
	}

	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := symlinkDiff(entry), "- (not a symlink)\n+ -> "+dir; got != want {
		t.Errorf("regular file target: got %q, want %q", got, want)
	}
}
