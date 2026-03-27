package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type generatePluginOpts struct {
	Dir               string
	Module            string
	Version           string
	PluginVersion     string
	WithObservability bool
	NoInteractive     bool
	Interactive       bool
	DryRun            bool
	Force             bool
}

func newGeneratePluginCmd(opts *GlobalOptions) *cobra.Command {
	o := &generatePluginOpts{}
	cmd := &cobra.Command{
		Use:   "plugin [provider-id]",
		Short: "Generate a provider plugin boilerplate using Go SDK",
		Long:  "Scaffolds a standalone external provider plugin project with plugin.yaml, go.mod, main.go, and README. Generated methods align with the provider external adapter contract (Handshake, Doctor, Plan, Deploy, Remove, Invoke, Logs).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerID := ""
			if len(args) > 0 {
				providerID = strings.TrimSpace(args[0])
			}
			return runGeneratePlugin(opts, o, providerID)
		},
	}
	cmd.Flags().StringVar(&o.Dir, "dir", "", "Output directory (default: <provider-id>-provider-plugin)")
	cmd.Flags().StringVar(&o.Module, "module", "", "Go module path (default: github.com/example/<provider-id>-provider-plugin)")
	cmd.Flags().StringVar(&o.Version, "version", "0.1.0", "Plugin version written to plugin.yaml")
	cmd.Flags().StringVar(&o.PluginVersion, "plugin-version", "1", "Provider contract version written to plugin.yaml")
	cmd.Flags().BoolVar(&o.WithObservability, "with-observability", false, "Include FetchMetrics and FetchTraces metadata/stubs in generated plugin")
	cmd.Flags().BoolVar(&o.Interactive, "interactive", false, "Prompt for missing inputs")
	cmd.Flags().BoolVar(&o.NoInteractive, "no-interactive", false, "Disable prompts and require explicit args/flags")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Preview files and directories without writing")
	cmd.Flags().BoolVar(&o.Force, "force", false, "Overwrite files if they already exist")
	return cmd
}

func runGeneratePlugin(gopts *GlobalOptions, o *generatePluginOpts, providerID string) error {
	interactive, err := resolveGenerateInteractivity(gopts, o.Interactive, o.NoInteractive)
	if err != nil {
		return err
	}
	if interactive {
		providerID = promptGenerateName("Provider plugin", providerID, "acme-provider")
	}
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	if providerID == "" {
		return fmt.Errorf("provider plugin id is required (e.g. runfabric generate plugin acme-provider)")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(providerID) {
		return fmt.Errorf("invalid provider plugin id %q: use lowercase letters, numbers, and hyphen", providerID)
	}

	outDir := strings.TrimSpace(o.Dir)
	if outDir == "" {
		outDir = providerID + "-provider-plugin"
	}
	module := strings.TrimSpace(o.Module)
	if module == "" {
		module = "github.com/example/" + providerID + "-provider-plugin"
	}
	version := strings.TrimSpace(o.Version)
	if version == "" {
		version = "0.1.0"
	}
	pluginVersion := strings.TrimSpace(o.PluginVersion)
	if pluginVersion == "" {
		pluginVersion = "1"
	}
	mainPath := filepath.Join(outDir, "main.go")
	pluginYAMLPath := filepath.Join(outDir, "plugin.yaml")
	goModPath := filepath.Join(outDir, "go.mod")
	readmePath := filepath.Join(outDir, "README.md")

	if o.DryRun {
		fmt.Fprintf(os.Stdout, "Dry-run: would create plugin scaffold in %s\n", outDir)
		fmt.Fprintf(os.Stdout, "  - %s\n", mainPath)
		fmt.Fprintf(os.Stdout, "  - %s\n", pluginYAMLPath)
		fmt.Fprintf(os.Stdout, "  - %s\n", goModPath)
		fmt.Fprintf(os.Stdout, "  - %s\n", readmePath)
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writeScaffoldFile(pluginYAMLPath, generatePluginYAML(providerID, version, pluginVersion, o.WithObservability), o.Force); err != nil {
		return err
	}
	if err := writeScaffoldFile(goModPath, generatePluginGoMod(module, outDir), o.Force); err != nil {
		return err
	}
	if err := writeScaffoldFile(mainPath, generatePluginMain(providerID, o.WithObservability), o.Force); err != nil {
		return err
	}
	if err := writeScaffoldFile(readmePath, generatePluginREADME(providerID, o.WithObservability), o.Force); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Created provider plugin scaffold in %s\n", outDir)
	fmt.Fprintf(os.Stdout, "Next: cd %s && go mod tidy && go build -o bin/%s-plugin .\n", outDir, providerID)
	return nil
}

func writeScaffoldFile(path, content string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func generatePluginYAML(providerID, version, pluginVersion string, withObservability bool) string {
	capabilities := []string{"doctor", "plan", "deploy", "remove", "invoke", "logs"}
	if withObservability {
		capabilities = append(capabilities, "observability")
	}
	capabilityYAML := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		capabilityYAML = append(capabilityYAML, "  - "+capability)
	}
	return fmt.Sprintf(`apiVersion: runfabric/v1
kind: provider
id: %s
name: %s
description: External provider plugin scaffold generated by runfabric generate plugin
version: %s
pluginVersion: %s
executable: ./bin/%s-plugin
capabilities:
%s
supportsRuntime:
  - nodejs
  - python
supportsTriggers:
  - http
supportsResources: []
permissions:
  fs: true
  env: true
  network: true
  cloud: true
`, providerID, providerID, version, pluginVersion, providerID, strings.Join(capabilityYAML, "\n"))
}

func generatePluginGoMod(module, outDir string) string {
	replacePath := "../packages/go/plugin-sdk"
	if rel, err := filepath.Rel(outDir, filepath.Join("packages", "go", "plugin-sdk")); err == nil && strings.TrimSpace(rel) != "" {
		replacePath = filepath.ToSlash(rel)
	}
	return fmt.Sprintf(`module %s

go 1.25.0

require github.com/runfabric/runfabric/plugin-sdk/go v0.0.0

replace github.com/runfabric/runfabric/plugin-sdk/go => %s
`, module, replacePath)
}

func generatePluginMain(providerID string, withObservability bool) string {
	providerEnvPrefix := strings.ToUpper(strings.ReplaceAll(providerID, "-", "_"))
	observabilityMethods := ""
	if withObservability {
		observabilityMethods = `

func (p *plugin) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	_ = ctx
	_ = req
	return &sdkprovider.MetricsResult{
		PerFunction: map[string]any{},
		Message:     p.provider + ": replace scaffold FetchMetrics implementation",
	}, nil
}

func (p *plugin) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	_ = ctx
	_ = req
	return &sdkprovider.TracesResult{
		Traces:  []any{},
		Message: p.provider + ": replace scaffold FetchTraces implementation",
	}, nil
}
`
	}
	template := `package main

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
	defaultTokenEnv  = "__PROVIDER_ENV_PREFIX___TOKEN"
	defaultCLIBinEnv = "__PROVIDER_ENV_PREFIX___CLI_BIN"
	deployCommandEnv = "__PROVIDER_ENV_PREFIX___DEPLOY_CMD"
	removeCommandEnv = "__PROVIDER_ENV_PREFIX___REMOVE_CMD"
	invokeCommandEnv = "__PROVIDER_ENV_PREFIX___INVOKE_CMD"
	logsCommandEnv   = "__PROVIDER_ENV_PREFIX___LOGS_CMD"
)

type commandRunner func(ctx context.Context, cwd, command string, env []string) ([]byte, error)

type plugin struct {
	provider      string
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
	InvokeURL string
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
		provider: "__PROVIDER_ID__",
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
		SupportsTriggers: []string{"http"},
		SupportsResources: []string{},
	}
}

func (p *plugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	_ = ctx
	_, err := inspectConfig(req.Config)
	return err
}

func (p *plugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	_ = ctx
	functions, err := inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	checks := []string{
		"scaffold plugin loaded",
		fmt.Sprintf("functions discovered: %d", len(functions)),
		fmt.Sprintf("deploy command configured: %t", p.resolveCommand(req.Config, "deploy") != ""),
		fmt.Sprintf("remove command configured: %t", p.resolveCommand(req.Config, "remove") != ""),
		fmt.Sprintf("invoke command configured: %t", p.resolveCommand(req.Config, "invoke") != ""),
		fmt.Sprintf("logs command configured: %t", p.resolveCommand(req.Config, "logs") != ""),
	}
	return &sdkprovider.DoctorResult{Provider: p.provider, Checks: checks}, nil
}

func (p *plugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	_ = ctx
	functions, err := inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	actions := make([]map[string]any, 0, len(functions))
	warnings := make([]string, 0)
	for _, fn := range functions {
		actions = append(actions, map[string]any{
			"type":     "deploy-function",
			"function": fn.Name,
			"runtime":  fn.Runtime,
			"entry":    fn.Entry,
			"artifact": fn.Artifact,
		})
		if strings.TrimSpace(fn.Artifact) == "" {
			warnings = append(warnings, fmt.Sprintf("function %s has no artifact configured", fn.Name))
		}
	}
	return &sdkprovider.PlanResult{Provider: p.provider, Plan: map[string]any{"provider": p.provider, "actions": actions}, Warnings: warnings}, nil
}

func (p *plugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	out, err := p.executeOperation(ctx, req.Root, req.Config, "deploy", req.Stage, "", nil)
	if err != nil {
		return nil, err
	}
	return &sdkprovider.DeployResult{Provider: p.provider, DeploymentID: fmt.Sprintf("%s-%d", p.provider, p.deploymentNow().Unix()), Outputs: map[string]string{"stdout": strings.TrimSpace(string(out))}}, nil
}

func (p *plugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	if _, err := p.executeOperation(ctx, req.Root, req.Config, "remove", req.Stage, "", nil); err != nil {
		return nil, err
	}
	return &sdkprovider.RemoveResult{Provider: p.provider, Removed: true}, nil
}

func (p *plugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	functions, err := inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	functionName := resolveFunctionName(functions, req.Function)
	if invokeURL := resolveInvokeURL(functions, functionName); invokeURL != "" {
		return p.invokeHTTP(ctx, invokeURL, functionName, req.Payload)
	}
	out, err := p.executeOperation(ctx, "", req.Config, "invoke", req.Stage, functionName, req.Payload)
	if err != nil {
		return nil, err
	}
	return &sdkprovider.InvokeResult{Provider: p.provider, Function: functionName, Output: strings.TrimSpace(string(out))}, nil
}

func (p *plugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	functions, err := inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	functionName := resolveFunctionName(functions, req.Function)
	out, err := p.executeOperation(ctx, "", req.Config, "logs", req.Stage, functionName, nil)
	if err != nil {
		return nil, err
	}
	return &sdkprovider.LogsResult{Provider: p.provider, Function: functionName, Lines: splitOutputLines(out)}, nil
}

func inspectConfig(cfg sdkprovider.Config) ([]functionSpec, error) {
	service := strings.TrimSpace(asString(cfg["service"]))
	if service == "" {
		service = "provider-service"
	}
	entries, ok := cfg["functions"].([]any)
	if !ok || len(entries) == 0 {
		return []functionSpec{{Name: service, Runtime: normalizeRuntime(asString(cfg["runtime"])), Entry: strings.TrimSpace(asString(cfg["entry"])), Artifact: firstNonEmpty(strings.TrimSpace(asString(cfg["artifact"])), strings.TrimSpace(asString(cfg["outputPath"])), pathJoin("dist", service+".zip"))}}, nil
	}
	functions := make([]functionSpec, 0, len(entries))
	for index, raw := range entries {
		fn, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config.functions[%d] must be an object", index)
		}
		name := strings.TrimSpace(asString(fn["name"]))
		if name == "" {
			return nil, fmt.Errorf("config.functions[%d].name is required", index)
		}
		functions = append(functions, functionSpec{Name: name, Runtime: normalizeRuntime(firstNonEmpty(asString(fn["runtime"]), asString(cfg["runtime"]))), Entry: firstNonEmpty(strings.TrimSpace(asString(fn["entry"])), strings.TrimSpace(asString(cfg["entry"]))), Artifact: firstNonEmpty(strings.TrimSpace(asString(fn["artifact"])), strings.TrimSpace(asString(fn["outputPath"])), pathJoin("dist", name+".zip")), InvokeURL: firstNonEmpty(strings.TrimSpace(asString(fn["invokeUrl"])), strings.TrimSpace(asString(fn["url"])))})
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].Name < functions[j].Name })
	return functions, nil
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
	return p.defaultProviderCLICommand(cfg, operation)
}

func (p *plugin) defaultProviderCLICommand(cfg sdkprovider.Config, operation string) string {
	_ = cfg
	_ = operation
	return ""
}

func (p *plugin) executeOperation(ctx context.Context, root string, cfg sdkprovider.Config, operation, stage, function string, payload []byte) ([]byte, error) {
	command := p.resolveCommand(cfg, operation)
	if command == "" {
		return nil, fmt.Errorf("no %s command configured: set %s or config.commands.%s", operation, commandEnvForOperation(operation), operation)
	}
	functions, err := inspectConfig(cfg)
	if err != nil {
		return nil, err
	}
	selectedFunction := resolveFunctionName(functions, function)
	artifactPath := resolveArtifactPath(root, functions, selectedFunction)
	env := append(os.Environ(), "RUNFABRIC_PROVIDER="+p.provider, "RUNFABRIC_STAGE="+stage, "RUNFABRIC_ROOT="+root, "RUNFABRIC_FUNCTION="+selectedFunction, "RUNFABRIC_ARTIFACT_PATH="+artifactPath, "RUNFABRIC_ARTIFACT_DIR="+pathDir(artifactPath), "RUNFABRIC_ARTIFACT_BASENAME="+pathBase(artifactPath), "RUNFABRIC_PAYLOAD_BASE64="+base64.StdEncoding.EncodeToString(payload))
	if token := strings.TrimSpace(p.getenv(defaultTokenEnv)); token != "" {
		env = append(env, defaultTokenEnv+"="+token)
	}
	return p.runCommand(ctx, root, command, env)
}

func (p *plugin) invokeHTTP(ctx context.Context, url, function string, payload []byte) (*sdkprovider.InvokeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if json.Valid(payload) || len(payload) == 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return &sdkprovider.InvokeResult{Provider: p.provider, Function: function, Output: strings.TrimSpace(string(body))}, nil
}

func defaultCommandRunner(ctx context.Context, cwd, command string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Env = env
	return cmd.CombinedOutput()
}

func resolveFunctionName(functions []functionSpec, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	if len(functions) == 1 {
		return functions[0].Name
	}
	return ""
}

func resolveInvokeURL(functions []functionSpec, function string) string {
	for _, fn := range functions {
		if function != "" && fn.Name != function {
			continue
		}
		if strings.TrimSpace(fn.InvokeURL) != "" {
			return fn.InvokeURL
		}
	}
	return ""
}

func resolveArtifactPath(root string, functions []functionSpec, function string) string {
	for _, fn := range functions {
		if function != "" && fn.Name != function {
			continue
		}
		return joinRoot(root, fn.Artifact)
	}
	return ""
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

func splitOutputLines(out []byte) []string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return []string{"no output"}
	}
	return strings.Split(trimmed, "\n")
}

func joinRoot(root, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") || strings.TrimSpace(root) == "" {
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
__OBSERVABILITY_METHODS__
`
	return strings.NewReplacer(
		"__PROVIDER_ID__", providerID,
		"__PROVIDER_ENV_PREFIX__", providerEnvPrefix,
		"__OBSERVABILITY_METHODS__", observabilityMethods,
	).Replace(template)
}

func generatePluginREADME(providerID string, withObservability bool) string {
	providerEnvPrefix := strings.ToUpper(strings.ReplaceAll(providerID, "-", "_"))
	observabilityNote := ""
	if withObservability {
		observabilityNote = "\n- `FetchMetrics`\n- `FetchTraces`\n"
	}
	template := `# __PROVIDER_ID__ provider plugin

Generated by ` + "`runfabric generate plugin`" + `.

## Build

` + "```bash\ngo mod tidy\ngo build -o bin/%s-plugin .\n```" + `

## Install (local)

Copy this folder to:

` + "```text\n$RUNFABRIC_HOME/plugins/provider/%s/0.1.0/\n```" + `

Make sure ` + "`plugin.yaml`" + ` executable points to ` + "`./bin/%s-plugin`" + `.

## Contract

This scaffold implements methods expected by the external provider adapter:

- Import SDK contracts only (` + "`github.com/runfabric/runfabric/plugin-sdk/go/provider`" + `)
- Do not import ` + "`github.com/runfabric/runfabric/platform/...`" + ` from extension/provider/runtime/simulator plugins

- SDK-managed ` + "`Handshake`" + ` metadata via ` + "`server.HandshakeMetadata`" + `
- ` + "`Doctor`" + `
- ` + "`Plan`" + `
- ` + "`Deploy`" + `
- ` + "`Remove`" + `
- ` + "`Invoke`" + `
- ` + "`Logs`" + `
__OBSERVABILITY_NOTE__

## Scaffold Conventions

- Token env defaults to ` + "`__PROVIDER_ENV_PREFIX___TOKEN`" + `.
- Command env defaults are ` + "`__PROVIDER_ENV_PREFIX___DEPLOY_CMD`" + `, ` + "`__PROVIDER_ENV_PREFIX___REMOVE_CMD`" + `, ` + "`__PROVIDER_ENV_PREFIX___INVOKE_CMD`" + `, and ` + "`__PROVIDER_ENV_PREFIX___LOGS_CMD`" + `.
- Optional CLI binary override env is ` + "`__PROVIDER_ENV_PREFIX___CLI_BIN`" + `.
- Command resolution order is ` + "`config.commands.*`" + ` -> ` + "`<operation>Command`" + ` -> env var -> ` + "`defaultProviderCLICommand`" + `.
- Artifact resolution starts from ` + "`functions[].artifact`" + ` or ` + "`functions[].outputPath`" + ` and should be standardized to a zip per function.

## Next Step

Implement ` + "`defaultProviderCLICommand`" + ` with provider-specific fallbacks when the provider CLI/API shape is stable.
`
	return strings.NewReplacer(
		"__PROVIDER_ID__", providerID,
		"__PROVIDER_ENV_PREFIX__", providerEnvPrefix,
		"__OBSERVABILITY_NOTE__", observabilityNote,
	).Replace(template)
}
