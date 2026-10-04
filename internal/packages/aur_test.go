package packages

import (
	"os"
	"path/filepath"
	"testing"
)

// fakePath replaces PATH with a directory holding empty executables of the given
// names, so helper detection sees exactly those and nothing on the real system.
func fakePath(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

// With aur_helper unset, a machine that only has paru must still get its aur:
// list handled. Defaulting to a fixed "yay" made the AUR manager unavailable
// there, and every AUR package was silently ignored.
func TestNewAURManager_DetectsHelper(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed []string
		want      string
	}{
		{"only paru", []string{"paru"}, "paru"},
		{"only yay", []string{"yay"}, "yay"},
		{"both prefer yay", []string{"paru", "yay"}, "yay"},
		{"neither", nil, "yay"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakePath(t, tc.installed...)
			m := NewAURManager("")
			if m.helper != tc.want {
				t.Errorf("helper = %q, want %q", m.helper, tc.want)
			}
			if got, want := m.IsAvailable(), len(tc.installed) > 0; got != want {
				t.Errorf("IsAvailable() = %v, want %v", got, want)
			}
		})
	}
}

// An explicitly configured helper is used as is, even when another is installed.
func TestNewAURManager_ConfiguredHelperWins(t *testing.T) {
	fakePath(t, "yay")
	if m := NewAURManager("paru"); m.helper != "paru" || m.IsAvailable() {
		t.Errorf("helper = %q (available %v), want unavailable paru", m.helper, m.IsAvailable())
	}
}
