package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML keeps the functions contract strict: reference-format arrays only.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	type rawConfig Config
	var raw rawConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	for i := 0; i < len(value.Content)-1; i += 2 {
		keyNode := value.Content[i]
		if keyNode.Value != "functions" {
			continue
		}
		functionsNode := value.Content[i+1]
		if functionsNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("functions must be an array")
		}
		break
	}

	*c = Config(raw)
	return nil
}
