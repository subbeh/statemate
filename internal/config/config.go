package config

import "path/filepath"

type Config struct {
	SourceDirPath string              `yaml:"source_dir" toml:"source_dir"`
	Sources       []string            `yaml:"sources" toml:"sources"`
	DefaultSource string              `yaml:"default_source" toml:"default_source"`
	TargetBase    string              `yaml:"target_base" toml:"target_base"`
	Profiles      map[string]*Profile `yaml:"profiles" toml:"profiles"`
	Profile       string              `yaml:"profile" toml:"profile"`
	Editor        string              `yaml:"editor" toml:"editor"`
	Age           *AgeConfig          `yaml:"age" toml:"age"`
	Variables     map[string]any      `yaml:"variables" toml:"variables"`
	VarFiles      []string            `yaml:"var_files" toml:"var_files"`
	Packages     PackageList  `yaml:"packages" toml:"packages"`
	Include      []string     `yaml:"include" toml:"include"`
	Ignore       []string     `yaml:"ignore" toml:"ignore"`
	AURHelper    string       `yaml:"aur_helper" toml:"aur_helper"`
	SecretsCache string       `yaml:"secrets_cache" toml:"secrets_cache"`
	DiffTool     string       `yaml:"diff_tool" toml:"diff_tool"`
	Hooks        map[string]*Hook `yaml:"hooks" toml:"hooks"`
	LogFile      string       `yaml:"log_file" toml:"log_file"`
	LogLevel     string       `yaml:"log_level" toml:"log_level"`

	sourceDir string
}

func (c *Config) ApplyOverrides(override *Config) {
	if override == nil {
		return
	}
	if len(override.Sources) > 0 {
		c.Sources = override.Sources
	}
	if override.DefaultSource != "" {
		c.DefaultSource = override.DefaultSource
	}
	if override.TargetBase != "" {
		c.TargetBase = override.TargetBase
	}
	if override.Profile != "" {
		c.Profile = override.Profile
	}
	if override.Editor != "" {
		c.Editor = override.Editor
	}
	if override.Age != nil {
		c.Age = override.Age
	}
	if override.Variables != nil {
		c.Variables = override.Variables
	}
	if len(override.VarFiles) > 0 {
		c.VarFiles = override.VarFiles
	}
	if override.Packages != nil {
		c.Packages = override.Packages
	}
	if override.Profiles != nil {
		c.Profiles = override.Profiles
	}
	if override.DiffTool != "" {
		c.DiffTool = override.DiffTool
	}
	if override.LogFile != "" {
		c.LogFile = override.LogFile
	}
	if override.LogLevel != "" {
		c.LogLevel = override.LogLevel
	}
	c.Hooks = mergeHooks(c.Hooks, override.Hooks)
}

func (c *Config) SourceDir() string {
	return c.sourceDir
}

func (c *Config) AbsoluteSources() []string {
	return c.ResolveSourcePaths(c.Sources)
}

func (c *Config) ResolveSourcePaths(sources []string) []string {
	result := make([]string, len(sources))
	for i, s := range sources {
		if filepath.IsAbs(s) {
			result[i] = s
		} else {
			result[i] = filepath.Join(c.sourceDir, s)
		}
	}
	return result
}

type Profile struct {
	Extends   string         `yaml:"extends" toml:"extends"`
	Sources   []string       `yaml:"sources" toml:"sources"`
	Detection *Detection     `yaml:"detection" toml:"detection"`
	Packages  PackageList    `yaml:"packages" toml:"packages"`
	Include   []string       `yaml:"include" toml:"include"`
	Variables map[string]any `yaml:"variables" toml:"variables"`
}

type Detection struct {
	Mode     string `yaml:"mode" toml:"mode"`
	Hostname any    `yaml:"hostname" toml:"hostname"`
	User     any    `yaml:"user" toml:"user"`
	OS       string `yaml:"os" toml:"os"`
	Arch     string `yaml:"arch" toml:"arch"`
	Command  string `yaml:"command" toml:"command"`
}

type AgeConfig struct {
	Identity        string   `yaml:"identity" toml:"identity"`
	IdentityCommand string   `yaml:"identity_command" toml:"identity_command"`
	Recipients      []string `yaml:"recipients" toml:"recipients"`
}

type DirConfig struct {
	TargetBase string                `yaml:"target_base" toml:"target_base"`
	Profile    string                `yaml:"profile" toml:"profile"`
	Targets    map[string]StringList `yaml:"targets" toml:"targets"`
	Ignore     []string              `yaml:"ignore" toml:"ignore"`
	Owner      string                `yaml:"owner" toml:"owner"`
	Group      string                `yaml:"group" toml:"group"`
	Perm       string                `yaml:"perm" toml:"perm"`
	Packages   PackageList           `yaml:"packages" toml:"packages"`
	Generate   []GenerateConfig      `yaml:"generate" toml:"generate"`
	Hooks      map[string]*Hook      `yaml:"hooks" toml:"hooks"`
	Include    []string              `yaml:"include" toml:"include"`

	// Variables holds what Include loaded; .mate.yaml cannot set them itself.
	Variables map[string]any `yaml:"-" toml:"-"`
}

type GenerateConfig struct {
	Target  string `yaml:"target" toml:"target"`
	Mode    string `yaml:"mode" toml:"mode"`
	Content string `yaml:"content" toml:"content"`
	Profile string `yaml:"profile" toml:"profile"`
}
