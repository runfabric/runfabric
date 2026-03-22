// Package configpatch provides safe YAML patching for runfabric.yml: merge new
// function entries without full regeneration. Used by runfabric generate.
package configpatch

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AddMapEntryOptions controls backup when patching any root-level map.
type AddMapEntryOptions struct {
	Backup bool
}

// AddMapEntry reads runfabric.yml at path, merges a new entry under rootKey.entryKey, and writes back.
// rootKey is e.g. "functions", "resources", "addons", "providerOverrides". The existing rootKey must be a map.
// If entryKey already exists, returns an error unless the entry is identical (no-op). conflictMsg is used when
// the key exists with a different value (e.g. "function %q already exists in runfabric.yml; choose another name or remove it first").
func AddMapEntry(path, rootKey, entryKey string, entry map[string]any, opts AddMapEntryOptions, conflictMsg string) error {
	if opts.Backup {
		if err := backup(path); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	current, _ := root[rootKey]
	if current == nil {
		root[rootKey] = map[string]any{entryKey: entry}
	} else {
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be a map to patch (found %T); use init or edit manually", rootKey, current)
		}
		if existing, exists := m[entryKey]; exists {
			if !mapsEqual(existing, entry) {
				return fmt.Errorf(conflictMsg, entryKey)
			}
			return nil
		}
		m[entryKey] = entry
	}
	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}

// PlanAddMapEntry returns the merged map fragment for rootKey (for dry-run) without writing. collision is true if entryKey exists.
func PlanAddMapEntry(path, rootKey, entryKey string, entry map[string]any) (merged map[string]any, collision bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, false, fmt.Errorf("parse yaml: %w", err)
	}
	current, _ := root[rootKey]
	if current == nil {
		return map[string]any{entryKey: entry}, false, nil
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%s must be a map to patch (found %T)", rootKey, current)
	}
	if _, exists := m[entryKey]; exists {
		return nil, true, nil
	}
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	out[entryKey] = entry
	return out, false, nil
}

// AddFunctionOptions controls backup and merge behavior.
type AddFunctionOptions struct {
	Backup bool // if true, write path to path+".bak" before overwriting (default true)
}

// AddFunction reads runfabric.yml at path, appends a new function entry to the
// functions array, and writes back. The existing "functions" key must be an
// array. If the function name already exists, returns an error unless the entry
// is identical (no-op). Backup is written to path+".bak" when Backup is true.
func AddFunction(path string, name string, functionEntry map[string]any, opts AddFunctionOptions) error {
	if opts.Backup {
		if err := backup(path); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	entry := functionArrayEntry(name, functionEntry)
	functions, err := loadFunctionsArray(root["functions"])
	if err != nil {
		return err
	}
	for _, current := range functions {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("functions entries must be objects")
		}
		if currentName, _ := currentMap["name"].(string); currentName == name {
			if mapsEqual(currentMap, entry) {
				return nil
			}
			return fmt.Errorf("function %q already exists in runfabric.yml; choose another name or remove it first", name)
		}
	}
	root["functions"] = append(functions, entry)
	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}

func backup(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backPath := path + ".bak"
	return os.WriteFile(backPath, data, 0o644)
}

// mapsEqual does a shallow equality check for map[string]any (used for no-op detection).
func mapsEqual(a, b any) bool {
	am, ok1 := a.(map[string]any)
	bm, ok2 := b.(map[string]any)
	if !ok1 || !ok2 {
		return false
	}
	if len(am) != len(bm) {
		return false
	}
	for k, v := range am {
		if v2, ok := bm[k]; !ok || !yamlValueEqual(v, v2) {
			return false
		}
	}
	return true
}

func yamlValueEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
		return false
	case int:
		if bv, ok := b.(int); ok {
			return av == bv
		}
		if bv, ok := b.(int64); ok {
			return int64(av) == bv
		}
		return false
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !yamlValueEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		return mapsEqual(a, b)
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

// PlanAddFunction returns a description of the patch (for dry-run) without writing.
// It returns the function entry that would be appended and any error (e.g. collision).
func PlanAddFunction(path string, name string, functionEntry map[string]any) (mergedFragment map[string]any, collision bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, false, fmt.Errorf("parse yaml: %w", err)
	}
	functions, err := loadFunctionsArray(root["functions"])
	if err != nil {
		return nil, false, err
	}
	for _, current := range functions {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("functions entries must be objects")
		}
		if currentName, _ := currentMap["name"].(string); currentName == name {
			return nil, true, nil
		}
	}
	return functionArrayEntry(name, functionEntry), false, nil
}

// AddResourceOptions controls backup when patching resources.
type AddResourceOptions struct {
	Backup bool
}

// AddResource merges a new resource entry under "resources.<name>" and writes back.
func AddResource(path string, name string, resourceEntry map[string]any, opts AddResourceOptions) error {
	return AddMapEntry(path, "resources", name, resourceEntry, AddMapEntryOptions{Backup: opts.Backup},
		"resource %q already exists in runfabric.yml; choose another name or remove it first")
}

// AddAddonOptions controls backup when patching addons.
type AddAddonOptions struct {
	Backup bool
}

// AddAddon merges a new addon entry under "addons.<name>" and writes back.
func AddAddon(path string, name string, addonEntry map[string]any, opts AddAddonOptions) error {
	return AddMapEntry(path, "addons", name, addonEntry, AddMapEntryOptions{Backup: opts.Backup},
		"addon %q already exists in runfabric.yml; choose another name or remove it first")
}

// AddProviderOverrideOptions controls backup when patching providerOverrides.
type AddProviderOverrideOptions struct {
	Backup bool
}

// AddProviderOverride merges a new provider under "providerOverrides.<key>" and writes back.
func AddProviderOverride(path string, key string, providerEntry map[string]any, opts AddProviderOverrideOptions) error {
	return AddMapEntry(path, "providerOverrides", key, providerEntry, AddMapEntryOptions{Backup: opts.Backup},
		"provider override %q already exists in runfabric.yml; choose another key or remove it first")
}

// AppendFunctionAddon appends addonID to the named function entry if not already present. Creates addons array if missing.
func AppendFunctionAddon(path, functionName, addonID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	functions, err := loadFunctionsArray(root["functions"])
	if err != nil {
		return err
	}
	for index, fn := range functions {
		fnEntry, ok := fn.(map[string]any)
		if !ok {
			return fmt.Errorf("functions entries must be objects")
		}
		currentName, _ := fnEntry["name"].(string)
		if currentName != functionName {
			continue
		}
		var addons []any
		if a := fnEntry["addons"]; a != nil {
			addons, ok = a.([]any)
			if !ok {
				return fmt.Errorf("functions[%s].addons must be an array", functionName)
			}
			for _, v := range addons {
				if s, ok := v.(string); ok && s == addonID {
					return nil
				}
			}
		}
		fnEntry["addons"] = append(addons, addonID)
		functions[index] = fnEntry
		root["functions"] = functions
		out, err := yaml.Marshal(root)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		return os.WriteFile(path, out, 0o644)
	}
	return fmt.Errorf("function %q not found", functionName)
}

// RemoveFunctionAddon removes addonID from the named function entry's addons array.
// Returns an error if the function is not found. If the addon is not present it is a no-op.
func RemoveFunctionAddon(path, functionName, addonID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	functions, err := loadFunctionsArray(root["functions"])
	if err != nil {
		return err
	}
	for index, fn := range functions {
		fnEntry, ok := fn.(map[string]any)
		if !ok {
			return fmt.Errorf("functions entries must be objects")
		}
		currentName, _ := fnEntry["name"].(string)
		if currentName != functionName {
			continue
		}
		if a := fnEntry["addons"]; a != nil {
			addons, ok := a.([]any)
			if !ok {
				return fmt.Errorf("functions[%s].addons must be an array", functionName)
			}
			filtered := make([]any, 0, len(addons))
			for _, v := range addons {
				if s, ok := v.(string); ok && s == addonID {
					continue
				}
				filtered = append(filtered, v)
			}
			if len(filtered) == 0 {
				delete(fnEntry, "addons")
			} else {
				fnEntry["addons"] = filtered
			}
		}
		functions[index] = fnEntry
		root["functions"] = functions
		out, err := yaml.Marshal(root)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		return os.WriteFile(path, out, 0o644)
	}
	return fmt.Errorf("function %q not found", functionName)
}

func functionArrayEntry(name string, entry map[string]any) map[string]any {
	out := make(map[string]any, len(entry)+1)
	out["name"] = name
	for key, value := range entry {
		out[key] = value
	}
	return out
}

func loadFunctionsArray(current any) ([]any, error) {
	if current == nil {
		return []any{}, nil
	}
	functions, ok := current.([]any)
	if !ok {
		return nil, fmt.Errorf("functions must be an array to patch (found %T); edit manually", current)
	}
	return functions, nil
}

// ResolveConfigPath returns the absolute path to runfabric.yml. It uses path if non-empty,
// otherwise looks for runfabric.yml or runfabric.yaml in dir, then in parent directories up to maxDepth.
func ResolveConfigPath(path, dir string, maxDepth int) (string, error) {
	return resolveConfigPath(path, dir, maxDepth)
}

// ProjectRoot returns the project root directory (parent of config file) for a given config path.
// Use after ResolveConfigPath. Returns filepath.Dir(configPath).
func ProjectRoot(configPath string) string {
	return filepath.Dir(configPath)
}

// ResolveConfigAndRoot resolves config path and returns both config path and project root in one call.
// Convenience for commands that need both; dir is typically os.Getwd().
func ResolveConfigAndRoot(path, dir string, maxDepth int) (configPath, projectRoot string, err error) {
	configPath, err = ResolveConfigPath(path, dir, maxDepth)
	if err != nil {
		return "", "", err
	}
	return configPath, ProjectRoot(configPath), nil
}

func resolveConfigPath(path, dir string, maxDepth int) (string, error) {
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("config file %s: %w", abs, err)
		}
		return abs, nil
	}
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for i := 0; i <= maxDepth; i++ {
		for _, name := range []string{"runfabric.yml", "runfabric.yaml"} {
			candidate := filepath.Join(d, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("no runfabric.yml or runfabric.yaml found in %s or parents", dir)
}
