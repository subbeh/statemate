package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

func TestDiscoverTemplateFiles(t *testing.T) {
	isolateHome(t)
	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}

	files := []string{
		"mate.yaml",
		"common/plain.conf",
		"common/app.conf#template",
		"common/work.conf#template#profile:work",
		"common/home.conf#template#profile:home",
		"common/.mate.toml",
		"common/.matescripts/setup.sh#template",
		"common/.matescripts/work.sh#template#profile:work",
		"common/.matescripts/raw.sh",
		".matescripts/root.sh#template",
		".matescripts/nested/deep.sh#template",
		".matescripts/home.sh#template#profile:home",
	}
	for _, f := range files {
		path := filepath.Join(repo, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		content := ""
		if f == "mate.yaml" {
			content = "sources: [common]\nprofiles:\n  home: {}\n  work: {}\n"
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Load(filepath.Join(repo, "mate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sourcePaths := cfg.ResolveSourcePaths(cfg.Sources)

	rel := func(paths []string) []string {
		var out []string
		for _, p := range paths {
			r, err := filepath.Rel(repo, p)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		sort.Strings(out)
		return out
	}

	tests := []struct {
		profile string
		want    []string
	}{
		{
			// Under a profile, apply deploys only that profile's files and runs
			// only its scripts; discovery must not fetch secrets for the rest.
			profile: "home",
			want: []string{
				".matescripts/home.sh#template#profile:home",
				".matescripts/nested/deep.sh#template",
				".matescripts/root.sh#template",
				"common/.mate.toml",
				"common/.matescripts/setup.sh#template",
				"common/app.conf#template",
				"common/home.conf#template#profile:home",
			},
		},
		{
			// With no profile apply deploys every file but runs only scripts
			// without a #profile: attribute.
			profile: "",
			want: []string{
				".matescripts/nested/deep.sh#template",
				".matescripts/root.sh#template",
				"common/.mate.toml",
				"common/.matescripts/setup.sh#template",
				"common/app.conf#template",
				"common/home.conf#template#profile:home",
				"common/work.conf#template#profile:work",
			},
		},
	}

	for _, tc := range tests {
		t.Run("profile="+tc.profile, func(t *testing.T) {
			got := rel(discoverTemplateFiles(cfg, tc.profile, sourcePaths))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v\nwant %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v\nwant %v", got, tc.want)
				}
			}
		})
	}
}
