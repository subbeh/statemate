package template

import (
	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

func parseYAML(data []byte) (map[string]any, error) {
	var vars map[string]any
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func parseTOML(data []byte) (map[string]any, error) {
	var vars map[string]any
	if err := toml.Unmarshal(data, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}
