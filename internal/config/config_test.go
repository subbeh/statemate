package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "nvim"), 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
sources:
  - nvim

target_base: "~"

profiles:
  base:
    packages:
      brew:
        - git
        - neovim
  work:
    extends: base
    detection:
      mode: and
      hostname: "work-*"
      os: darwin

variables:
  email: test@example.com

packages:
  brew:
    - ripgrep
`
	if err := os.WriteFile(filepath.Join(dir, "mate.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "mate.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sources) != 1 || cfg.Sources[0] != "nvim" {
		t.Errorf("expected sources=[nvim], got %v", cfg.Sources)
	}

	if cfg.Profiles["work"].Extends != "base" {
		t.Errorf("expected work extends base, got %q", cfg.Profiles["work"].Extends)
	}

	if cfg.Profiles["work"].Detection.Mode != "and" {
		t.Errorf("expected detection mode=and, got %q", cfg.Profiles["work"].Detection.Mode)
	}

	if cfg.Variables["email"] != "test@example.com" {
		t.Errorf("expected email=test@example.com, got %v", cfg.Variables["email"])
	}

	if len(cfg.Packages.Brew) != 1 || cfg.Packages.Brew[0] != "ripgrep" {
		t.Errorf("expected packages.brew=[ripgrep], got %v", cfg.Packages.Brew)
	}
}

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "zsh"), 0755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
sources = ["zsh"]
target_base = "~"

[variables]
editor = "nvim"

[packages]
brew = ["fd", "ripgrep"]
`
	if err := os.WriteFile(filepath.Join(dir, "mate.toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "mate.toml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sources) != 1 || cfg.Sources[0] != "zsh" {
		t.Errorf("expected sources=[zsh], got %v", cfg.Sources)
	}

	if cfg.Variables["editor"] != "nvim" {
		t.Errorf("expected editor=nvim, got %v", cfg.Variables["editor"])
	}
}

func TestLoadDirConfig(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
targets:
  etc: /etc

packages:
  brew:
    - neovim
`
	if err := os.WriteFile(filepath.Join(dir, ".mate.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDirConfig(dir)
	if err != nil {
		t.Fatalf("LoadDirConfig failed: %v", err)
	}

	if cfg.Targets["etc"] != "/etc" {
		t.Errorf("expected targets[etc]=/etc, got %v", cfg.Targets["etc"])
	}

	if len(cfg.Packages.Brew) != 1 || cfg.Packages.Brew[0] != "neovim" {
		t.Errorf("expected packages.brew=[neovim], got %v", cfg.Packages.Brew)
	}
}

func TestValidateInvalidExtends(t *testing.T) {
	cfg := &Config{
		Sources: []string{"."},
		Profiles: map[string]*Profile{
			"work": {Extends: "nonexistent"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid extends")
	}
}

func TestValidateInvalidDetectionMode(t *testing.T) {
	cfg := &Config{
		Sources: []string{"."},
		Profiles: map[string]*Profile{
			"work": {
				Detection: &Detection{Mode: "invalid"},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid detection mode")
	}
}

func TestFindConfigAutodetect(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "mate.yaml"), []byte("sources: []"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := findConfigInDir(dir)
	if err != nil {
		t.Fatalf("findConfigInDir failed: %v", err)
	}

	if filepath.Base(path) != "mate.yaml" {
		t.Errorf("expected mate.yaml, got %s", path)
	}
}

func TestTargetBaseExpansion(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "app"), 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
sources:
  - app
target_base: "~"
`
	if err := os.WriteFile(filepath.Join(dir, "mate.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "mate.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	if cfg.TargetBase != home {
		t.Errorf("expected target_base=%s, got %s", home, cfg.TargetBase)
	}
}
