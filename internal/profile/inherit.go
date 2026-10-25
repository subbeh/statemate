package profile

import (
	"slices"

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
			resolved.Packages = slices.Concat(resolved.Packages, p.Packages)
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
