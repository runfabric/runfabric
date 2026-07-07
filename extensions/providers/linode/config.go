package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type functionSpec struct {
	Name      string
	Runtime   string
	Entry     string
	Artifact  string
	Triggers  []string
	InvokeURL string
}

func (p *plugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	_ = ctx
	_, _, _, err := p.inspectConfig(req.Config)
	return err
}

func (p *plugin) inspectConfig(cfg sdkprovider.Config) (string, []functionSpec, []string, error) {
	service := strings.TrimSpace(asString(cfg["service"]))
	if service == "" {
		service = "linode-service"
	}
	functions, err := extractFunctions(cfg, service)
	if err != nil {
		return "", nil, nil, err
	}
	warnings := make([]string, 0)
	for _, fn := range functions {
		for _, trigger := range fn.Triggers {
			if trigger != "" && trigger != "http" {
				warnings = append(warnings, fmt.Sprintf("function %s uses trigger %s which is not advertised by this plugin", fn.Name, trigger))
			}
		}
	}
	return service, functions, dedupeStrings(warnings), nil
}

func extractFunctions(cfg sdkprovider.Config, service string) ([]functionSpec, error) {
	var functions []functionSpec
	if raw := cfg["functions"]; raw != nil {
		entries, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("config.functions must be a list")
		}
		for index, item := range entries {
			fnMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("config.functions[%d] must be an object", index)
			}
			name := strings.TrimSpace(asString(fnMap["name"]))
			if name == "" {
				return nil, fmt.Errorf("config.functions[%d].name is required", index)
			}
			runtime := normalizeRuntime(asString(fnMap["runtime"]))
			if runtime == "" {
				runtime = normalizeRuntime(asString(cfg["runtime"]))
			}
			if runtime == "" {
				runtime = "nodejs"
			}
			if !isSupportedRuntime(runtime) {
				return nil, fmt.Errorf("function %s uses unsupported runtime %q", name, asString(fnMap["runtime"]))
			}
			functions = append(functions, functionSpec{
				Name:      name,
				Runtime:   runtime,
				Entry:     firstNonEmpty(strings.TrimSpace(asString(fnMap["entry"])), strings.TrimSpace(asString(cfg["entry"]))),
				Artifact:  firstNonEmpty(strings.TrimSpace(asString(fnMap["artifact"])), strings.TrimSpace(asString(fnMap["outputPath"]))),
				Triggers:  extractTriggerTypes(firstNonNil(fnMap["triggers"], cfg["triggers"])),
				InvokeURL: firstNonEmpty(strings.TrimSpace(asString(fnMap["invokeUrl"])), strings.TrimSpace(asString(fnMap["url"]))),
			})
		}
	}
	if len(functions) == 0 {
		runtime := normalizeRuntime(asString(cfg["runtime"]))
		if runtime == "" {
			runtime = "nodejs"
		}
		if !isSupportedRuntime(runtime) {
			return nil, fmt.Errorf("unsupported runtime %q", asString(cfg["runtime"]))
		}
		functions = append(functions, functionSpec{
			Name:      service,
			Runtime:   runtime,
			Entry:     strings.TrimSpace(asString(cfg["entry"])),
			Artifact:  firstNonEmpty(strings.TrimSpace(asString(cfg["artifact"])), strings.TrimSpace(asString(cfg["outputPath"]))),
			Triggers:  extractTriggerTypes(cfg["triggers"]),
			InvokeURL: firstNonEmpty(strings.TrimSpace(asString(cfg["invokeUrl"])), strings.TrimSpace(asString(cfg["url"]))),
		})
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].Name < functions[j].Name })
	return functions, nil
}

func (p *plugin) resolveToken(cfg sdkprovider.Config) (string, string) {
	if token := strings.TrimSpace(asString(cfg["token"])); token != "" {
		return token, "config.token"
	}
	envName := strings.TrimSpace(asString(cfg["tokenEnv"]))
	if envName == "" {
		envName = defaultTokenEnv
	}
	if token := strings.TrimSpace(p.getenv(envName)); token != "" {
		return token, envName
	}
	return "", envName
}

func (p *plugin) resolveCommand(cfg sdkprovider.Config, operation string) string {
	if commands, ok := cfg["commands"].(map[string]any); ok {
		if cmd := strings.TrimSpace(asString(commands[operation])); cmd != "" {
			return cmd
		}
	}
	if cmd := strings.TrimSpace(asString(cfg[operation+"Command"])); cmd != "" {
		return cmd
	}
	if cmd := strings.TrimSpace(p.getenv(commandEnvForOperation(operation))); cmd != "" {
		return cmd
	}
	return ""
}

func (p *plugin) resolveFunctionName(cfg sdkprovider.Config, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	service, functions, _, err := p.inspectConfig(cfg)
	if err == nil && len(functions) == 1 {
		return functions[0].Name
	}
	return service
}

func (p *plugin) resolveInvokeURL(cfg sdkprovider.Config, function string) string {
	service, functions, _, err := p.inspectConfig(cfg)
	if err != nil {
		return ""
	}
	for _, fn := range functions {
		if function != "" && fn.Name != function {
			continue
		}
		if fn.InvokeURL != "" {
			return fn.InvokeURL
		}
	}
	if function == "" || function == service {
		return firstNonEmpty(strings.TrimSpace(asString(cfg["invokeUrl"])), strings.TrimSpace(asString(cfg["url"])))
	}
	return ""
}

func extractTriggerTypes(raw any) []string {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	triggers := make([]string, 0, len(entries))
	for _, item := range entries {
		triggerMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		trigger := strings.ToLower(strings.TrimSpace(asString(triggerMap["type"])))
		if trigger != "" {
			triggers = append(triggers, trigger)
		}
	}
	return dedupeStrings(triggers)
}

func normalizeRuntime(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(v, "nodejs"):
		return "nodejs"
	case strings.HasPrefix(v, "python"):
		return "python"
	default:
		return v
	}
}

func isSupportedRuntime(runtime string) bool {
	return runtime == "nodejs" || runtime == "python"
}

func commandEnvForOperation(operation string) string {
	switch operation {
	case "deploy":
		return deployCommandEnv
	case "remove":
		return removeCommandEnv
	case "invoke":
		return invokeCommandEnv
	case "logs":
		return logsCommandEnv
	default:
		return ""
	}
}
