package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/config"
	tmpl "github.com/subbeh/statemate/internal/template"
)

func writeTemplate(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tmpl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func discover(t *testing.T, content string) []FetchItem {
	t.Helper()
	ctx, err := tmpl.NewContext(&config.Config{
		Variables: map[string]any{"user": "u123456-sub1"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return DiscoverByRendering([]string{writeTemplate(t, content)}, ctx, nil)
}

func TestDiscoverByRendering(t *testing.T) {
	items := discover(t, `{{ bitwarden "item" "field" "password" }}`)

	if len(items) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(items))
	}
	if items[0].Item != "item" || items[0].Field != "password" {
		t.Errorf("got %+v", items[0])
	}
}

// Discovery skips any template it cannot parse, so its function set must cover
// everything Render accepts. When it had a hand-rolled subset, a template using
// a sprig function parsed during apply but not during discovery -- so its
// secrets were never fetched and the apply failed on a cache miss.
func TestDiscoverByRendering_SprigFuncs(t *testing.T) {
	items := discover(t, `
Host {{ (splitList "-" .Vars.user) | first }}
Pass {{ bitwarden "hetzner.com" "field" "storage-box-password" }}
`)

	if len(items) != 1 {
		t.Fatalf("expected the secret to be discovered alongside a sprig call, got %d", len(items))
	}
	if items[0].Field != "storage-box-password" {
		t.Errorf("got %+v", items[0])
	}
}

// The placeholder value substituted for a secret must not trip validation, or
// discovery aborts before reaching later secrets in the same template.
func TestDiscoverByRendering_RequiredDoesNotAbort(t *testing.T) {
	items := discover(t, `
{{ required (bitwarden "a" "field" "one") }}
{{ bitwarden "b" "field" "two" }}
`)

	if len(items) != 2 {
		t.Fatalf("expected both secrets, got %d", len(items))
	}
}

// cmd shells out, which discovery must not do -- it renders untrusted-ish
// templates purely to observe which secrets they ask for.
func TestDiscoverByRendering_CmdIsNeutralized(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	items := discover(t, `{{ cmd "touch `+marker+`" }}{{ bitwarden "a" "field" "f" }}`)

	if len(items) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(items))
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("cmd executed during discovery; it must be a no-op")
	}
}
