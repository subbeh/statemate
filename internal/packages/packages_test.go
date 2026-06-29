package packages

import (
	"testing"
)

func TestParsePackageSpec(t *testing.T) {
	tests := []struct {
		spec        string
		wantName    string
		wantVersion string
	}{
		{"neovim", "neovim", ""},
		{"node@20", "node", "20"},
		{"python@3.11", "python", "3.11"},
		{"foo@bar@baz", "foo", "bar@baz"},
	}

	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			name, version := ParsePackageSpec(tc.spec)
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if version != tc.wantVersion {
				t.Errorf("version = %q, want %q", version, tc.wantVersion)
			}
		})
	}
}

func TestPackageString(t *testing.T) {
	tests := []struct {
		pkg  Package
		want string
	}{
		{Package{Name: "neovim"}, "neovim"},
		{Package{Name: "node", Version: "20"}, "node@20"},
	}

	for _, tc := range tests {
		if got := tc.pkg.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.pkg, got, tc.want)
		}
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusInstalled, "installed"},
		{StatusMissing, "missing"},
		{StatusExtra, "extra"},
		{StatusVersionMismatch, "version mismatch"},
	}

	for _, tc := range tests {
		if got := tc.status.String(); got != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestAppendUnique(t *testing.T) {
	slice := []string{"a", "b"}

	result := appendUnique(slice, "c")
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}

	result = appendUnique(result, "b")
	if len(result) != 3 {
		t.Errorf("expected still 3 elements, got %d", len(result))
	}
}
