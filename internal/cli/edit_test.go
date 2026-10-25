package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/plan"
)

// editTestEnv builds a small dotfiles repo and returns its Env, mirroring what
// runEdit works with.
func editTestEnv(t *testing.T) (*plan.Env, string) {
	t.Helper()

	repo := t.TempDir()
	targetBase := t.TempDir()

	// nvim source with one managed file
	nvimDir := filepath.Join(repo, "nvim", ".config", "nvim")
	if err := os.MkdirAll(nvimDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- init"), 0644); err != nil {
		t.Fatal(err)
	}

	// an include file outside any source dir, with the #enc suffix
	dataDir := filepath.Join(repo, ".matedata")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "secrets.yaml#enc"), []byte("cipher"), 0600); err != nil {
		t.Fatal(err)
	}

	// a file inside the repo that is not a managed source file
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Note: the encrypted file is intentionally not listed under include: here.
	// Loading an encrypted include requires a configured age identity, and
	// resolveEditPath resolves by filesystem path rather than config membership.
	cfgContent := "target_base: " + targetBase + "\n" +
		"sources:\n  - nvim\n"
	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := plan.NewEnv(cfgPath, "")
	if err != nil {
		t.Fatalf("loading env: %v", err)
	}

	// Resolve symlinks so expected paths match what resolveEditPath returns
	// (macOS temp dirs are symlinked via /var -> /private/var).
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}

	return env, repo
}

func TestResolveEditPath_IncludeFileRelative(t *testing.T) {
	env, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	got, managed, err := resolveEditPath(".matedata/secrets.yaml#enc", env)
	if err != nil {
		t.Fatalf("expected include file to resolve, got error: %v", err)
	}
	want := filepath.Join(repo, ".matedata", "secrets.yaml#enc")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if managed {
		t.Error("include file should not be reported as a managed source file")
	}
}

func TestResolveEditPath_UnmanagedTargetErrors(t *testing.T) {
	env, _ := editTestEnv(t)

	outside := filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveEditPath(outside, env); err == nil {
		t.Error("expected error for file outside source_dir with no managed target")
	}
}

func TestResolveEditPath_UnmanagedFileInRepoIsEditable(t *testing.T) {
	env, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	// README.md is inside source_dir but is not a managed source file.
	got, managed, err := resolveEditPath("README.md", env)
	if err != nil {
		t.Fatalf("files under source_dir should be editable, got error: %v", err)
	}
	if got != filepath.Join(repo, "README.md") {
		t.Errorf("unexpected path: %q", got)
	}
	if managed {
		t.Error("README.md should not be reported as a managed source file")
	}
}

func TestResolveEditPath_MissingFileErrors(t *testing.T) {
	env, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveEditPath("does-not-exist.txt", env); err == nil {
		t.Error("expected error for nonexistent file under source_dir")
	}
}
