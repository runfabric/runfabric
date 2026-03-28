package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

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
