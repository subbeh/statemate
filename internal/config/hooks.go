package config

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Hook runs commands or scripts after files matching its patterns are written or
// removed.
type Hook struct {
	Match       StringList `yaml:"match" toml:"match"`
	Do          []HookStep `yaml:"do" toml:"do"`
	Profile     string     `yaml:"profile" toml:"profile"`
	Description string     `yaml:"description" toml:"description"`
	Enabled     *bool      `yaml:"enabled" toml:"enabled"`

	// dir is the directory of the config file that declared the hook: steps run
	// there, and relative paths resolve against it.
	dir string
	// local marks a hook declared in the machine-local config.
	local bool
}

// HookStep is one command or script in a hook. Exactly one field is set.
type HookStep struct {
	Run    string `yaml:"run" toml:"run"`
	Script string `yaml:"script" toml:"script"`
}

// StringList accepts either a single string or a list of strings.
type StringList []string

func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*l = StringList{node.Value}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*l = list
	return nil
}

func (l *StringList) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case string:
		*l = StringList{v}
	case []any:
		list := make(StringList, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected a string, got %T", item)
			}
			list = append(list, s)
		}
		*l = list
	default:
		return fmt.Errorf("expected a string or a list of strings, got %T", data)
	}
	return nil
}

// Dir returns the directory of the config file that declared the hook.
func (h *Hook) Dir() string { return h.dir }

// SetDir records the directory of the config file that declared the hook.
func (h *Hook) SetDir(dir string) { h.dir = dir }

// IsLocal reports whether the hook came from the machine-local config.
func (h *Hook) IsLocal() bool { return h.local }

// IsEnabled reports whether the hook may run. Hooks are enabled unless they say
// otherwise.
func (h *Hook) IsEnabled() bool { return h.Enabled == nil || *h.Enabled }

// Validate checks the hook's structure. A disabled hook needs nothing else, so
// that a local override can switch a hook off with `enabled: false` alone.
func (h *Hook) Validate() error {
	if !h.IsEnabled() {
		return nil
	}
	if len(h.Match) == 0 {
		return fmt.Errorf("match is required")
	}
	for _, p := range h.Match {
		if err := validatePattern(p); err != nil {
			return fmt.Errorf("invalid match pattern %q: %w", p, err)
		}
	}
	if len(h.Do) == 0 {
		return fmt.Errorf("do needs at least one run or script step")
	}
	for i, step := range h.Do {
		if (step.Run == "") == (step.Script == "") {
			return fmt.Errorf("step %d must set exactly one of run or script", i+1)
		}
	}
	return nil
}

func validatePattern(p string) error {
	if strings.TrimSpace(p) == "" {
		return fmt.Errorf("pattern is empty")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return err
		}
	}
	return nil
}

// ValidateHooks validates every hook in a set, naming the offending hook.
func ValidateHooks(hooks map[string]*Hook) error {
	for name, h := range hooks {
		if h == nil {
			return fmt.Errorf("hook %q has no settings (use `enabled: false` to disable it)", name)
		}
		if err := h.Validate(); err != nil {
			return fmt.Errorf("hook %q: %w", name, err)
		}
	}
	return nil
}

// mergeHooks overlays hooks by name: an override replaces the whole hook rather
// than merging its fields, so what the local config says is exactly what runs.
func mergeHooks(base, override map[string]*Hook) map[string]*Hook {
	if len(override) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]*Hook, len(override))
	}
	for name, h := range override {
		base[name] = h
	}
	return base
}

func setHookOrigin(hooks map[string]*Hook, dir string, local bool) {
	for _, h := range hooks {
		if h == nil {
			continue // rejected by ValidateHooks
		}
		h.dir = dir
		h.local = local
	}
}
