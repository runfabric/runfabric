package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const (
	defaultLinodeAPIBaseURL = "https://api.linode.com/v4"
	defaultTokenEnv         = "LINODE_TOKEN"
	defaultCLIBinEnv        = "LINODE_CLI_BIN"
	deployCommandEnv        = "LINODE_DEPLOY_CMD"
	removeCommandEnv        = "LINODE_REMOVE_CMD"
	invokeCommandEnv        = "LINODE_INVOKE_CMD"
	logsCommandEnv          = "LINODE_LOGS_CMD"
)

type commandRunner func(ctx context.Context, cwd, command string, env []string) ([]byte, error)

type plugin struct {
	provider      string
	apiBaseURL    string
	httpClient    *http.Client
	runCommand    commandRunner
	getenv        func(string) string
	deploymentNow func() time.Time
}

type functionSpec struct {
	Name      string
	Runtime   string
	Entry     string
	Artifact  string
	Triggers  []string
	InvokeURL string
}

type linodeProfile struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func main() {
	p := newPlugin()
	s := sdkprovider.NewServer(p, sdkprovider.ServeOptions{ProtocolVersion: "1"})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newPlugin() *plugin {
	return &plugin{
		provider:   "linode",
		apiBaseURL: defaultLinodeAPIBaseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		runCommand: defaultCommandRunner,
		getenv:     os.Getenv,
		deploymentNow: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (p *plugin) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:            p.provider,
		Version:         "0.1.0",
		PluginVersion:   "1",
		SupportsRuntime: []string{"nodejs", "python"},
		SupportsTriggers: []string{
			"http",
		},
		SupportsResources: []string{},
	}
}

func (p *plugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	_ = ctx
	_, _, _, err := p.inspectConfig(req.Config)
	return err
}

func (p *plugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	service, functions, warnings, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	token, tokenSource := p.resolveToken(req.Config)
	if token == "" {
		return nil, fmt.Errorf("missing Linode token: set %s or config.token/config.tokenEnv", defaultTokenEnv)
	}
	profile, err := p.fetchProfile(ctx, token)
	if err != nil {
		return nil, err
	}
	checks := []string{
		fmt.Sprintf("authenticated to Linode API as %s", firstNonEmpty(profile.Username, profile.Email)),
		fmt.Sprintf("service: %s", service),
		fmt.Sprintf("functions discovered: %d", len(functions)),
		fmt.Sprintf("token source: %s", tokenSource),
	}
	for _, op := range []string{"deploy", "remove", "invoke", "logs"} {
		cmd := strings.TrimSpace(p.resolveCommand(req.Config, op))
		if cmd == "" {
			checks = append(checks, fmt.Sprintf("%s command not configured", op))
			continue
		}
		checks = append(checks, fmt.Sprintf("%s command configured", op))
	}
	checks = append(checks, warnings...)
	return &sdkprovider.DoctorResult{Provider: p.provider, Checks: checks}, nil
}

func (p *plugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	_ = ctx
	service, functions, warnings, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	actions := make([]map[string]any, 0, len(functions))
	for _, fn := range functions {
		actions = append(actions, map[string]any{
			"type":     "deploy-function",
			"function": fn.Name,
			"runtime":  fn.Runtime,
			"entry":    fn.Entry,
			"artifact": fn.Artifact,
			"triggers": append([]string(nil), fn.Triggers...),
		})
	}
	plan := map[string]any{
		"provider": p.provider,
		"service":  service,
		"stage":    req.Stage,
		"root":     req.Root,
		"actions":  actions,
		"commands": map[string]bool{
			"deploy": p.resolveCommand(req.Config, "deploy") != "",
			"remove": p.resolveCommand(req.Config, "remove") != "",
			"invoke": p.resolveCommand(req.Config, "invoke") != "" || p.resolveInvokeURL(req.Config, "") != "",
			"logs":   p.resolveCommand(req.Config, "logs") != "",
		},
	}
	if token, _ := p.resolveToken(req.Config); token == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.token before running doctor", defaultTokenEnv))
	}
	if p.resolveCommand(req.Config, "deploy") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.deploy to enable deployments", deployCommandEnv))
	}
	for _, fn := range functions {
		if strings.TrimSpace(fn.Artifact) == "" {
			warnings = append(warnings, fmt.Sprintf("function %s has no artifact configured; set artifact/outputPath or place a zip in dist/, build/, or .runfabric/", fn.Name))
		}
	}
	if p.resolveCommand(req.Config, "remove") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.remove to enable removals", removeCommandEnv))
	}
	if p.resolveCommand(req.Config, "logs") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s or config.commands.logs to enable log collection", logsCommandEnv))
	}
	if p.resolveCommand(req.Config, "invoke") == "" && p.resolveInvokeURL(req.Config, "") == "" {
		warnings = append(warnings, fmt.Sprintf("set %s, config.commands.invoke, or a function URL to enable invocation", invokeCommandEnv))
	}
	return &sdkprovider.PlanResult{Provider: p.provider, Plan: plan, Warnings: warnings}, nil
}

func (p *plugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	service, functions, _, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	out, err := p.executeOperation(ctx, req.Root, req.Config, "deploy", req.Stage, "", nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseDeployResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.DeploymentID == "" {
			parsed.DeploymentID = p.defaultDeploymentID(service, req.Stage)
		}
		return parsed, nil
	}
	result := &sdkprovider.DeployResult{
		Provider:     p.provider,
		DeploymentID: p.defaultDeploymentID(service, req.Stage),
		Outputs: map[string]string{
			"stdout": strings.TrimSpace(string(out)),
		},
		Metadata: map[string]string{
			"mode":    "command",
			"service": service,
			"stage":   req.Stage,
		},
		Functions: make(map[string]sdkprovider.DeployedFunction, len(functions)),
	}
	for _, fn := range functions {
		result.Functions[fn.Name] = sdkprovider.DeployedFunction{ResourceName: fn.Name}
	}
	return result, nil
}

func (p *plugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	out, err := p.executeOperation(ctx, req.Root, req.Config, "remove", req.Stage, "", nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseRemoveResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		return parsed, nil
	}
	return &sdkprovider.RemoveResult{Provider: p.provider, Removed: true}, nil
}

func (p *plugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	functionName := p.resolveFunctionName(req.Config, req.Function)
	if invokeURL := p.resolveInvokeURL(req.Config, functionName); invokeURL != "" {
		return p.invokeHTTP(ctx, invokeURL, functionName, req.Payload)
	}
	out, err := p.executeOperation(ctx, "", req.Config, "invoke", req.Stage, functionName, req.Payload)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseInvokeResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.Function == "" {
			parsed.Function = functionName
		}
		return parsed, nil
	}
	return &sdkprovider.InvokeResult{
		Provider: p.provider,
		Function: functionName,
		Output:   strings.TrimSpace(string(out)),
	}, nil
}

func (p *plugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	functionName := p.resolveFunctionName(req.Config, req.Function)
	out, err := p.executeOperation(ctx, "", req.Config, "logs", req.Stage, functionName, nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseLogsResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.Function == "" {
			parsed.Function = functionName
		}
		return parsed, nil
	}
	lines := splitOutputLines(out)
	if len(lines) == 0 {
		lines = []string{"no log output"}
	}
	return &sdkprovider.LogsResult{Provider: p.provider, Function: functionName, Lines: lines}, nil
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

func (p *plugin) fetchProfile(ctx context.Context, token string) (*linodeProfile, error) {
	url := strings.TrimSuffix(p.apiBaseURL, "/") + "/profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Linode profile API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Linode profile API returned %s: %s", resp.Status, parseAPIError(body))
	}
	var profile linodeProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("decode Linode profile response: %w", err)
	}
	return &profile, nil
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
	return p.defaultLinodeCLICommand(cfg, operation)
}

func (p *plugin) defaultLinodeCLICommand(cfg sdkprovider.Config, operation string) string {
	cliBin := shellQuote(p.resolveCLIBin(cfg))
	switch operation {
	case "deploy":
		if strings.TrimSpace(asString(cfg["appID"])) == "" {
			return ""
		}
		return cliBin + ` functions action-create "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION" --app-id "$RUNFABRIC_LINODE_APP_ID" --runtime "$RUNFABRIC_RUNTIME" --file "$RUNFABRIC_ARTIFACT_PATH"`
	case "remove":
		return cliBin + ` functions action-delete "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION"`
	case "logs":
		return cliBin + ` functions activation-list "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION"`
	default:
		return ""
	}
}

func (p *plugin) resolveCLIBin(cfg sdkprovider.Config) string {
	if bin := strings.TrimSpace(asString(cfg["cliBin"])); bin != "" {
		return bin
	}
	if bin := strings.TrimSpace(p.getenv(defaultCLIBinEnv)); bin != "" {
		return bin
	}
	return "linode-cli"
}

func (p *plugin) executeOperation(ctx context.Context, root string, cfg sdkprovider.Config, operation, stage, function string, payload []byte) ([]byte, error) {
	command := p.resolveCommand(cfg, operation)
	if command == "" {
		return nil, fmt.Errorf("no %s command configured: set %s or config.commands.%s", operation, commandEnvForOperation(operation), operation)
	}
	service, functions, _, err := p.inspectConfig(cfg)
	if err != nil {
		return nil, err
	}
	selectedFunction := function
	selectedSpec := functionSpec{}
	if selectedFunction == "" && len(functions) == 1 {
		selectedFunction = functions[0].Name
	}
	for _, fn := range functions {
		if fn.Name == selectedFunction {
			selectedSpec = fn
			break
		}
	}
	artifactPath := p.resolveArtifactPath(root, selectedSpec)
	env := append(os.Environ(),
		"RUNFABRIC_PROVIDER="+p.provider,
		"RUNFABRIC_SERVICE="+service,
		"RUNFABRIC_STAGE="+stage,
		"RUNFABRIC_ROOT="+root,
		"RUNFABRIC_FUNCTION="+selectedFunction,
		"RUNFABRIC_RUNTIME="+selectedSpec.Runtime,
		"RUNFABRIC_ENTRY="+selectedSpec.Entry,
		"RUNFABRIC_ARTIFACT_PATH="+artifactPath,
		"RUNFABRIC_ARTIFACT_DIR="+pathDir(artifactPath),
		"RUNFABRIC_ARTIFACT_BASENAME="+pathBase(artifactPath),
		"RUNFABRIC_PAYLOAD_BASE64="+base64.StdEncoding.EncodeToString(payload),
	)
	if token, _ := p.resolveToken(cfg); token != "" {
		env = append(env, "LINODE_TOKEN="+token)
	}
	if appID := strings.TrimSpace(asString(cfg["appID"])); appID != "" {
		env = append(env, "RUNFABRIC_LINODE_APP_ID="+appID)
	}
	out, err := p.runCommand(ctx, root, command, env)
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return nil, fmt.Errorf("%s command failed: %w", operation, err)
		}
		return nil, fmt.Errorf("%s command failed: %w: %s", operation, err, trimmed)
	}
	return out, nil
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

func (p *plugin) invokeHTTP(ctx context.Context, url, function string, payload []byte) (*sdkprovider.InvokeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	contentType := "application/octet-stream"
	if json.Valid(payload) || len(payload) == 0 {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke function over HTTP: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	output := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &sdkprovider.InvokeResult{Provider: p.provider, Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, output)}, nil
	}
	return &sdkprovider.InvokeResult{
		Provider: p.provider,
		Function: function,
		Output:   output,
		RunID:    strings.TrimSpace(resp.Header.Get("X-Request-Id")),
	}, nil
}

func (p *plugin) defaultDeploymentID(service, stage string) string {
	return fmt.Sprintf("linode-%s-%s-%d", service, stage, p.deploymentNow().Unix())
}

func defaultCommandRunner(ctx context.Context, cwd, command string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Env = env
	return cmd.CombinedOutput()
}

func parseDeployResult(out []byte) (*sdkprovider.DeployResult, bool) {
	var result sdkprovider.DeployResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.DeploymentID == "" && len(result.Outputs) == 0 && len(result.Metadata) == 0 && len(result.Functions) == 0 && len(result.Artifacts) == 0 {
		return nil, false
	}
	return &result, true
}

func parseRemoveResult(out []byte) (*sdkprovider.RemoveResult, bool) {
	var result sdkprovider.RemoveResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.Provider == "" && !result.Removed {
		return nil, false
	}
	return &result, true
}

func parseInvokeResult(out []byte) (*sdkprovider.InvokeResult, bool) {
	var result sdkprovider.InvokeResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.Output == "" && result.RunID == "" && result.Workflow == "" {
		return nil, false
	}
	return &result, true
}

func parseLogsResult(out []byte) (*sdkprovider.LogsResult, bool) {
	var result sdkprovider.LogsResult
	if json.Unmarshal(out, &result) == nil && len(result.Lines) > 0 {
		return &result, true
	}
	var lines []string
	if json.Unmarshal(out, &lines) == nil && len(lines) > 0 {
		return &sdkprovider.LogsResult{Lines: lines}, true
	}
	return nil, false
}

func parseAPIError(body []byte) string {
	var payload struct {
		Errors []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &payload) == nil && len(payload.Errors) > 0 {
		parts := make([]string, 0, len(payload.Errors))
		for _, err := range payload.Errors {
			if strings.TrimSpace(err.Reason) != "" {
				parts = append(parts, strings.TrimSpace(err.Reason))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return strings.TrimSpace(string(body))
}

func splitOutputLines(out []byte) []string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	rawLines := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
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

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (p *plugin) resolveArtifactPath(root string, fn functionSpec) string {
	if strings.TrimSpace(fn.Artifact) != "" {
		return joinRoot(root, fn.Artifact)
	}
	if strings.TrimSpace(fn.Name) == "" {
		return ""
	}
	for _, candidate := range []string{
		pathJoin(root, ".runfabric", fn.Name+".zip"),
		pathJoin(root, "dist", fn.Name+".zip"),
		pathJoin(root, "build", fn.Name+".zip"),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func joinRoot(root, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	if strings.TrimSpace(root) == "" {
		return trimmed
	}
	return pathJoin(root, trimmed)
}

func pathJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, strings.Trim(part, "/"))
	}
	if len(filtered) == 0 {
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(parts[0]), "/") {
		return "/" + strings.Join(filtered, "/")
	}
	return strings.Join(filtered, "/")
}

func pathDir(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index < 0 {
		return ""
	}
	return trimmed[:index]
}

func pathBase(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index < 0 {
		return trimmed
	}
	return trimmed[index+1:]
}
