package packages

import (
	"testing"
)

// A tap formula is reported by `brew list --formula` under its bare name
// (`hermes`), but users declare it by the fully-qualified name they installed it
// with (`jamf/internal-tap/hermes`) -- and `brew leaves` prints that qualified form
// too. Matching either spelling against the other is what stops such a package
// from being reported missing forever and reinstalled on every apply.
func TestBrewNameIndexMatchesBothSpellings(t *testing.T) {
	// What `brew list --formula --full-name` and `--cask` actually print.
	installed := newBrewIndex([]string{
		"jamf/internal-tap/hermes",
		"atlassian/acli/acli",
		"ripgrep",
	})

	tests := []struct {
		declared string
		want     bool
		why      string
	}{
		{"jamf/internal-tap/hermes", true, "declared exactly as installed"},
		{"hermes", true, "declared bare, installed from a tap"},
		{"atlassian/acli/acli", true, "qualified name where formula and tap share a name"},
		{"acli", true, "bare form of that same formula"},
		{"ripgrep", true, "plain core formula"},
		{"other/tap/hermes", true, "same formula name from a different tap spelling"},
		{"notinstalled", false, "genuinely absent"},
		{"jamf/internal-tap/absent", false, "absent, qualified"},
		{"erme", false, "substring of an installed name must not match"},
		{"hermes-extra", false, "superstring must not match"},
	}

	for _, tc := range tests {
		t.Run(tc.declared, func(t *testing.T) {
			if got := installed.has(tc.declared); got != tc.want {
				t.Errorf("has(%q) = %v, want %v (%s)", tc.declared, got, tc.want, tc.why)
			}
		})
	}
}

// Two different taps can provide the same formula name. Bare lookup cannot tell
// them apart, which is the documented trade-off: prefer reporting an installed
// package over reinstalling it forever.
func TestBrewNameIndexBareLookupIsAmbiguous(t *testing.T) {
	installed := newBrewIndex([]string{"tap-a/x/thing"})

	if !installed.has("thing") {
		t.Error("bare name should match a tap-installed formula")
	}
	if !installed.has("tap-b/y/thing") {
		t.Error("a same-named formula from another tap matches by design; see brewIndex")
	}
}

func TestBrewNameIndexEmpty(t *testing.T) {
	installed := newBrewIndex(nil)

	if installed.has("anything") {
		t.Error("empty index must not match")
	}
	if installed.has("") {
		t.Error("empty index must not match an empty name")
	}
}

// A blank line in brew's output must not become a matchable entry, or an empty
// declared name would look installed.
func TestBrewNameIndexIgnoresBlanks(t *testing.T) {
	installed := newBrewIndex([]string{"", "  ", "ripgrep"})

	if installed.has("") {
		t.Error("empty name must not match")
	}
	if !installed.has("ripgrep") {
		t.Error("real entry should still match")
	}
}

