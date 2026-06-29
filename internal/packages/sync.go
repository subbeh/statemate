package packages

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/subbeh/statemate/internal/config"
	"gopkg.in/yaml.v3"
)

type SyncResult struct {
	Manager  string
	Missing  []string
	Extra    []string
	Statuses []PackageStatus
}

func ComputeSync(cfg *config.Config, profileName string, sources []string) ([]SyncResult, error) {
	wanted := collectPackages(cfg, profileName)

	// Merge packages from source directories
	sourcePackages := CollectFromSources(sources)
	for manager, pkgs := range sourcePackages {
		for _, pkg := range pkgs {
			wanted[manager] = appendUnique(wanted[manager], pkg)
		}
	}

	// Resolve common packages to the primary available manager
	if commonPkgs, ok := wanted["common"]; ok && len(commonPkgs) > 0 {
		primaryManager := getPrimaryManager(cfg.AURHelper)
		if primaryManager != "" {
			for _, pkg := range commonPkgs {
				wanted[primaryManager] = appendUnique(wanted[primaryManager], pkg)
			}
		}
		delete(wanted, "common")
	}

	var results []SyncResult

	for managerName, wantedPkgs := range wanted {
		manager, err := GetManager(managerName, cfg.AURHelper)
		if err != nil {
			continue
		}
		if !manager.IsAvailable() {
			continue
		}

		installed, err := manager.ListInstalled()
		if err != nil {
			return nil, err
		}

		installedMap := make(map[string]Package)
		for _, p := range installed {
			installedMap[p.Name] = p
		}

		wantedMap := make(map[string]string)
		for _, spec := range wantedPkgs {
			name, version := ParsePackageSpec(spec)
			wantedMap[name] = version
		}

		result := SyncResult{Manager: managerName}

		for name, wantedVersion := range wantedMap {
			if inst, ok := installedMap[name]; ok {
				status := PackageStatus{
					Name:      name,
					Version:   wantedVersion,
					Status:    StatusInstalled,
					Installed: inst.Version,
				}
				if wantedVersion != "" && inst.Version != "" && inst.Version != wantedVersion {
					status.Status = StatusVersionMismatch
				}
				result.Statuses = append(result.Statuses, status)
			} else {
				result.Missing = append(result.Missing, name)
				result.Statuses = append(result.Statuses, PackageStatus{
					Name:    name,
					Version: wantedVersion,
					Status:  StatusMissing,
				})
			}
		}

		for name, inst := range installedMap {
			if _, ok := wantedMap[name]; !ok {
				result.Extra = append(result.Extra, name)
				result.Statuses = append(result.Statuses, PackageStatus{
					Name:      name,
					Status:    StatusExtra,
					Installed: inst.Version,
				})
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func collectPackages(cfg *config.Config, profileName string) map[string][]string {
	result := make(map[string][]string)

	// Global packages from main config
	if cfg.Packages != nil {
		appendPackages(result, cfg.Packages)
	}

	// Profile-specific packages
	if profileName != "" {
		if profile, ok := cfg.Profiles[profileName]; ok {
			if profile.Packages != nil {
				appendPackages(result, profile.Packages)
			}
		}
	}

	return result
}

// CollectFromSources scans source directories for .mate/packages.yaml files
func CollectFromSources(sources []string) map[string][]string {
	result := make(map[string][]string)

	for _, source := range sources {
		pkgs := loadSourcePackages(source)
		if pkgs != nil {
			appendPackages(result, pkgs)
		}
	}

	return result
}

func loadSourcePackages(sourceDir string) *config.PackageList {
	candidates := []string{
		filepath.Join(sourceDir, ".mate", "packages.yaml"),
		filepath.Join(sourceDir, ".mate", "packages.yml"),
		filepath.Join(sourceDir, ".mate", "packages.toml"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var pkgs config.PackageList
		switch {
		case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
			if err := yaml.Unmarshal(data, &pkgs); err != nil {
				continue
			}
		case strings.HasSuffix(path, ".toml"):
			if err := toml.Unmarshal(data, &pkgs); err != nil {
				continue
			}
		}
		return &pkgs
	}

	return nil
}

func appendPackages(result map[string][]string, pkgs *config.PackageList) {
	if pkgs == nil {
		return
	}

	for _, p := range pkgs.Common {
		result["common"] = appendUnique(result["common"], p)
	}
	for _, p := range pkgs.Brew {
		result["brew"] = appendUnique(result["brew"], p)
	}
	for _, p := range pkgs.Pacman {
		result["pacman"] = appendUnique(result["pacman"], p)
	}
	for _, p := range pkgs.AUR {
		result["aur"] = appendUnique(result["aur"], p)
	}
}

// getPrimaryManager returns the first available package manager for common packages
func getPrimaryManager(aurHelper string) string {
	// Check in order of preference: brew (macOS), pacman (Arch)
	managers := []Manager{
		NewBrewManager(),
		NewPacmanManager(),
	}

	for _, m := range managers {
		if m.IsAvailable() {
			return m.Name()
		}
	}
	return ""
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
