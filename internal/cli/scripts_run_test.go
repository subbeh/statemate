package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/secrets"
)

// isolateHome points every directory mate reads or writes at a temp dir.
func isolateHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("STATEMATE_DIR", "")
	return root
}

// A #tmpl script run by hand renders with the same context mate apply gives
// it: cached secrets and encrypted var_files both resolve.
func TestRunScript_TemplateSeesSecretsAndEncryptedVars(t *testing.T) {
	root := isolateHome(t)
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".matescripts"), 0755); err != nil {
		t.Fatal(err)
	}

	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "key.txt")
	if err := os.WriteFile(keyPath, []byte(id.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	toKey, err := encrypt.NewAgeEncryptor("", "", []string{id.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}

	cachePath := filepath.Join(root, "secrets.age")
	key := secrets.CacheKey{Provider: "bitwarden", Item: "github", Type: "field", Field: "token"}
	cache, err := json.Marshal(&secrets.Cache{Items: map[string]*secrets.CachedValue{
		key.String(): {Value: "s3cret"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := toKey.EncryptToFile(cache, cachePath); err != nil {
		t.Fatal(err)
	}
	if err := toKey.EncryptToFile([]byte("greeting: hello\n"), filepath.Join(repo, "vars.yaml#enc")); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(repo, "mate.yaml")
	cfg := "sources: []\n" +
		"var_files: [vars.yaml]\n" +
		"secrets_cache: " + cachePath + "\n" +
		"age:\n  identity: " + keyPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "out.txt")
	script := "#!/bin/sh\necho '{{ .Vars.greeting }} {{ bitwarden \"github\" \"field\" \"token\" }}' > " + out + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".matescripts", "show.sh#tmpl"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "run", RunE: runScript}
	cmd.Flags().String("config", cfgPath, "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Bool("dry-run", false, "")

	if err := runScript(cmd, []string{"show.sh#tmpl"}); err != nil {
		t.Fatalf("runScript: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "hello s3cret" {
		t.Errorf("script rendered %q, want %q", strings.TrimSpace(string(got)), "hello s3cret")
	}
}

// --tags runs the active profile's tagged scripts, before ones first, and
// nothing else.
func TestRunScript_Tags(t *testing.T) {
	root := isolateHome(t)
	repo := filepath.Join(root, "repo")
	dir := filepath.Join(repo, ".matescripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte("sources: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(root, "log")
	for name, tags := range map[string]string{
		"01-late.sh#after":        "update",
		"02-early.sh#before":      "brew update",
		"03-other.sh":             "other",
		"04-work.sh#profile:work": "update",
		"05-untagged.sh#always":   "",
	} {
		body := "#!/bin/sh\n# Tags: " + tags + "\necho " + strings.SplitN(name, "#", 2)[0] + " >> " + log + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// A fresh command per run: a second Set on a slice flag appends.
	withTags := func(tags ...string) *cobra.Command {
		cmd := &cobra.Command{Use: "run", RunE: runScript}
		cmd.Flags().String("config", cfgPath, "")
		cmd.Flags().String("profile", "", "")
		cmd.Flags().Bool("dry-run", false, "")
		cmd.Flags().StringSlice("tags", tags, "")
		return cmd
	}

	if err := runScript(withTags("update"), nil); err != nil {
		t.Fatalf("runScript: %v", err)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if want := "02-early.sh\n01-late.sh\n"; string(got) != want {
		t.Errorf("ran %q, want %q", got, want)
	}

	// Several tags run the scripts with any of them, each once.
	_ = os.Remove(log)
	if err := runScript(withTags("brew", "other"), nil); err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if got, _ := os.ReadFile(log); string(got) != "02-early.sh\n03-other.sh\n" {
		t.Errorf("ran %q with two tags", got)
	}

	if err := runScript(withTags("missing"), nil); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected an error naming the unknown tag, got %v", err)
	}
}

func TestScriptsRunArgs(t *testing.T) {
	for _, tc := range []struct {
		tags []string
		args []string
		ok   bool
	}{
		{nil, []string{"a.sh"}, true},
		{[]string{"update"}, nil, true},
		{nil, nil, false},
		{[]string{"update"}, []string{"a.sh"}, false},
		{nil, []string{"a.sh", "b.sh"}, false},
	} {
		cmd := &cobra.Command{}
		cmd.Flags().StringSlice("tags", tc.tags, "")
		if err := scriptsRunCmd.Args(cmd, tc.args); (err == nil) != tc.ok {
			t.Errorf("tags %q args %v: err = %v", tc.tags, tc.args, err)
		}
	}
}
