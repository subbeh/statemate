package state

import (
	"path/filepath"
	"testing"
)

// Renaming sources from the long attributes to the short ones must not make
// scripts that already ran look unrun: their records follow the new names.
func TestOpenRenamesLongAttrsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	old := "/repo/zsh/.matescripts/50-zsh-init.sh#once#after#template"
	renamed := "/repo/zsh/.matescripts/50-zsh-init.sh#once#after#tmpl"

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RecordScriptRun(old, "hash"); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordScriptFailure("/repo/x#template#encrypted"); err != nil {
		t.Fatal(err)
	}
	// Stand in for a database written before the migration existed.
	if _, err := db.db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if run, err := db.GetScriptRun(renamed); err != nil || run == nil {
		t.Errorf("run record not found under the new name: run=%v err=%v", run, err)
	}
	if failed, _ := db.HasScriptFailed("/repo/x#tmpl#enc"); !failed {
		t.Error("adjacent long attributes should both be renamed")
	}
	if run, _ := db.GetScriptRun(old); run != nil {
		t.Error("the old name should no longer have a record")
	}
}

func TestShortAttrPath(t *testing.T) {
	for in, want := range map[string]string{
		"/r/ssh/.ssh/config#encrypted#perm:600": "/r/ssh/.ssh/config#enc#perm:600",
		"/r/a#template/b#symlink":               "/r/a#tmpl/b#link",
		"/r/note#template-copy":                 "/r/note#template-copy",
		"/r/plain":                              "/r/plain",
	} {
		if got := shortAttrPath(in); got != want {
			t.Errorf("shortAttrPath(%q) = %q, want %q", in, got, want)
		}
	}
}
