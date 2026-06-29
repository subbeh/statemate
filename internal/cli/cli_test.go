package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	cwd, _ := os.Getwd()
	os.MkdirAll(filepath.Join(tmpDir, ".config", "statemate"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".config", "statemate", "mate.yaml"), []byte("source_dir: "+cwd+"\n"), 0644)

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
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	cwd, _ := os.Getwd()
	os.MkdirAll(filepath.Join(tmpDir, ".config", "statemate"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".config", "statemate", "mate.yaml"), []byte("source_dir: "+cwd+"\n"), 0644)

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
