package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindUnmanaged(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		".zshrc",                // managed
		".bashrc",               // unmanaged file beside a managed one
		".config/nvim/init.lua", // managed, two levels down
		".config/nvim/lazy.json",
		".config/htop/htoprc", // whole directory unmanaged
		"empty/stray",         // inside a declared empty directory
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	targets := []string{
		filepath.Join(root, ".zshrc"),
		filepath.Join(root, ".config/nvim/init.lua"),
		filepath.Join(root, "empty"),
	}
	got, err := findUnmanaged(root, targets)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		filepath.Join(root, ".bashrc"),
		filepath.Join(root, ".config/htop") + "/",
		filepath.Join(root, ".config/nvim/lazy.json"),
		filepath.Join(root, "empty/stray"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("findUnmanaged =\n%v\nwant\n%v", got, want)
	}
}
