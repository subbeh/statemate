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

func TestLoadHooks(t *testing.T) {
	dir := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repo := `
hooks:
  systemd:
    match: "*.service"
    do:
      - run: sudo systemctl daemon-reload
  tmux:
    match: [.config/tmux/*.conf, .tmux.conf]
    do:
      - run: tmux source-file ~/.tmux.conf
      - script: notify.sh
  replaced:
    match: "*"
    description: from the repo
    do:
      - run: "true"
`
	local := `
hooks:
  systemd:
    enabled: false
  replaced:
    match: /etc/*
    do:
      - run: echo local
`
	if err := os.WriteFile(filepath.Join(dir, "mate.yaml"), []byte(repo), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(xdg, "statemate"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "statemate", "mate.yaml"), []byte(local), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "mate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if h := cfg.Hooks["systemd"]; h.IsEnabled() || !h.IsLocal() {
		t.Errorf("local enabled: false should disable the repo hook, got %+v", h)
	}
	tmux := cfg.Hooks["tmux"]
	if len(tmux.Match) != 2 || len(tmux.Do) != 2 || tmux.Do[1].Script != "notify.sh" || tmux.Dir() != dir {
		t.Errorf("tmux hook parsed wrong: %+v", tmux)
	}
	// A local override replaces the whole hook rather than merging fields.
	if r := cfg.Hooks["replaced"]; r.Description != "" || r.Match[0] != "/etc/*" || r.Dir() != filepath.Join(xdg, "statemate") {
		t.Errorf("local override should replace the repo hook, got %+v", r)
	}
}

func TestLoadHooksTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	content := `
[hooks.single]
match = "*.service"
[[hooks.single.do]]
run = "true"

[hooks.list]
match = ["a", "b"]
[[hooks.list.do]]
script = "x.sh"
`
	if err := os.WriteFile(filepath.Join(dir, "mate.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(filepath.Join(dir, "mate.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if m := cfg.Hooks["single"].Match; len(m) != 1 || m[0] != "*.service" {
		t.Errorf("single match = %v", m)
	}
	if m := cfg.Hooks["list"].Match; len(m) != 2 || cfg.Hooks["list"].Do[0].Script != "x.sh" {
		t.Errorf("list hook = %+v", cfg.Hooks["list"])
	}
}

func TestValidateHooks(t *testing.T) {
	cfg := &Config{Hooks: map[string]*Hook{"h": {Match: StringList{"*"}}}}
	if err := cfg.Validate(); err == nil {
		t.Error("a hook with no steps should fail validation")
	}
}
