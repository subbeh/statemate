package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/state"
)

// trackedTargets builds the tracked entries forget matches against. Stored
// target paths are always absolute.
func trackedTargets(paths ...string) []*state.FileEntry {
	entries := make([]*state.FileEntry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, &state.FileEntry{TargetPath: p})
	}
	return entries
}

// A target path typed relative to the current directory used to match nothing,
// because only a leading ~ was expanded -- so `mate forget Library/.../state.db`
// from home failed while the same path with ~/ in front worked.
func TestForgetPatternResolvesRelativeToCwd(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "Library", "Application Support", "app", "state.db")
	tracked := trackedTargets(target)

	matches := matchTrackedFiles(tracked, forgetPattern("Library/Application Support/app/state.db"))
	if len(matches) != 1 || matches[0] != target {
		t.Errorf("relative path should match %q, got %v", target, matches)
	}
}

func TestForgetPatternExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(home, ".config", "nvim", "init.lua")
	tracked := trackedTargets(target)

	matches := matchTrackedFiles(tracked, forgetPattern("~/.config/nvim/init.lua"))
	if len(matches) != 1 || matches[0] != target {
		t.Errorf("~ path should match %q, got %v", target, matches)
	}
}

func TestForgetPatternGlobMatchesRelative(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	db := filepath.Join(dir, "app", "state.db")
	shm := filepath.Join(dir, "app", "state.db-shm")
	other := filepath.Join(dir, "app", "config.toml")
	tracked := trackedTargets(db, shm, other)

	matches := matchTrackedFiles(tracked, forgetPattern("app/state.db*"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
	for _, m := range matches {
		if m == other {
			t.Errorf("glob should not match %q", other)
		}
	}
}

func TestForgetPatternLeavesAbsoluteAlone(t *testing.T) {
	target := "/etc/keyd/default.conf"
	tracked := trackedTargets(target)

	matches := matchTrackedFiles(tracked, forgetPattern(target))
	if len(matches) != 1 || matches[0] != target {
		t.Errorf("absolute path should match %q, got %v", target, matches)
	}
}
