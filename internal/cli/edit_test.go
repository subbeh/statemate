package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/source"
)

// editTestEnv builds a small dotfiles repo and returns the loaded config plus
// the scanned entries, mirroring what runEdit works with.
func editTestEnv(t *testing.T) (*config.Config, []*source.Entry, string) {
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

	// an include file outside any source dir, with the #encrypted suffix
	dataDir := filepath.Join(repo, ".matedata")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "secrets.yaml#encrypted"), []byte("cipher"), 0600); err != nil {
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

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	scanner := source.NewScannerWithIgnore(cfg.TargetBase, cfg.SourceDir(), nil, cfg.Ignore)
	tree, err := scanner.Scan(cfg.AbsoluteSources())
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	// Resolve symlinks so expected paths match what resolveEditPath returns
	// (macOS temp dirs are symlinked via /var -> /private/var).
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}

	return cfg, tree.Files(), repo
}

func TestResolveEditPath_IncludeFileRelative(t *testing.T) {
	cfg, entries, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	got, managed, err := resolveEditPath(".matedata/secrets.yaml#encrypted", cfg, entries)
	if err != nil {
		t.Fatalf("expected include file to resolve, got error: %v", err)
	}
	want := filepath.Join(repo, ".matedata", "secrets.yaml#encrypted")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if managed {
		t.Error("include file should not be reported as a managed source file")
	}
}

func TestResolveEditPath_ManagedSourceRelative(t *testing.T) {
	cfg, entries, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	got, managed, err := resolveEditPath("nvim/.config/nvim/init.lua", cfg, entries)
	if err != nil {
		t.Fatalf("expected source file to resolve, got error: %v", err)
	}
	want := filepath.Join(repo, "nvim", ".config", "nvim", "init.lua")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !managed {
		t.Error("expected managed source file to be reported as managed")
	}
}

func TestResolveEditPath_TargetResolvesToSource(t *testing.T) {
	cfg, entries, repo := editTestEnv(t)

	if len(entries) == 0 {
		t.Fatal("no entries scanned")
	}
	target := entries[0].TargetPath

	got, managed, err := resolveEditPath(target, cfg, entries)
	if err != nil {
		t.Fatalf("expected target path to resolve, got error: %v", err)
	}
	want := filepath.Join(repo, "nvim", ".config", "nvim", "init.lua")
	if got != want {
		t.Errorf("target should resolve to source: got %q, want %q", got, want)
	}
	if !managed {
		t.Error("expected target-resolved source to be reported as managed")
	}
}

func TestResolveEditPath_UnmanagedTargetErrors(t *testing.T) {
	cfg, entries, _ := editTestEnv(t)

	outside := filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveEditPath(outside, cfg, entries); err == nil {
		t.Error("expected error for file outside source_dir with no managed target")
	}
}

func TestResolveEditPath_UnmanagedFileInRepoIsEditable(t *testing.T) {
	cfg, entries, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	// README.md is inside source_dir but is not a managed source file.
	got, managed, err := resolveEditPath("README.md", cfg, entries)
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
	cfg, entries, repo := editTestEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveEditPath("does-not-exist.txt", cfg, entries); err == nil {
		t.Error("expected error for nonexistent file under source_dir")
	}
}

func TestResolveEditPath_NoSuffixMatching(t *testing.T) {
	cfg, entries, repo := editTestEnv(t)

	// Run from a subdirectory of the repo. A bare basename must NOT resolve via
	// suffix search -- resolution is strictly cwd-relative.
	sub := filepath.Join(repo, "nvim")
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveEditPath("init.lua", cfg, entries); err == nil {
		t.Error("expected bare basename not to resolve via suffix search")
	}
}
