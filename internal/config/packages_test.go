package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

func TestPackageList(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string // the value of packages:
		toml    string // a whole document, empty to skip TOML
		want    PackageList
		wantErr string
	}{
		{
			name: "bare name",
			yaml: "\n  - ripgrep", toml: `packages = ["ripgrep"]`,
			want: PackageList{{Name: "ripgrep"}},
		},
		{
			name: "one manager",
			yaml: "\n  - gnu-sed: brew", toml: `packages = [{gnu-sed = "brew"}]`,
			want: PackageList{{Name: "gnu-sed", Managers: map[string]string{"brew": "gnu-sed"}}},
		},
		{
			name: "managers with a rename",
			yaml: "\n  - gh: [brew, pacman: github-cli]", toml: `packages = [{gh = ["brew", {pacman = "github-cli"}]}]`,
			want: PackageList{{Name: "gh", Managers: map[string]string{"brew": "gh", "pacman": "github-cli"}}},
		},
		{
			name: "block style",
			yaml: "\n  - tpack:\n      - brew\n      - aur: tpack-bin",
			want: PackageList{{Name: "tpack", Managers: map[string]string{"brew": "tpack", "aur": "tpack-bin"}}},
		},
		{name: "empty", yaml: "", want: nil},
		{
			name: "old format",
			yaml: "\n  brew: [git]", toml: "[packages]\nbrew = [\"git\"]",
			wantErr: "packages is a list now",
		},
		{name: "unknown manager", yaml: "\n  - fd: pacmn", toml: `packages = [{fd = "pacmn"}]`, wantErr: `line 2: package fd: unknown manager "pacmn"`},
		{name: "manager twice", yaml: "\n  - fd: [brew, brew: fd-find]", wantErr: "brew is listed twice"},
		{name: "pacman and aur", yaml: "\n  - tpack: [pacman, aur: tpack-bin]", wantErr: "both pacman and aur"},
		{name: "several names", yaml: "\n  - {fd: brew, jq: brew}", wantErr: "one package per item, got fd, jq"},
		{name: "no managers after the colon", yaml: "\n  - fd:", wantErr: "follow it with a manager"},
		{name: "rename that is not a name", yaml: "\n  - fd: [brew: [a]]", wantErr: "brew must be followed by the package's name there"},
		{name: "empty name", yaml: "\n  - ''", wantErr: "cannot be empty"},
		{name: "not a package", yaml: "\n  - [fd]", wantErr: "is not a package"},
	}

	type doc struct {
		Packages PackageList `yaml:"packages" toml:"packages"`
	}
	check := func(t *testing.T, got PackageList, err error, wantErr string, want PackageList) {
		t.Helper()
		if wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("expected an error containing %q, got %v", wantErr, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var y doc
			err := yaml.Unmarshal([]byte("packages:"+tt.yaml+"\n"), &y)
			check(t, y.Packages, err, tt.wantErr, tt.want)

			if tt.toml != "" {
				var d doc
				_, err := toml.Decode(tt.toml, &d)
				// TOML has no line numbers to report.
				check(t, d.Packages, err, strings.TrimPrefix(tt.wantErr, "line 2: "), tt.want)
			}
		})
	}
}
