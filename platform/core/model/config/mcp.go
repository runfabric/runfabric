package config

import (
	"fmt"
	"strings"
)

type MCPServerConfig struct {
	URL string
}

type MCPIntegrationsConfig struct {
	Servers map[string]MCPServerConfig
}

type MCPPolicyRuleSet struct {
	Servers   []string
	Tools     []string
	Resources []string
	Prompts   []string
}

// MCPProviderPolicyRule holds provider-context enforcement rules for MCP policy.
// Rules are keyed by normalized provider id (e.g. aws-lambda, gcp-functions, azure-functions)
// under policies.mcp.providers.
type MCPProviderPolicyRule struct {
	// RequiredRegion, if set, denies MCP calls when the active region does not match.
	RequiredRegion string
	// RequiredAuth names the required auth mechanism (e.g. "oauth", "managed-identity").
	// Enforcement is advisory — recorded in metadata for audit; real auth is at the MCP client.
	RequiredAuth string
	// DenyCrossRegion denies calls when the active region differs from RequiredRegion.
	DenyCrossRegion bool
	// DenyRegions lists specific regions from which MCP calls are blocked.
	DenyRegions []string
	// Models configures per-step model selection overrides.
	// Supported keys: default, ai-retrieval, ai-generate, ai-structured, ai-eval.
	Models map[string]string
}

type MCPPolicyConfig struct {
	DefaultDeny bool
	Allow       MCPPolicyRuleSet
	Deny        MCPPolicyRuleSet
	// Providers holds provider-context rules keyed by normalized provider id.
	Providers map[string]MCPProviderPolicyRule
}

func ParseMCPIntegrations(cfg *Config) (MCPIntegrationsConfig, error) {
	out := MCPIntegrationsConfig{Servers: map[string]MCPServerConfig{}}
	if cfg == nil || cfg.Integrations == nil {
		return out, nil
	}
	raw, ok := cfg.Integrations["mcp"]
	if !ok || raw == nil {
		return out, nil
	}
	mcpObj, ok := raw.(map[string]any)
	if !ok {
		return out, fmt.Errorf("integrations.mcp must be an object")
	}
	serversRaw, ok := mcpObj["servers"]
	if !ok || serversRaw == nil {
		return out, nil
	}
	serversObj, ok := serversRaw.(map[string]any)
	if !ok {
		return out, fmt.Errorf("integrations.mcp.servers must be an object")
	}
	for name, entry := range serversObj {
		sname := strings.TrimSpace(name)
		if sname == "" {
			return out, fmt.Errorf("integrations.mcp.servers contains empty server name")
		}
		serverObj, ok := entry.(map[string]any)
		if !ok {
			return out, fmt.Errorf("integrations.mcp.servers.%s must be an object", sname)
		}
		url := asString(serverObj["url"])
		if url == "" {
			return out, fmt.Errorf("integrations.mcp.servers.%s.url is required", sname)
		}
		out.Servers[sname] = MCPServerConfig{URL: url}
	}
	return out, nil
}

func ParseMCPPolicy(cfg *Config) (MCPPolicyConfig, error) {
	out := MCPPolicyConfig{}
	if cfg == nil || cfg.Policies == nil {
		return out, nil
	}
	raw, ok := cfg.Policies["mcp"]
	if !ok || raw == nil {
		return out, nil
	}
	mcpObj, ok := raw.(map[string]any)
	if !ok {
		return out, fmt.Errorf("policies.mcp must be an object")
	}
	if rawDeny, ok := mcpObj["defaultDeny"]; ok && rawDeny != nil {
		d, ok := rawDeny.(bool)
		if !ok {
			return out, fmt.Errorf("policies.mcp.defaultDeny must be a boolean")
		}
		out.DefaultDeny = d
	}
	if allowRaw, ok := mcpObj["allow"]; ok && allowRaw != nil {
		allowObj, ok := allowRaw.(map[string]any)
		if !ok {
			return out, fmt.Errorf("policies.mcp.allow must be an object")
		}
		rules, err := parseMCPPolicyRuleSet("policies.mcp.allow", allowObj)
		if err != nil {
			return out, err
		}
		out.Allow = rules
	}
	if denyRaw, ok := mcpObj["deny"]; ok && denyRaw != nil {
		denyObj, ok := denyRaw.(map[string]any)
		if !ok {
			return out, fmt.Errorf("policies.mcp.deny must be an object")
		}
		rules, err := parseMCPPolicyRuleSet("policies.mcp.deny", denyObj)
		if err != nil {
			return out, err
		}
		out.Deny = rules
	}
	if providersRaw, ok := mcpObj["providers"]; ok && providersRaw != nil {
		providersObj, ok := providersRaw.(map[string]any)
		if !ok {
			return out, fmt.Errorf("policies.mcp.providers must be an object")
		}
		out.Providers = make(map[string]MCPProviderPolicyRule, len(providersObj))
		for provName, provRaw := range providersObj {
			normalizedProvider, err := normalizeMCPProviderPolicyKey(provName)
			if err != nil {
				return out, err
			}
			if normalizedProvider == "" {
				return out, fmt.Errorf("policies.mcp.providers contains empty provider name")
			}
			provObj, ok := provRaw.(map[string]any)
			if !ok {
				return out, fmt.Errorf("policies.mcp.providers.%s must be an object", provName)
			}
			rule := MCPProviderPolicyRule{
				RequiredRegion: asString(provObj["requiredRegion"]),
				RequiredAuth:   asString(provObj["requiredAuth"]),
			}
			if v, ok := provObj["denyCrossRegion"].(bool); ok {
				rule.DenyCrossRegion = v
			}
			if denyRegions, ok := provObj["denyRegions"]; ok && denyRegions != nil {
				list, err := readStringList(map[string]any{"denyRegions": denyRegions}, "denyRegions",
					"policies.mcp.providers."+provName)
				if err != nil {
					return out, err
				}
				rule.DenyRegions = list
			}
			if modelsRaw, ok := provObj["models"]; ok && modelsRaw != nil {
				modelsObj, ok := modelsRaw.(map[string]any)
				if !ok {
					return out, fmt.Errorf("policies.mcp.providers.%s.models must be an object", provName)
				}
				rule.Models = make(map[string]string, len(modelsObj))
				for stepKind, modelRaw := range modelsObj {
					normalizedKind := strings.ToLower(strings.TrimSpace(stepKind))
					if normalizedKind == "" {
						return out, fmt.Errorf("policies.mcp.providers.%s.models contains empty key", provName)
					}
					model, ok := modelRaw.(string)
					if !ok || strings.TrimSpace(model) == "" {
						return out, fmt.Errorf("policies.mcp.providers.%s.models.%s must be a non-empty string", provName, normalizedKind)
					}
					rule.Models[normalizedKind] = strings.TrimSpace(model)
				}
			}
			out.Providers[normalizedProvider] = rule
		}
	}
	return out, nil
}

func normalizeMCPProviderPolicyKey(raw string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(raw))
	switch provider {
	case "":
		return "", nil
	case "aws":
		return "", fmt.Errorf("policies.mcp.providers.%s is not supported; use %q", raw, "aws-lambda")
	case "gcp":
		return "", fmt.Errorf("policies.mcp.providers.%s is not supported; use %q", raw, "gcp-functions")
	case "azure":
		return "", fmt.Errorf("policies.mcp.providers.%s is not supported; use %q", raw, "azure-functions")
	case "aws-lambda", "gcp-functions", "azure-functions":
		return provider, nil
	default:
		return provider, nil
	}
}

func ValidateMCPConfig(cfg *Config) error {
	integrations, err := ParseMCPIntegrations(cfg)
	if err != nil {
		return err
	}
	policy, err := ParseMCPPolicy(cfg)
	if err != nil {
		return err
	}
	serverExists := func(name string) bool {
		if strings.TrimSpace(name) == "*" {
			return true
		}
		_, ok := integrations.Servers[strings.TrimSpace(name)]
		return ok
	}
	validateServers := func(path string, list []string) error {
		for i, s := range list {
			v := strings.TrimSpace(s)
			if v == "" {
				return fmt.Errorf("%s[%d] must not be empty", path, i)
			}
			if !serverExists(v) {
				return fmt.Errorf("%s[%d] references unknown MCP server %q", path, i, v)
			}
		}
		return nil
	}
	if err := validateServers("policies.mcp.allow.servers", policy.Allow.Servers); err != nil {
		return err
	}
	if err := validateServers("policies.mcp.deny.servers", policy.Deny.Servers); err != nil {
		return err
	}
	return nil
}

func parseMCPPolicyRuleSet(path string, raw map[string]any) (MCPPolicyRuleSet, error) {
	out := MCPPolicyRuleSet{}
	var err error
	out.Servers, err = readStringList(raw, "servers", path)
	if err != nil {
		return out, err
	}
	out.Tools, err = readStringList(raw, "tools", path)
	if err != nil {
		return out, err
	}
	out.Resources, err = readStringList(raw, "resources", path)
	if err != nil {
		return out, err
	}
	out.Prompts, err = readStringList(raw, "prompts", path)
	if err != nil {
		return out, err
	}
	return out, nil
}

func readStringList(obj map[string]any, key, path string) ([]string, error) {
	raw, ok := obj[key]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s.%s must be an array", path, key)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s.%s[%d] must be a non-empty string", path, key, i)
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, nil
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
