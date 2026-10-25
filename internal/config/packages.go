package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageManagers are the manager names a package may list. The packages package
// implements them; they are named here so a typo fails while parsing.
var PackageManagers = []string{"brew", "pacman", "aur"}

// PackageList declares packages, one per item: a bare name for the system
// manager (brew, else pacman), or a name with exactly the managers it is
// installed with.
type PackageList []Package

type Package struct {
	Name string
	// Managers maps each manager to the name the package has there. Empty means
	// the system manager, under Name.
	Managers map[string]string
}

var errOldPackages = errors.New(`packages is a list now, one package per item: "- ripgrep", "- gnu-sed: brew" or "- gh: [brew, pacman: github-cli]" (the brew:, pacman:, aur: and common: lists are gone; see docs/packages.md)`)

func (l *PackageList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		return errOldPackages
	case yaml.SequenceNode:
	default:
		if node.Tag == "!!null" {
			return nil
		}
		return errors.New("packages must be a list")
	}

	list := PackageList{}
	for _, item := range node.Content {
		var v any
		if err := item.Decode(&v); err != nil {
			return err
		}
		p, err := parsePackage(v)
		if err != nil {
			return fmt.Errorf("line %d: %w", item.Line, err)
		}
		list = append(list, p)
	}
	*l = list
	return nil
}

func (l *PackageList) UnmarshalTOML(data any) error {
	items, ok := data.([]any)
	if !ok {
		if _, isTable := data.(map[string]any); isTable {
			return errOldPackages
		}
		return errors.New("packages must be a list")
	}

	list := PackageList{}
	for _, item := range items {
		p, err := parsePackage(item)
		if err != nil {
			return err
		}
		list = append(list, p)
	}
	*l = list
	return nil
}

// parsePackage reads one item, decoded the same way from YAML and TOML.
func parsePackage(item any) (Package, error) {
	switch item := item.(type) {
	case string:
		if item == "" {
			return Package{}, errors.New("a package name cannot be empty")
		}
		return Package{Name: item}, nil
	case map[string]any:
		if len(item) != 1 {
			return Package{}, fmt.Errorf("one package per item, got %s", strings.Join(slices.Sorted(maps.Keys(item)), ", "))
		}
		for name, v := range item {
			managers, err := parseManagers(name, v)
			return Package{Name: name, Managers: managers}, err
		}
	}
	return Package{}, fmt.Errorf("%v is not a package: write a name, or a name with its managers", item)
}

// parseManagers reads what follows a package name: one manager, or a list of
// managers each optionally renamed (`[brew, pacman: github-cli]`).
func parseManagers(name string, v any) (map[string]string, error) {
	entries, ok := v.([]any)
	if !ok {
		entries = []any{v}
	}

	managers := make(map[string]string)
	for _, entry := range entries {
		manager, as := "", name
		switch entry := entry.(type) {
		case string:
			manager = entry
		case map[string]any:
			if len(entry) != 1 {
				return nil, fmt.Errorf("package %s: write one manager per entry, as in [brew, pacman: %s]", name, name)
			}
			for k, v := range entry {
				s, ok := v.(string)
				if !ok || s == "" {
					return nil, fmt.Errorf("package %s: %s must be followed by the package's name there", name, k)
				}
				manager, as = k, s
			}
		default:
			return nil, fmt.Errorf("package %s: follow it with a manager such as brew, or a list such as [brew, pacman: %s]", name, name)
		}

		if !slices.Contains(PackageManagers, manager) {
			return nil, fmt.Errorf("package %s: unknown manager %q (want %s)", name, manager, strings.Join(PackageManagers, ", "))
		}
		if _, dup := managers[manager]; dup {
			return nil, fmt.Errorf("package %s: %s is listed twice", name, manager)
		}
		managers[manager] = as
	}

	// Both are Arch, and one item is one package there.
	if _, pacman := managers["pacman"]; pacman {
		if _, aur := managers["aur"]; aur {
			return nil, fmt.Errorf("package %s: lists both pacman and aur; make them two items", name)
		}
	}
	return managers, nil
}
