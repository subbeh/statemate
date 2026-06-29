package config

import (
	"fmt"
	"os"
)

func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return fmt.Errorf("no sources defined")
	}

	for _, source := range c.AbsoluteSources() {
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", source)
		}
	}

	for name, profile := range c.Profiles {
		if profile == nil {
			continue
		}
		if profile.Extends != "" {
			if _, ok := c.Profiles[profile.Extends]; !ok {
				return fmt.Errorf("profile %q extends unknown profile %q", name, profile.Extends)
			}
		}
		if err := validateDetection(profile.Detection); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
	}

	return nil
}

func validateDetection(d *Detection) error {
	if d == nil {
		return nil
	}
	if d.Mode != "" && d.Mode != "and" && d.Mode != "or" {
		return fmt.Errorf("detection mode must be 'and' or 'or', got %q", d.Mode)
	}
	return nil
}

func (c *DirConfig) Validate() error {
	return nil
}
