package template

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/subbeh/statemate/internal/config"
)

func TestNewContext(t *testing.T) {
	cfg := &config.Config{
		Variables: map[string]any{
			"email": "test@example.com",
		},
	}

	ctx, err := NewContext(cfg, "")
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.OS != runtime.GOOS {
		t.Errorf("expected OS=%s, got %s", runtime.GOOS, ctx.OS)
	}

	if ctx.Arch != runtime.GOARCH {
		t.Errorf("expected Arch=%s, got %s", runtime.GOARCH, ctx.Arch)
	}

	if ctx.Vars["email"] != "test@example.com" {
		t.Errorf("expected email from vars, got %v", ctx.Vars["email"])
	}

	if ctx.Env["HOME"] == "" && ctx.Env["USERPROFILE"] == "" {
		t.Error("expected HOME or USERPROFILE in env")
	}
}

func TestContextWithProfile(t *testing.T) {
	cfg := &config.Config{
		Variables: map[string]any{
			"editor": "vim",
		},
		Profiles: map[string]*config.Profile{
			"work": {
				Variables: map[string]any{
					"editor": "nvim",
					"proxy":  "http://proxy:8080",
				},
			},
		},
	}

	ctx, err := NewContext(cfg, "work")
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.Vars["editor"] != "nvim" {
		t.Errorf("expected editor=nvim (from profile), got %v", ctx.Vars["editor"])
	}

	if ctx.Vars["proxy"] != "http://proxy:8080" {
		t.Errorf("expected proxy from profile, got %v", ctx.Vars["proxy"])
	}
}

func TestContextVarCommands(t *testing.T) {
	cfg := &config.Config{
		VarCmds: map[string]string{
			"test_var": "echo hello",
		},
	}

	ctx, err := NewContext(cfg, "")
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.Vars["test_var"] != "hello" {
		t.Errorf("expected test_var=hello, got %v", ctx.Vars["test_var"])
	}
}

func TestRenderBasic(t *testing.T) {
	cfg := &config.Config{
		Variables: map[string]any{
			"name": "John",
		},
	}

	ctx, _ := NewContext(cfg, "")

	content := []byte("Hello, {{ .Vars.name }}!")
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "Hello, John!" {
		t.Errorf("expected 'Hello, John!', got %q", string(result))
	}
}

func TestRenderWithBuiltins(t *testing.T) {
	cfg := &config.Config{}
	ctx, _ := NewContext(cfg, "")

	content := []byte("OS: {{ .OS }}, Arch: {{ .Arch }}")
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "OS: " + runtime.GOOS + ", Arch: " + runtime.GOARCH
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestRenderWithConditional(t *testing.T) {
	cfg := &config.Config{}
	ctx, _ := NewContext(cfg, "work")

	content := []byte(`{{ if eq .Profile "work" }}WORK{{ else }}PERSONAL{{ end }}`)
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "WORK" {
		t.Errorf("expected 'WORK', got %q", string(result))
	}
}

func TestRenderWithEnv(t *testing.T) {
	_ = os.Setenv("TEST_TEMPLATE_VAR", "test-value")
	defer func() { _ = os.Unsetenv("TEST_TEMPLATE_VAR") }()

	cfg := &config.Config{}
	ctx, _ := NewContext(cfg, "")

	content := []byte(`Env: {{ env "TEST_TEMPLATE_VAR" }}`)
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "Env: test-value" {
		t.Errorf("expected 'Env: test-value', got %q", string(result))
	}
}

func TestRenderWithCmd(t *testing.T) {
	cfg := &config.Config{}
	ctx, _ := NewContext(cfg, "")

	content := []byte(`Result: {{ cmd "echo hello" }}`)
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "Result: hello" {
		t.Errorf("expected 'Result: hello', got %q", string(result))
	}
}

func TestRenderWithDefault(t *testing.T) {
	cfg := &config.Config{
		Variables: map[string]any{
			"set_var": "value",
		},
	}
	ctx, _ := NewContext(cfg, "")

	content := []byte(`{{ default "fallback" .Vars.set_var }},{{ default "fallback" .Vars.unset_var }}`)
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "value,fallback" {
		t.Errorf("expected 'value,fallback', got %q", string(result))
	}
}

// Sprig provides the bulk of the function library. This is the exact expression
// that failed in a real config: splitList and first are sprig functions, and
// text/template has no string-splitting function of its own.
func TestRenderWithSprigFuncs(t *testing.T) {
	ctx, _ := NewContext(&config.Config{
		Variables: map[string]any{"user": "u123456-sub1"},
	}, "")

	content := []byte(`{{ (splitList "-" .Vars.user) | first }}.your-storagebox.de`)
	result, err := Render(content, ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "u123456.your-storagebox.de" {
		t.Errorf("got %q", string(result))
	}
}

// Statemate's functions must override sprig's on a name clash, or existing
// templates break. env is the one that matters: sprig's reads the live process
// environment, statemate's reads the rendering context.
func TestMateFuncsOverrideSprig(t *testing.T) {
	ctx, _ := NewContext(&config.Config{}, "")
	ctx.Env = map[string]string{"FROM_CONTEXT": "context-value"}

	result, err := Render([]byte(`{{ env "FROM_CONTEXT" }}`), ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if string(result) != "context-value" {
		t.Errorf("statemate's env must win over sprig's, got %q", string(result))
	}

	// indent takes (spaces, string) here; sprig's has the same shape, but only
	// statemate's leaves blank lines unpadded (sprig gives "  a\n  \n  b").
	result, err = Render([]byte(`{{ indent 2 "a\n\nb" }}`), ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if string(result) != "  a\n\n  b" {
		t.Errorf("statemate's indent must win over sprig's, got %q", string(result))
	}

	// Statemate's default substitutes only for nil and ""; sprig's also replaces
	// 0 and false, which would silently rewrite a legitimate zero in a config.
	result, err = Render([]byte(`{{ default "fb" 0 }}`), ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if string(result) != "0" {
		t.Errorf("default must leave 0 alone, got %q", string(result))
	}
}

// required is statemate's own (sprig has no such function), but sprig's library
// is large enough that a future version could add one -- this pins the signature.
func TestRenderRequired(t *testing.T) {
	ctx, _ := NewContext(&config.Config{}, "")

	if _, err := Render([]byte(`{{ required .Vars.missing }}`), ctx); err == nil {
		t.Error("expected an error for a missing required value")
	}
	if _, err := Render([]byte(`{{ required "present" }}`), ctx); err != nil {
		t.Errorf("unexpected error for a present value: %v", err)
	}
}

func TestLoadVarFile(t *testing.T) {
	dir := t.TempDir()
	varFile := dir + "/vars.yaml"

	content := `
email: from-file@example.com
nested:
  key: value
`
	if err := os.WriteFile(varFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		VarFiles: []string{varFile},
	}

	ctx, err := NewContext(cfg, "")
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.Vars["email"] != "from-file@example.com" {
		t.Errorf("expected email from var file, got %v", ctx.Vars["email"])
	}

	nested, ok := ctx.Vars["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", ctx.Vars["nested"])
	}
	if nested["key"] != "value" {
		t.Errorf("expected nested.key=value, got %v", nested["key"])
	}
}

func TestRenderFile(t *testing.T) {
	dir := t.TempDir()
	tmplFile := dir + "/config.tmpl"

	content := `# Config for {{ .Username }}
editor = "{{ .Vars.editor }}"
`
	if err := os.WriteFile(tmplFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Variables: map[string]any{
			"editor": "nvim",
		},
	}

	ctx, _ := NewContext(cfg, "")

	result, err := RenderFile(tmplFile, ctx)
	if err != nil {
		t.Fatalf("RenderFile failed: %v", err)
	}

	if !strings.Contains(string(result), `editor = "nvim"`) {
		t.Errorf("expected rendered content, got %q", string(result))
	}
}
