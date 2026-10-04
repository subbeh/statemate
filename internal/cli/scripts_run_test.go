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

// A #template script run by hand renders with the same context mate apply gives
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
	if err := toKey.EncryptToFile([]byte("greeting: hello\n"), filepath.Join(repo, "vars.yaml#encrypted")); err != nil {
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
	if err := os.WriteFile(filepath.Join(repo, ".matescripts", "show.sh#template"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "run", RunE: runScript}
	cmd.Flags().String("config", cfgPath, "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")

	if err := runScript(cmd, []string{"show.sh#template"}); err != nil {
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
