package profile

import (
	"github.com/subbeh/statemate/internal/config"
)

func Resolve(cfg *config.Config, profileName string) *config.Profile {
	if profileName == "" {
		return nil
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return nil
	}

	if profile.Extends == "" {
		return profile
	}

	resolved := &config.Profile{
		Detection: profile.Detection,
		Variables: make(map[string]any),
	}

	chain := resolveChain(cfg, profileName)

	for _, p := range chain {
		if p.Packages != nil {
			resolved.Packages = mergePackages(resolved.Packages, p.Packages)
		}
		for k, v := range p.Variables {
			resolved.Variables[k] = v
		}
	}

	return resolved
}

func resolveChain(cfg *config.Config, profileName string) []*config.Profile {
	var chain []*config.Profile
	seen := make(map[string]bool)

	for name := profileName; name != ""; {
		if seen[name] {
			break
		}
		seen[name] = true

		profile, ok := cfg.Profiles[name]
		if !ok {
			break
		}

		chain = append([]*config.Profile{profile}, chain...)
		name = profile.Extends
	}

	return chain
}

func mergePackages(base, overlay *config.PackageList) *config.PackageList {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := &config.PackageList{
		Common: uniqueStrings(append(base.Common, overlay.Common...)),
		Brew:   uniqueStrings(append(base.Brew, overlay.Brew...)),
		Pacman: uniqueStrings(append(base.Pacman, overlay.Pacman...)),
		AUR:    uniqueStrings(append(base.AUR, overlay.AUR...)),
	}
	return result
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
