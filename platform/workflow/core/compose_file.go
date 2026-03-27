package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ComposeService is one service in a runfabric.compose.yml.
type ComposeService struct {
	Name      string   `yaml:"name"`
	Config    string   `yaml:"config"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
}

// ComposeFile is the root of runfabric.compose.yml.
type ComposeFile struct {
	Services []ComposeService `yaml:"services"`
}

// LoadCompose reads and parses a compose file. Paths in services are relative to the compose file directory.
func LoadCompose(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	var out ComposeFile
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("compose file has no services")
	}
	return &out, nil
}

// ResolveServiceConfigPaths resolves each service's config path relative to the compose file directory.
// Returns a map of service name -> absolute config path.
func ResolveServiceConfigPaths(composePath string, c *ComposeFile) (map[string]string, error) {
	base := filepath.Dir(composePath)
	out := make(map[string]string)
	for _, svc := range c.Services {
		abs := filepath.Join(base, svc.Config)
		abs, err := filepath.Abs(abs)
		if err != nil {
			return nil, fmt.Errorf("resolve config for service %q: %w", svc.Name, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("service %q config %q: %w", svc.Name, abs, err)
		}
		out[svc.Name] = abs
	}
	return out, nil
}

// TopoOrder returns service names in dependency order (dependencies first). Returns error on cycle or unknown dep.
func TopoOrder(c *ComposeFile) ([]string, error) {
	names := make(map[string]bool)
	for _, s := range c.Services {
		names[s.Name] = true
	}
	for _, s := range c.Services {
		for _, d := range s.DependsOn {
			if !names[d] {
				return nil, fmt.Errorf("service %q depends on unknown service %q", s.Name, d)
			}
		}
	}
	var order []string
	visited := make(map[string]bool)
	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true
		for _, s := range c.Services {
			if s.Name != name {
				continue
			}
			for _, d := range s.DependsOn {
				if !visited[d] {
					if err := visit(d); err != nil {
						return err
					}
				}
			}
			break
		}
		order = append(order, name)
		return nil
	}
	for _, s := range c.Services {
		if err := visit(s.Name); err != nil {
			return nil, err
		}
	}
	// detect cycle: if any dep appears after the node in order, we have a cycle
	idx := make(map[string]int)
	for i, n := range order {
		idx[n] = i
	}
	for _, s := range c.Services {
		for _, d := range s.DependsOn {
			if idx[d] > idx[s.Name] {
				return nil, fmt.Errorf("cycle in dependsOn involving %q and %q", s.Name, d)
			}
		}
	}
	return order, nil
}
