package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAttrs(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantAttr Attrs
	}{
		{
			input:    "config.yaml",
			wantName: "config.yaml",
			wantAttr: Attrs{},
		},
		{
			input:    "config.yaml#profile:work",
			wantName: "config.yaml",
			wantAttr: Attrs{Profile: "work"},
		},
		{
			input:    "config.yaml#perm:600",
			wantName: "config.yaml",
			wantAttr: Attrs{Perm: 0600},
		},
		{
			input:    "config.yaml#encrypted",
			wantName: "config.yaml",
			wantAttr: Attrs{Encrypted: true},
		},
		{
			input:    "config.yaml#template",
			wantName: "config.yaml",
			wantAttr: Attrs{Template: true},
		},
		{
			input:    "config.yaml#profile:work#perm:600#encrypted#template",
			wantName: "config.yaml",
			wantAttr: Attrs{Profile: "work", Perm: 0600, Encrypted: true, Template: true},
		},
		{
			input:    "config.yaml#owner:root#group:wheel",
			wantName: "config.yaml",
			wantAttr: Attrs{Owner: "root", Group: "wheel"},
		},
		{
			input:    "link#symlink",
			wantName: "link",
			wantAttr: Attrs{Symlink: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, attrs := ParseAttrs(tt.input)
			if name != tt.wantName {
				t.Errorf("name: got %q, want %q", name, tt.wantName)
			}
			if attrs != tt.wantAttr {
				t.Errorf("attrs: got %+v, want %+v", attrs, tt.wantAttr)
			}
		})
	}
}

func TestAttrsMerge(t *testing.T) {
	parent := Attrs{Profile: "work", Perm: 0755}
	child := Attrs{Perm: 0600}

	child.Merge(parent)

	if child.Profile != "work" {
		t.Errorf("expected Profile=work, got %q", child.Profile)
	}
	if child.Perm != 0600 {
		t.Errorf("expected Perm=0600, got %o", child.Perm)
	}
}

func TestScannerBasic(t *testing.T) {
	dir := t.TempDir()

	nvimDir := filepath.Join(dir, "nvim", ".config", "nvim")
	if err := os.MkdirAll(nvimDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- nvim config"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "nvim")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Name != "init.lua" {
		t.Errorf("expected name=init.lua, got %q", files[0].Name)
	}
	if files[0].TargetPath != "/home/testuser/.config/nvim/init.lua" {
		t.Errorf("expected target=/home/testuser/.config/nvim/init.lua, got %q", files[0].TargetPath)
	}
}

func TestScannerWithProfile(t *testing.T) {
	dir := t.TempDir()

	configDir := filepath.Join(dir, "app", ".config", "app")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("default: true"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml#profile:work"), []byte("work: true"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "app")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(tree.Files()) != 2 {
		t.Fatalf("expected 2 files, got %d", len(tree.Files()))
	}

	filtered := tree.FilterByProfile([]string{"work"})
	files := filtered.Files()

	var hasDefault, hasWork bool
	for _, f := range files {
		if f.Attrs.Profile == "" {
			hasDefault = true
		}
		if f.Attrs.Profile == "work" {
			hasWork = true
		}
	}

	if !hasDefault || !hasWork {
		t.Error("expected both default and work profile files after filter")
	}
}

func TestScannerConflictDetection(t *testing.T) {
	dir := t.TempDir()

	app1Dir := filepath.Join(dir, "app1", ".config", "app")
	app2Dir := filepath.Join(dir, "app2", ".config", "app")

	for _, d := range []string{app1Dir, app2Dir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "config.yaml"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{
		filepath.Join(dir, "app1"),
		filepath.Join(dir, "app2"),
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if !tree.HasConflicts() {
		t.Error("expected conflict to be detected")
	}

	if len(tree.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(tree.Conflicts))
	}

	if tree.Conflicts[0].TargetPath != "/home/testuser/.config/app/config.yaml" {
		t.Errorf("unexpected conflict target: %s", tree.Conflicts[0].TargetPath)
	}
}

func TestScannerWithDirConfig(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "app", "etc", "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "config.yaml"), []byte("system: true"), 0644); err != nil {
		t.Fatal(err)
	}

	dirCfgContent := `
targets:
  etc: /etc
`
	if err := os.WriteFile(filepath.Join(dir, "app", ".mate.yaml"), []byte(dirCfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "app")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].TargetPath != "/etc/app/config.yaml" {
		t.Errorf("expected target=/etc/app/config.yaml, got %q", files[0].TargetPath)
	}
}

func TestScannerDirConfigIgnore(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "app", ".config", "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "config.yaml"), []byte("app"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, ".stylua.toml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	dirCfgContent := `
ignore:
  - .stylua.toml
`
	if err := os.WriteFile(filepath.Join(dir, "app", ".mate.yaml"), []byte(dirCfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "app")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file (config.yaml), got %d", len(files))
	}
	if files[0].Name != "config.yaml" {
		t.Errorf("expected config.yaml, got %q", files[0].Name)
	}
}

func TestScannerDirConfigIgnoreScopedToSource(t *testing.T) {
	dir := t.TempDir()

	// Source "a" ignores README.md; source "b" does not.
	for _, src := range []string{"a", "b"} {
		srcDir := filepath.Join(dir, src)
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	dirCfgContent := `
ignore:
  - README.md
`
	if err := os.WriteFile(filepath.Join(dir, "a", ".mate.yaml"), []byte(dirCfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "a"), filepath.Join(dir, "b")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file (b/README.md), got %d", len(files))
	}
	if !filepath.IsAbs(files[0].SourcePath) || filepath.Base(filepath.Dir(files[0].SourcePath)) != "b" {
		t.Errorf("expected the surviving file to come from source b, got %q", files[0].SourcePath)
	}
}

func TestScannerPermRInheritance(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "app", ".local", "bin#perm-r:755")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "script.sh"), []byte("#!/bin/sh"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "app")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Attrs.Perm != 0755 {
		t.Errorf("expected perm 0755 from perm-r inheritance, got %#o", files[0].Attrs.Perm)
	}
	if files[0].TargetPath != "/home/testuser/.local/bin/script.sh" {
		t.Errorf("expected target /home/testuser/.local/bin/script.sh, got %q", files[0].TargetPath)
	}
}

func TestScannerSkipsSpecialDirs(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "app", ".config", "app")
	gitDir := filepath.Join(dir, "app", ".git")
	scriptsDir := filepath.Join(dir, "app", ".matescripts")

	for _, d := range []string{appDir, gitDir, scriptsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(appDir, "config.yaml"), []byte("app"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "run_once_01-setup.sh"), []byte("#!/bin/bash"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "app")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file (only config.yaml), got %d", len(files))
	}

	if files[0].Name != "config.yaml" {
		t.Errorf("expected config.yaml, got %q", files[0].Name)
	}
}

func TestScannerProfileInheritance(t *testing.T) {
	dir := t.TempDir()

	workDir := filepath.Join(dir, "nvim#profile:work", ".config", "nvim")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "init.lua"), []byte("-- work config"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("/home/testuser", "")
	tree, err := scanner.Scan([]string{filepath.Join(dir, "nvim#profile:work")})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	files := tree.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Attrs.Profile != "work" {
		t.Errorf("expected profile=work (inherited), got %q", files[0].Attrs.Profile)
	}
}
