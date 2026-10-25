package plan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	home := newRepo(t, map[string]string{
		"mate.yaml":                       "sources: [nvim, git]\nprofiles:\n  work: {}\n  home: {}\n",
		"nvim/.config/nvim/init.lua#tmpl": "-- {{ .Hostname }}\n",
		"git/.gitconfig#profile:work":     "work\n",
		"git/.gitconfig#profile:home":     "home\n",
		"git/notes.txt":                   "x\n",
		".matedata/vars.yaml":             "a: 1\n",
	})
	repo := os.Getenv("STATEMATE_DIR")
	env, err := NewEnv("", "work")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		arg, cwd, want string
	}{
		{arg: "nvim/.config/nvim/init.lua#tmpl", cwd: repo, want: "init.lua#tmpl"},
		{arg: "nvim/.config/nvim/init.lua", cwd: repo, want: "init.lua#tmpl"},
		{arg: "~/.config/nvim/init.lua", cwd: repo, want: "init.lua#tmpl"},
		{arg: filepath.Join(home, ".config", "nvim", "init.lua"), cwd: repo, want: "init.lua#tmpl"},
		{arg: ".config/nvim/init.lua", cwd: home, want: "init.lua#tmpl"},
		// Not in the current directory, so tried against the source directory.
		{arg: "git/notes.txt", cwd: home, want: "notes.txt"},
		// The variant the current profile deploys.
		{arg: "~/.gitconfig", cwd: repo, want: ".gitconfig#profile:work"},
	} {
		t.Chdir(tc.cwd)
		got, err := env.Resolve(tc.arg)
		if err != nil {
			t.Errorf("Resolve(%q) in %s: %v", tc.arg, tc.cwd, err)
			continue
		}
		if filepath.Base(got.SourcePath) != tc.want {
			t.Errorf("Resolve(%q) in %s = %s, want %s", tc.arg, tc.cwd, got.SourcePath, tc.want)
		}
	}

	t.Chdir(repo)
	for _, arg := range []string{
		"init.lua", // a bare name is not searched for
		".matedata/vars.yaml",
		"~/.bashrc",
	} {
		_, err := env.Resolve(arg)
		var notManaged *NotManagedError
		if !errors.As(err, &notManaged) {
			t.Errorf("Resolve(%q) = %v, want NotManagedError", arg, err)
		}
	}
	_, err = env.Resolve(".matedata/vars.yaml")
	var notManaged *NotManagedError
	if errors.As(err, &notManaged) && notManaged.Path != filepath.Join(repo, ".matedata", "vars.yaml") {
		t.Errorf("NotManagedError.Path = %s, want the file in the repo", notManaged.Path)
	}

	noProfile, err := NewEnv("", "")
	if err != nil {
		t.Fatal(err)
	}
	if noProfile.Profile != "" {
		t.Fatalf("detected profile %q, want none", noProfile.Profile)
	}
	if _, err := noProfile.Resolve("~/.gitconfig"); err == nil || !strings.Contains(err.Error(), "matches several files") {
		t.Errorf("two variants and no profile: got %v, want an ambiguity error", err)
	}
}
