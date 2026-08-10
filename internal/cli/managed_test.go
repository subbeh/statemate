package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/source"
)

// managedFilterEnv builds two entries whose targets share a basename, which is
// the case that made a bare "config" filter ambiguous.
func managedFilterEnv(t *testing.T) (targetDir string, ssh, git *source.Entry) {
	t.Helper()

	targetDir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(targetDir); err == nil {
		targetDir = resolved
	}

	for _, sub := range []string{".ssh", ".config/git"} {
		if err := os.MkdirAll(filepath.Join(targetDir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{".ssh/config", ".config/git/config"} {
		if err := os.WriteFile(filepath.Join(targetDir, rel), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}

	// The ssh source file exists on disk so an absolute source path is also a
	// resolvable, unambiguous lookup.
	sshSrcDir := filepath.Join(repo, "ssh", ".ssh")
	if err := os.MkdirAll(sshSrcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshSrcDir, "config#encrypted"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	ssh = &source.Entry{
		SourcePath: filepath.Join(sshSrcDir, "config#encrypted"),
		TargetPath: filepath.Join(targetDir, ".ssh", "config"),
		RelPath:    ".ssh/config#encrypted",
	}
	git = &source.Entry{
		SourcePath: filepath.Join(repo, "git", ".config", "git", "config#template"),
		TargetPath: filepath.Join(targetDir, ".config", "git", "config"),
		RelPath:    ".config/git/config#template",
	}
	return targetDir, ssh, git
}

func TestManagedFilter_AbsolutePathMatchesExactlyOne(t *testing.T) {
	targetDir, ssh, git := managedFilterEnv(t)
	filter := filepath.Join(targetDir, ".ssh", "config")

	if !matchesManagedFilter(ssh, "ssh/.ssh/config#encrypted", filter) {
		t.Error("absolute target path should match its own entry")
	}
	if matchesManagedFilter(git, "git/.config/git/config#template", filter) {
		t.Error("absolute target path must not match a different file with the same basename")
	}
}

func TestManagedFilter_BareNameResolvesAgainstCwd(t *testing.T) {
	targetDir, ssh, git := managedFilterEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(filepath.Join(targetDir, ".ssh")); err != nil {
		t.Fatal(err)
	}

	// "config" exists in cwd, so it is an exact path -- not a name fragment.
	if !matchesManagedFilter(ssh, "ssh/.ssh/config#encrypted", "config") {
		t.Error("bare name should match the file in the current directory")
	}
	if matchesManagedFilter(git, "git/.config/git/config#template", "config") {
		t.Error("bare name resolved to a real path must not also match other basenames")
	}
}

func TestManagedFilter_BareNameFallsBackToLooseMatch(t *testing.T) {
	_, ssh, git := managedFilterEnv(t)

	// Run from a directory with no "config" file, so it stays a fragment and
	// loose matching applies to both entries.
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if !matchesManagedFilter(ssh, "ssh/.ssh/config#encrypted", "config") {
		t.Error("expected loose match for ssh entry")
	}
	if !matchesManagedFilter(git, "git/.config/git/config#template", "config") {
		t.Error("expected loose match for git entry")
	}
}

func TestManagedFilter_SourceNameStillFiltersLoosely(t *testing.T) {
	_, ssh, git := managedFilterEnv(t)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if !matchesManagedFilter(ssh, "ssh/.ssh/config#encrypted", "ssh") {
		t.Error("source name should match entries in that source")
	}
	if matchesManagedFilter(git, "git/.config/git/config#template", "ssh") {
		t.Error("source name should not match an unrelated source")
	}
}

func TestManagedFilter_SourcePathMatches(t *testing.T) {
	_, ssh, _ := managedFilterEnv(t)

	// The absolute source path is also an unambiguous lookup.
	if !matchesManagedFilter(ssh, "ssh/.ssh/config#encrypted", ssh.SourcePath) {
		t.Error("absolute source path should match its entry")
	}
}
