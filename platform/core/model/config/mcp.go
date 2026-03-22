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

type MCPPolicyConfig struct {
	DefaultDeny bool
	Allow       MCPPolicyRuleSet
	Deny        MCPPolicyRuleSet
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
	return out, nil
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
