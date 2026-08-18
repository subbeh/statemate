package target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

// importFixture sets up a source and target for one #import file and applies it
// once, so the state DB has a baseline to compare later drift against.
type importFixture struct {
	t          *testing.T
	db         *state.DB
	tree       *source.Tree
	sourcePath string
	targetPath string
	targetDir  string
}

func newImportFixture(t *testing.T, sourceName, content string) *importFixture {
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

	sourcePath := filepath.Join(sourceDir, sourceName)
	if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{sourceDir})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(tree.Files()) != 1 {
		t.Fatalf("expected 1 file in tree, got %d", len(tree.Files()))
	}

	f := &importFixture{
		t:          t,
		db:         db,
		tree:       tree,
		sourcePath: sourcePath,
		targetPath: tree.Files()[0].TargetPath,
		targetDir:  targetDir,
	}
	return f
}

// apply runs a real apply, establishing (or updating) the recorded state.
func (f *importFixture) apply() *ApplyResult {
	f.t.Helper()
	res, err := NewApplier(f.db, nil, nil, false, false, 0).Apply(f.tree)
	if err != nil {
		f.t.Fatalf("apply failed: %v", err)
	}
	return res
}

func (f *importFixture) status() ChangeStatus {
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

func (f *importFixture) write(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *importFixture) read(path string) string {
	f.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}

// The application rewrote the file while the source stood still. That is the
// whole point of #import: take the target as truth without prompting.
func TestImport_TargetChangedIsImported(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)
	f.apply()

	f.write(f.targetPath, `{"a":2}`)

	if got := f.status(); got != StatusImport {
		t.Fatalf("status: got %v, want import", got)
	}

	res := f.apply()
	if res.Imported != 1 {
		t.Errorf("expected 1 imported, got %d imported / %d applied", res.Imported, res.Applied)
	}

	if got := f.read(f.sourcePath); got != `{"a":2}` {
		t.Errorf("source should hold the target's content, got %q", got)
	}

	// A second run has nothing left to do.
	if got := f.status(); got != StatusUnchanged {
		t.Errorf("after importing, status should be unchanged, got %v", got)
	}
}

// Without #import the same drift is a conflict, which is what prompts.
func TestImport_WithoutAttributeTargetChangeIsConflict(t *testing.T) {
	f := newImportFixture(t, "settings.json", `{"a":1}`)
	f.apply()

	f.write(f.targetPath, `{"a":2}`)

	if got := f.status(); got != StatusConflict {
		t.Errorf("status without #import: got %v, want conflict", got)
	}
}

// Editing the source of an #import file must still deploy normally -- otherwise
// the attribute would make a file read-only from the user's side.
func TestImport_SourceChangedStillDeploys(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)
	f.apply()

	f.write(f.sourcePath, `{"a":9}`)

	if got := f.status(); got != StatusModified {
		t.Fatalf("status: got %v, want modified", got)
	}

	res := f.apply()
	if res.Applied != 1 {
		t.Errorf("expected 1 applied, got %d applied / %d imported", res.Applied, res.Imported)
	}
	if got := f.read(f.targetPath); got != `{"a":9}` {
		t.Errorf("target should have the source's content, got %q", got)
	}
}

// Both sides moved, so there is no safe silent answer: importing would discard
// the source edit. Fall back to the conflict prompt.
func TestImport_BothChangedIsConflict(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)
	f.apply()

	f.write(f.sourcePath, `{"a":"source"}`)
	f.write(f.targetPath, `{"a":"target"}`)

	if got := f.status(); got != StatusConflict {
		t.Errorf("status: got %v, want conflict when both sides changed", got)
	}
}

// A fresh machine has no target yet. #import must still deploy, or the file
// would never exist and the attribute would be useless for bootstrapping.
func TestImport_MissingTargetIsDeployed(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)

	if got := f.status(); got != StatusNew {
		t.Fatalf("status: got %v, want new", got)
	}

	f.apply()

	if got := f.read(f.targetPath); got != `{"a":1}` {
		t.Errorf("target should have been created from the source, got %q", got)
	}
}

// Nothing changed on either side: no work, and certainly no import.
func TestImport_UnchangedDoesNothing(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)
	f.apply()

	if got := f.status(); got != StatusUnchanged {
		t.Errorf("status: got %v, want unchanged", got)
	}

	res := f.apply()
	if res.Imported != 0 || res.Applied != 0 {
		t.Errorf("expected no work, got %d applied / %d imported", res.Applied, res.Imported)
	}
}

// Importing a template would write the rendered output over the template,
// destroying the source. Reject the combination rather than doing it.
func TestImport_TemplateCombinationIsRejected(t *testing.T) {
	f := newImportFixture(t, "settings.json#import#template", `{"a":1}`)

	_, err := ComputeChanges(f.tree, f.db)
	if err == nil {
		t.Fatal("expected an error for #import#template")
	}
	if !strings.Contains(err.Error(), "#template") {
		t.Errorf("error should explain the conflict, got: %v", err)
	}
}

// The motivating file (~/.claude/settings.json) is #encrypted, so an import must
// encrypt what it writes back -- otherwise it would leave plaintext in the repo.
func TestImport_EncryptedSourceStaysEncrypted(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source", "app")
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	// The source starts as ciphertext, as it would in a real repo.
	original := []byte(`{"a":1}`)
	ciphertext, err := enc.Encrypt(original)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "settings.json#encrypted#import")
	if err := os.WriteFile(sourcePath, ciphertext, 0644); err != nil {
		t.Fatal(err)
	}

	db, err := state.Open(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	scanner := source.NewScanner(targetDir, "")
	tree, err := scanner.Scan([]string{sourceDir})
	if err != nil {
		t.Fatal(err)
	}

	applier := NewApplier(db, nil, enc, false, false, 0)
	if _, err := applier.Apply(tree); err != nil {
		t.Fatalf("initial apply: %v", err)
	}

	targetPath := tree.Files()[0].TargetPath
	if got, _ := os.ReadFile(targetPath); string(got) != string(original) {
		t.Fatalf("target should be decrypted plaintext, got %q", got)
	}

	// The application rewrites the plaintext target.
	updated := []byte(`{"a":2}`)
	if err := os.WriteFile(targetPath, updated, 0644); err != nil {
		t.Fatal(err)
	}

	res, err := ComputeChanges(tree, db, ComputeOpts{Enc: enc})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 1 || res.Changes[0].Status != StatusImport {
		t.Fatalf("expected a pending import, got %+v", res.Changes)
	}

	if _, err := applier.Apply(tree); err != nil {
		t.Fatalf("import apply: %v", err)
	}

	// The source must be ciphertext holding the new value, not plaintext.
	stored, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), `"a"`) {
		t.Error("imported source is plaintext; it must be re-encrypted")
	}
	decrypted, err := enc.Decrypt(stored)
	if err != nil {
		t.Fatalf("imported source does not decrypt: %v", err)
	}
	if string(decrypted) != string(updated) {
		t.Errorf("decrypted source: got %q, want %q", decrypted, updated)
	}
}

// A dry run must report the import without touching either side.
func TestImport_DryRunChangesNothing(t *testing.T) {
	f := newImportFixture(t, "settings.json#import", `{"a":1}`)
	f.apply()

	f.write(f.targetPath, `{"a":2}`)

	res, err := NewApplier(f.db, nil, nil, true, false, 0).Apply(f.tree)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if res.Imported != 1 {
		t.Errorf("dry run should report 1 import, got %d", res.Imported)
	}

	if got := f.read(f.sourcePath); got != `{"a":1}` {
		t.Errorf("dry run must not modify the source, got %q", got)
	}
	if got := f.read(f.targetPath); got != `{"a":2}` {
		t.Errorf("dry run must not modify the target, got %q", got)
	}
}
