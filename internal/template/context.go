package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

type SecretLookup func(item, typ, field string) (string, error)

type DecryptFunc func(ciphertext []byte) ([]byte, error)

type Context struct {
	Profile      string
	Hostname     string
	OS           string
	Arch         string
	HomeDir      string
	Username     string
	SourceDir    string
	Vars         map[string]any
	Env          map[string]string
	SecretLookup SecretLookup
	Decrypt      DecryptFunc

	// Files lists the target paths that triggered a hook. Set only while
	// rendering a hook's run step.
	Files []string
}

type ContextOption func(*Context)

func WithDecrypt(fn DecryptFunc) ContextOption {
	return func(c *Context) {
		c.Decrypt = fn
	}
}

func NewContext(cfg *config.Config, profileName string, opts ...ContextOption) (*Context, error) {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	ctx := &Context{
		Profile:   profileName,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		HomeDir:   home,
		Username:  username,
		SourceDir: cfg.SourceDir(),
		Vars:      make(map[string]any),
		Env:       make(map[string]string),
	}

	for _, opt := range opts {
		opt(ctx)
	}

	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			ctx.Env[parts[0]] = parts[1]
		}
	}

	for k, v := range cfg.Variables {
		ctx.Vars[k] = v
	}

	for _, varFile := range cfg.VarFiles {
		if err := ctx.loadVarFile(varFile); err != nil {
			return nil, err
		}
	}

	for name, cmd := range cfg.VarCmds {
		val, err := runCommand(cmd)
		if err != nil {
			return nil, err
		}
		ctx.Vars[name] = strings.TrimSpace(val)
	}

	if profileName != "" {
		resolved := profile.Resolve(cfg, profileName)
		if resolved != nil {
			for k, v := range resolved.Variables {
				ctx.Vars[k] = v
			}
		}
	}

	// A source's .mate.yaml must be rendered to read its include, and its template
	// may use what that include provides, so this first render leaves those empty.
	// Secrets aren't wired up yet and stay empty too. A file that fails here is
	// reported by the scanner, which renders it again.
	ctx.SecretLookup = func(_, _, _ string) (string, error) { return "", nil }
	render := func(data []byte) ([]byte, error) { return Render(data, ctx) }
	for _, dir := range cfg.ResolveSourcePaths(profile.ResolveSources(cfg, profileName)) {
		if dirCfg, err := config.LoadDirConfigRaw(dir, render); err == nil && dirCfg != nil {
			for k, v := range dirCfg.Variables {
				ctx.Vars[k] = v
			}
		}
	}
	ctx.SecretLookup = nil

	return ctx, nil
}

func (c *Context) loadVarFile(path string) error {
	if strings.HasPrefix(path, "~/") {
		path = c.HomeDir + path[1:]
	} else if !strings.HasPrefix(path, "/") && c.SourceDir != "" {
		path = c.SourceDir + "/" + path
	}

	encrypted := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try the #enc variant
		if path = util.EncryptedVariant(path); path == "" {
			return nil
		}
		encrypted = true
	} else if _, attrs := source.ParseAttrs(filepath.Base(path)); attrs.Encrypted {
		// #enc may be any of the attributes, not only the last.
		encrypted = true
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if encrypted {
		if c.Decrypt == nil {
			return fmt.Errorf("var_file %s is encrypted but no age identity is configured", path)
		}
		data, err = c.Decrypt(data)
		if err != nil {
			return fmt.Errorf("decrypting var_file %s: %w", path, err)
		}
		// Strip the attributes for format detection
		baseName, _ := source.ParseAttrs(filepath.Base(path))
		path = filepath.Join(filepath.Dir(path), baseName)
	}

	vars, err := parseVarFile(data, path)
	if err != nil {
		return err
	}

	for k, v := range vars {
		c.Vars[k] = v
	}

	return nil
}

func parseVarFile(data []byte, path string) (map[string]any, error) {
	vars := make(map[string]any)

	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return parseYAML(data)
	}
	if strings.HasSuffix(path, ".toml") {
		return parseTOML(data)
	}

	return vars, nil
}

func runCommand(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (c *Context) Cmd(cmd string) string {
	out, err := runCommand(cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
