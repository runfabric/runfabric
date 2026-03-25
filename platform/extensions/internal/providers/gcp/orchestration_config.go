package gcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type cloudWorkflowDecl struct {
	Name           string
	Definition     map[string]any
	DefinitionPath string
	Bindings       map[string]string
}

func cloudWorkflowsFromConfig(cfg sdkprovider.Config, root string) ([]cloudWorkflowDecl, error) {
	if cfg == nil {
		return nil, nil
	}
	rawExt, ok := cfg["extensions"]
	if !ok || rawExt == nil {
		return nil, nil
	}
	extMap, ok := rawExt.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("extensions must be an object")
	}
	rawGCP, ok := extMap["gcp-functions"]
	if !ok || rawGCP == nil {
		return nil, nil
	}
	gcpMap, ok := rawGCP.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("extensions.gcp-functions must be an object")
	}
	rawWF, ok := gcpMap["cloudWorkflows"]
	if !ok || rawWF == nil {
		return nil, nil
	}
	items, ok := rawWF.([]any)
	if !ok {
		return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows must be an array")
	}
	out := make([]cloudWorkflowDecl, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d] must be an object", i)
		}
		name := strings.TrimSpace(anyString(m["name"]))
		if name == "" {
			return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].name is required", i)
		}
		decl := cloudWorkflowDecl{
			Name:           name,
			DefinitionPath: strings.TrimSpace(anyString(m["definitionPath"])),
			Bindings:       map[string]string{},
		}
		if rawBindings, ok := m["bindings"]; ok && rawBindings != nil {
			bindingsMap, ok := rawBindings.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].bindings must be an object", i)
			}
			for k, v := range bindingsMap {
				key := strings.TrimSpace(k)
				if key == "" {
					return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].bindings contains empty key", i)
				}
				value := strings.TrimSpace(anyString(v))
				if value == "" {
					return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].bindings.%s must be a non-empty string", i, key)
				}
				decl.Bindings[key] = value
			}
		}
		if rawDef, ok := m["definition"]; ok && rawDef != nil {
			defMap, ok := rawDef.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].definition must be an object", i)
			}
			decl.Definition = defMap
		}
		if decl.Definition == nil && decl.DefinitionPath == "" {
			return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d] must set definition or definitionPath", i)
		}
		if decl.Definition != nil && decl.DefinitionPath != "" {
			return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d] must set only one of definition or definitionPath", i)
		}
		if decl.DefinitionPath != "" {
			path := decl.DefinitionPath
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("extensions.gcp-functions.cloudWorkflows[%d].definitionPath not found: %s", i, decl.DefinitionPath)
			}
		}
		out = append(out, decl)
	}
	return out, nil
}

func cloudWorkflowDefinitionString(root string, decl cloudWorkflowDecl) (string, error) {
	if decl.Definition != nil {
		b, err := json.Marshal(decl.Definition)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	path := decl.DefinitionPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func applyCloudWorkflowBindings(source string, decl cloudWorkflowDecl, functionResourceByName map[string]string) string {
	if strings.TrimSpace(source) == "" || len(decl.Bindings) == 0 {
		return source
	}
	replacerArgs := make([]string, 0, len(decl.Bindings)*8)
	for key, ref := range decl.Bindings {
		resolved := resolveCloudWorkflowBinding(ref, functionResourceByName)
		if resolved == "" {
			continue
		}
		replacerArgs = append(replacerArgs,
			"${bindings."+key+"}", resolved,
			"${binding."+key+"}", resolved,
			"{{bindings."+key+"}}", resolved,
			"{{binding."+key+"}}", resolved,
		)
	}
	if len(replacerArgs) == 0 {
		return source
	}
	return strings.NewReplacer(replacerArgs...).Replace(source)
}

func resolveCloudWorkflowBinding(ref string, functionResourceByName map[string]string) string {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if value := strings.TrimSpace(functionResourceByName[trimmed]); value != "" {
		return value
	}
	return trimmed
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
