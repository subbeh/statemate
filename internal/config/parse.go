package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/subbeh/statemate/internal/util"
	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = FindConfig()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := &Config{}
	cfg.sourceDir = filepath.Dir(path)

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s", filepath.Ext(path))
	}

	localCfg := loadLocalConfig()
	cfg.ApplyOverrides(localCfg)

	if err := cfg.setDefaults(); err != nil {
		return nil, err
	}

	if err := cfg.loadPackageFiles(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func FindConfig() (string, error) {
	if envDir := os.Getenv("STATEMATE_DIR"); envDir != "" {
		return findConfigInDir(expandHome(envDir))
	}

	if lc := loadLocalConfig(); lc != nil && lc.SourceDirPath != "" {
		return findConfigInDir(expandHome(lc.SourceDirPath))
	}

	return findConfigInDir(".")
}

func SourceDir() string {
	if envDir := os.Getenv("STATEMATE_DIR"); envDir != "" {
		return expandHome(envDir)
	}

	if lc := loadLocalConfig(); lc != nil && lc.SourceDirPath != "" {
		return expandHome(lc.SourceDirPath)
	}

	cwd, _ := os.Getwd()
	return cwd
}

func LocalConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "statemate", "mate.yaml")
}

func loadLocalConfig() *Config {
	data, err := os.ReadFile(LocalConfigPath())
	if err != nil {
		return nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func SaveLocalSourceDir(dir string) error {
	path := LocalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	content := fmt.Sprintf("source_dir: %q\n", util.ShortenPath(dir))
	return os.WriteFile(path, []byte(content), 0644)
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func findConfigInDir(dir string) (string, error) {
	candidates := []string{"mate.yaml", "mate.yml", "mate.toml"}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no config file found in %s (tried: %s)", dir, strings.Join(candidates, ", "))
}

func LoadDirConfig(dir string) (*DirConfig, error) {
	path, err := findDirConfig(dir)
	if err != nil {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading dir config: %w", err)
	}

	cfg := &DirConfig{}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing TOML: %w", err)
		}
	}

	return cfg, nil
}


func findDirConfig(dir string) (string, error) {
	candidates := []string{".mate.yaml", ".mate.yml", ".mate.toml"}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no dir config found")
}

func (c *Config) loadPackageFiles() error {
	// Collect all package files to load
	var files []string

	// Convention: .mate/packages.yaml (or .yml/.toml)
	conventionPaths := []string{
		filepath.Join(c.sourceDir, ".mate", "packages.yaml"),
		filepath.Join(c.sourceDir, ".mate", "packages.yml"),
		filepath.Join(c.sourceDir, ".mate", "packages.toml"),
	}
	for _, path := range conventionPaths {
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
			break
		}
	}

	// Explicit package_files
	for _, f := range c.PackageFiles {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.sourceDir, path)
		}
		path = expandHome(path)
		files = append(files, path)
	}

	// Load and merge each file
	for _, path := range files {
		pkgs, err := loadPackageFile(path)
		if err != nil {
			return fmt.Errorf("loading package file %s: %w", path, err)
		}
		c.Packages = mergePackageLists(c.Packages, pkgs)
	}

	return nil
}

func loadPackageFile(path string) (*PackageList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkgs PackageList
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &pkgs); err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, &pkgs); err != nil {
			return nil, fmt.Errorf("parsing TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", filepath.Ext(path))
	}

	return &pkgs, nil
}

func mergePackageLists(base, add *PackageList) *PackageList {
	if add == nil {
		return base
	}
	if base == nil {
		return add
	}

	result := &PackageList{
		Common: appendUniqueStrings(base.Common, add.Common...),
		Brew:   appendUniqueStrings(base.Brew, add.Brew...),
		Pacman: appendUniqueStrings(base.Pacman, add.Pacman...),
		AUR:    appendUniqueStrings(base.AUR, add.AUR...),
	}
	return result
}

func appendUniqueStrings(slice []string, items ...string) []string {
	seen := make(map[string]bool)
	for _, s := range slice {
		seen[s] = true
	}
	result := slice
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func (c *Config) setDefaults() error {
	if c.TargetBase == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		c.TargetBase = home
	} else if c.TargetBase == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		c.TargetBase = home
	} else if strings.HasPrefix(c.TargetBase, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		c.TargetBase = filepath.Join(home, c.TargetBase[2:])
	}

	if c.Variables == nil {
		c.Variables = make(map[string]any)
	}
	if c.VarCmds == nil {
		c.VarCmds = make(map[string]string)
	}

	return nil
}
