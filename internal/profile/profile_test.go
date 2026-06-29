package profile

import (
	"os"
	"runtime"
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

func TestDetectFromEnv(t *testing.T) {
	os.Setenv("STATEMATE_PROFILE", "test-profile")
	defer os.Unsetenv("STATEMATE_PROFILE")

	cfg := &config.Config{}
	got := Detect(cfg)
	if got != "test-profile" {
		t.Errorf("expected test-profile, got %q", got)
	}
}

func TestDetectFromConfig(t *testing.T) {
	cfg := &config.Config{
		Profile: "configured-profile",
	}
	got := Detect(cfg)
	if got != "configured-profile" {
		t.Errorf("expected configured-profile, got %q", got)
	}
}

func TestDetectByOS(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"mac": {
				Detection: &config.Detection{
					OS: "darwin",
				},
			},
			"linux": {
				Detection: &config.Detection{
					OS: "linux",
				},
			},
		},
	}

	got := Detect(cfg)
	if runtime.GOOS == "darwin" && got != "mac" {
		t.Errorf("on darwin, expected mac, got %q", got)
	}
	if runtime.GOOS == "linux" && got != "linux" {
		t.Errorf("on linux, expected linux, got %q", got)
	}
}

func TestDetectByHostname(t *testing.T) {
	hostname, _ := os.Hostname()

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"current": {
				Detection: &config.Detection{
					Hostname: hostname,
				},
			},
			"other": {
				Detection: &config.Detection{
					Hostname: "nonexistent-host-12345",
				},
			},
		},
	}

	got := Detect(cfg)
	if got != "current" {
		t.Errorf("expected current, got %q", got)
	}
}

func TestDetectByHostnamePattern(t *testing.T) {
	hostname, _ := os.Hostname()
	if len(hostname) < 2 {
		t.Skip("hostname too short for pattern test")
	}

	pattern := hostname[:2] + "*"

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"matched": {
				Detection: &config.Detection{
					Hostname: pattern,
				},
			},
		},
	}

	got := Detect(cfg)
	if got != "matched" {
		t.Errorf("expected matched, got %q", got)
	}
}

func TestDetectModeAnd(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"specific": {
				Detection: &config.Detection{
					Mode: "and",
					OS:   runtime.GOOS,
					Arch: "nonexistent-arch",
				},
			},
		},
	}

	got := Detect(cfg)
	if got != "" {
		t.Errorf("expected no match with AND mode, got %q", got)
	}
}

func TestResolveInheritance(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"base": {
				Packages: &config.PackageList{
					Brew: []string{"git", "neovim"},
				},
				Variables: map[string]any{
					"editor": "vim",
				},
			},
			"work": {
				Extends: "base",
				Packages: &config.PackageList{
					Brew: []string{"slack"},
				},
				Variables: map[string]any{
					"editor": "nvim",
					"proxy":  "http://proxy:8080",
				},
			},
		},
	}

	resolved := Resolve(cfg, "work")
	if resolved == nil {
		t.Fatal("Resolve returned nil")
	}

	brew := resolved.Packages.Brew
	if len(brew) != 3 {
		t.Errorf("expected 3 brew packages, got %d: %v", len(brew), brew)
	}

	has := func(pkg string) bool {
		for _, p := range brew {
			if p == pkg {
				return true
			}
		}
		return false
	}

	if !has("git") || !has("neovim") || !has("slack") {
		t.Errorf("missing expected packages: %v", brew)
	}

	if resolved.Variables["editor"] != "nvim" {
		t.Errorf("expected editor=nvim (child override), got %v", resolved.Variables["editor"])
	}

	if resolved.Variables["proxy"] != "http://proxy:8080" {
		t.Errorf("expected proxy from work profile, got %v", resolved.Variables["proxy"])
	}
}

func TestResolveChainedInheritance(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"base": {
				Packages: &config.PackageList{
					Brew: []string{"git"},
				},
			},
			"dev": {
				Extends: "base",
				Packages: &config.PackageList{
					Brew: []string{"neovim"},
				},
			},
			"work-dev": {
				Extends: "dev",
				Packages: &config.PackageList{
					Brew: []string{"slack"},
				},
			},
		},
	}

	resolved := Resolve(cfg, "work-dev")
	if resolved == nil {
		t.Fatal("Resolve returned nil")
	}

	brew := resolved.Packages.Brew
	if len(brew) != 3 {
		t.Errorf("expected 3 brew packages from chain, got %d: %v", len(brew), brew)
	}
}

func TestResolveNonexistent(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{},
	}

	resolved := Resolve(cfg, "nonexistent")
	if resolved != nil {
		t.Error("expected nil for nonexistent profile")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"exact", "exact", true},
		{"exact", "different", false},
		{"work-*", "work-laptop", true},
		{"work-*", "personal-laptop", false},
		{"*-laptop", "work-laptop", true},
		{"*", "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}
