package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// FunctionList decodes the top-level `functions:` node in EITHER declaration
// format:
//   - reference list:  functions: [{name: api, entry: dist/api.handler, ...}]
//   - canonical map:   functions: {api: {handler: dist/api.handler, ...}}
//
// Map entries are converted to FunctionOverrideConfig (now field-complete), so
// downstream consumers — including the JSON-marshaled provider payload, which
// expects an array — see one uniform shape.
type FunctionList []FunctionOverrideConfig

func (f *FunctionList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var list []FunctionOverrideConfig
		if err := strictNodeDecode(node, &list); err != nil {
			return err
		}
		*f = list
		return nil
	case yaml.MappingNode:
		var byName map[string]FunctionConfig
		if err := strictNodeDecode(node, &byName); err != nil {
			return err
		}
		// Preserve the author's declaration order (map iteration is random).
		list := make([]FunctionOverrideConfig, 0, len(byName))
		for i := 0; i+1 < len(node.Content); i += 2 {
			name := node.Content[i].Value
			fc := byName[name]
			list = append(list, FunctionOverrideConfig{
				Name:                   name,
				Entry:                  fc.Handler,
				Runtime:                fc.Runtime,
				Memory:                 fc.Memory,
				Timeout:                fc.Timeout,
				Architecture:           fc.Architecture,
				Events:                 fc.Events,
				Environment:            fc.Environment,
				Secrets:                fc.Secrets,
				Tags:                   fc.Tags,
				Layers:                 fc.Layers,
				Resources:              fc.Resources,
				Addons:                 fc.Addons,
				ReservedConcurrency:    fc.ReservedConcurrency,
				ProvisionedConcurrency: fc.ProvisionedConcurrency,
			})
		}
		*f = list
		return nil
	case yaml.ScalarNode:
		// Empty/null node (e.g. `functions:` with nothing under it).
		if node.Value != "" && node.Value != "null" && node.Value != "~" {
			return fmt.Errorf("functions: expected a list or a map, got scalar %q", node.Value)
		}
		return nil
	default:
		return fmt.Errorf("functions: expected a list or a map")
	}
}

// strictNodeDecode re-decodes a sub-node with KnownFields so custom unmarshalers
// keep the loader's strict unknown-key behavior (node.Decode is lenient).
func strictNodeDecode(node *yaml.Node, out any) error {
	raw, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return LoadFromBytes(data)
}

// LoadFromBytes parses YAML and normalizes config. Used by the config API (POST /validate, POST /resolve).
func LoadFromBytes(data []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("parse yaml: expected a single YAML document")
	}
	Normalize(&cfg)
	return &cfg, nil
}
