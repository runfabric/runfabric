package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

type stepFunctionDecl struct {
	Name           string
	Definition     map[string]any
	DefinitionPath string
	Role           string
	Bindings       map[string]string
}

func stepFunctionsFromConfig(cfg *config.Config, root string) ([]stepFunctionDecl, error) {
	if cfg == nil || cfg.Extensions == nil {
		return nil, nil
	}
	rawAWS, ok := cfg.Extensions["aws-lambda"]
	if !ok || rawAWS == nil {
		return nil, nil
	}
	awsMap, ok := rawAWS.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("extensions.aws-lambda must be an object")
	}
	rawSFN, ok := awsMap["stepFunctions"]
	if !ok || rawSFN == nil {
		return nil, nil
	}
	items, ok := rawSFN.([]any)
	if !ok {
		return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions must be an array")
	}
	out := make([]stepFunctionDecl, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d] must be an object", i)
		}
		name := strings.TrimSpace(anyString(m["name"]))
		if name == "" {
			return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].name is required", i)
		}
		decl := stepFunctionDecl{
			Name:           name,
			Role:           strings.TrimSpace(anyString(m["role"])),
			DefinitionPath: strings.TrimSpace(anyString(m["definitionPath"])),
			Bindings:       map[string]string{},
		}
		if rawBindings, ok := m["bindings"]; ok && rawBindings != nil {
			bindingsMap, ok := rawBindings.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].bindings must be an object", i)
			}
			for k, v := range bindingsMap {
				key := strings.TrimSpace(k)
				if key == "" {
					return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].bindings contains empty key", i)
				}
				value := strings.TrimSpace(anyString(v))
				if value == "" {
					return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].bindings.%s must be a non-empty string", i, key)
				}
				decl.Bindings[key] = value
			}
		}
		if rawDef, ok := m["definition"]; ok && rawDef != nil {
			defMap, ok := rawDef.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].definition must be an object", i)
			}
			decl.Definition = defMap
		}
		if decl.Definition == nil && decl.DefinitionPath == "" {
			return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d] must set definition or definitionPath", i)
		}
		if decl.Definition != nil && decl.DefinitionPath != "" {
			return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d] must set only one of definition or definitionPath", i)
		}
		if decl.DefinitionPath != "" {
			path := decl.DefinitionPath
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("extensions.aws-lambda.stepFunctions[%d].definitionPath not found: %s", i, decl.DefinitionPath)
			}
		}
		out = append(out, decl)
	}
	return out, nil
}

func stepFunctionDefinitionString(root string, decl stepFunctionDecl) (string, error) {
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

func applyStepFunctionBindings(definition string, decl stepFunctionDecl, lambdaARNByFunction map[string]string) string {
	if strings.TrimSpace(definition) == "" || len(decl.Bindings) == 0 {
		return definition
	}
	replacerArgs := make([]string, 0, len(decl.Bindings)*8)
	for key, ref := range decl.Bindings {
		resolved := resolveBindingValue(ref, lambdaARNByFunction)
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
		return definition
	}
	return strings.NewReplacer(replacerArgs...).Replace(definition)
}

func resolveBindingValue(ref string, lambdaARNByFunction map[string]string) string {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "arn:") {
		return trimmed
	}
	if arn := strings.TrimSpace(lambdaARNByFunction[trimmed]); arn != "" {
		return arn
	}
	return trimmed
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
