package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	origXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", origXDG) }()

	cwd, _ := os.Getwd()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".config", "statemate"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".config", "statemate", "mate.yaml"), []byte("source_dir: "+cwd+"\n"), 0644)

	initFormat = "yaml"
	defer func() { initFormat = "" }()

	err := runInit(nil, nil)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := os.Stat("mate.yaml"); err != nil {
		t.Error("mate.yaml not created")
	}

	// Running init again should succeed (handles existing repos)
	err = runInit(nil, nil)
	if err != nil {
		t.Errorf("expected no error on existing repo, got: %v", err)
	}
}

func TestInitCommandTOML(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	origXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", origXDG) }()

	cwd, _ := os.Getwd()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".config", "statemate"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".config", "statemate", "mate.yaml"), []byte("source_dir: "+cwd+"\n"), 0644)

	initFormat = "toml"
	defer func() { initFormat = "" }()

	err := runInit(nil, nil)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := os.Stat("mate.toml"); err != nil {
		t.Error("mate.toml not created")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tc := range tests {
		got := expandPath(tc.input)
		if got != tc.want {
			t.Errorf("expandPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// mate cat and mate eval decide whether to decrypt by looking at the content, so a
// header test that never matches makes cat print ciphertext and eval render it as a
// template.
func TestIsEncrypted(t *testing.T) {
	armored := "-----BEGIN AGE ENCRYPTED FILE-----\n" +
		"YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSBvR1h5V2x4eWtCTTBSSXRz\n" +
		"-----END AGE ENCRYPTED FILE-----\n"

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"armored age file", armored, true},
		{"header alone", "-----BEGIN AGE ENCRYPTED FILE-----", true},
		{"plaintext script", "#!/bin/sh\nexport FOO=bar\n", false},
		{"empty file", "", false},
		{"truncated header", "-----BEGIN AGE ENCR", false},
		{"another PEM block", "-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEncrypted([]byte(tc.content)); got != tc.want {
				t.Errorf("isEncrypted() = %v, want %v", got, tc.want)
			}
		})
	}
}
