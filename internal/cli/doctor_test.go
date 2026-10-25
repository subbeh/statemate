package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
)

// mate doctor must look for the AUR helper the config names, not a default one.
// It used to ignore aur_helper, so a helper outside the default names reported
// no AUR manager even though mate apply used it fine.
func TestDoctor_HonoursConfiguredAURHelper(t *testing.T) {
	tmp := t.TempDir()
	for _, env := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(env, filepath.Join(tmp, strings.ToLower(env)))
	}
	t.Setenv("STATEMATE_DIR", "")
	// An empty PATH: nothing but the configured helper can be found.
	t.Setenv("PATH", t.TempDir())

	helper := filepath.Join(tmp, "custom-aur-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte("sources: []\naur_helper: "+helper+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "doctor", RunE: runDoctor}
	cmd.Flags().String("config", cfgPath, "")
	cmd.Flags().String("profile", "", "")

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := runDoctor(cmd, nil)
	_ = w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("doctor failed: %v\n%s", runErr, out)
	}
	if !strings.Contains(string(out), "[OK] aur") {
		t.Errorf("doctor did not find the configured AUR helper:\n%s", out)
	}
}

// statemate has age built in, so the age binary says nothing about whether
// encryption works; whether the configured identity and recipients load does.
func TestAgeStatus(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(key, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recipient := id.Recipient().String()

	tests := []struct {
		name  string
		age   *config.AgeConfig
		level string
	}{
		{"not configured", nil, "OK"},
		{"identity and recipients", &config.AgeConfig{Identity: key, Recipients: []string{recipient}}, "OK"},
		{"recipients only", &config.AgeConfig{Recipients: []string{recipient}}, "WARN"},
		{"identity only", &config.AgeConfig{Identity: key}, "WARN"},
		{"unreadable identity", &config.AgeConfig{Identity: key + ".missing", Recipients: []string{recipient}}, "ERROR"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			level, msg := ageStatus(&config.Config{Age: tc.age})
			if level != tc.level {
				t.Errorf("level = %s (%s), want %s", level, msg, tc.level)
			}
		})
	}
}

// mate doctor flags #enc files holding plaintext in any source, including
// one only a profile uses, and accepts armored and binary age files alike.
func TestCheckEncrypted_FlagsPlaintext(t *testing.T) {
	root := isolateHome(t)
	repo := filepath.Join(root, "repo")
	files := map[string]string{
		"mate.yaml":                     "sources: [app]\nprofiles:\n  work:\n    sources: [work]\n",
		"app/armored#enc":         "-----BEGIN AGE ENCRYPTED FILE-----\n",
		"app/binary#enc":          "age-encryption.org/v1\n",
		"app/leaked#enc":          "password=hunter2\n",
		"app/plain.conf":                "not marked, not checked\n",
		"work/leaked-at-work#enc": "token=abc\n",
	}
	for name, content := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(filepath.Join(repo, "mate.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	issues := checkEncrypted(cfg)
	_ = w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if issues != 2 {
		t.Errorf("expected 2 issues, got %d:\n%s", issues, out)
	}
	for _, want := range []string{"leaked#enc is marked", "leaked-at-work#enc is marked"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"armored", "binary", "plain.conf"} {
		if strings.Contains(string(out), unwanted) {
			t.Errorf("did not expect %q in output:\n%s", unwanted, out)
		}
	}
}
