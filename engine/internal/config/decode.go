package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML supports "functions" as either a map (legacy) or array (reference format).
func (f *FunctionsRaw) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.MappingNode:
		f.AsMap = make(map[string]FunctionConfig)
		return value.Decode(&f.AsMap)
	case yaml.SequenceNode:
		return value.Decode(&f.AsArray)
	default:
		return fmt.Errorf("functions must be object or array, got %v", value.Kind)
	}
}
