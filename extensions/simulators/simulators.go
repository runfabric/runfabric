package simulators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
	sdksimulator "github.com/runfabric/runfabric/plugin-sdk/go/simulator"
)

type Registry struct {
	mu         sync.RWMutex
	simulators map[string]sdksimulator.Plugin
}

// NewLocalSimulator returns the built-in local simulator plugin.
func NewLocalSimulator() sdksimulator.Plugin {
	return localSimulator{}
}

func NewRegistry() *Registry {
	return &Registry{simulators: map[string]sdksimulator.Plugin{}}
}

func (r *Registry) Register(sim sdksimulator.Plugin) error {
	if sim == nil {
		return fmt.Errorf("simulator plugin is nil")
	}
	id := strings.TrimSpace(sim.Meta().ID)
	if id == "" {
		return fmt.Errorf("simulator plugin id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.simulators[id] = sim
	return nil
}

func (r *Registry) Get(id string) (sdksimulator.Plugin, error) {
	id = strings.TrimSpace(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	sim, ok := r.simulators[id]
	if !ok {
		return nil, fmt.Errorf("simulator plugin %q is not registered", id)
	}
	return sim, nil
}

func (r *Registry) List() []sdkrouter.PluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdkrouter.PluginMeta, 0, len(r.simulators))
	for _, sim := range r.simulators {
		out = append(out, sim.Meta())
	}
	return out
}

// nodeRunner is an inline Node.js script executed per-request to invoke the
// handler function. It reads execution parameters from environment variables
// and writes the handler result as JSON to stdout.
const nodeRunner = `
const path = require('path');
(async () => {
	const handlerRef = process.env.RF_HANDLER;
	const workDir    = process.env.RF_WORKDIR;
	const event      = JSON.parse(process.env.RF_EVENT || '{}');
	const lastDot    = handlerRef.lastIndexOf('.');
	const modRelPath = lastDot > 0 ? handlerRef.slice(0, lastDot) : handlerRef;
	const fnName     = lastDot > 0 ? handlerRef.slice(lastDot + 1) : 'handler';
	const modPath = path.resolve(workDir || process.cwd(), modRelPath);
	const mod = require(modPath);
	const fn  = mod[fnName] || (mod.default && mod.default[fnName]);
	if (typeof fn !== 'function') {
		process.stderr.write('handler "' + fnName + '" not exported from ' + modRelPath + '\n');
		process.exit(1);
	}
	const result = await fn(event, {});
	process.stdout.write(JSON.stringify(result));
})().catch(e => {
	process.stderr.write(String(e.stack || e) + '\n');
	process.exit(1);
});
`

type localSimulator struct{}

func (s localSimulator) Meta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "local",
		Name:        "Local Simulator",
		Description: "Built-in local simulator for call-local/dev workflows",
	}
}

func (s localSimulator) Simulate(_ context.Context, req sdksimulator.Request) (*sdksimulator.Response, error) {
	ctx := context.Background()
	if req.WorkDir != "" && req.HandlerRef != "" && isNodeRuntime(req.Runtime) {
		return invokeNodeHandler(ctx, req)
	}
	// Fallback: echo the request metadata (used when no handler context is provided).
	body := map[string]any{
		"message":  "invoke local",
		"service":  req.Service,
		"stage":    req.Stage,
		"function": req.Function,
		"method":   req.Method,
		"path":     req.Path,
	}
	if len(req.Query) > 0 {
		body["query"] = req.Query
	}
	if len(req.Headers) > 0 {
		body["headers"] = req.Headers
	}
	if len(req.Body) > 0 {
		body["body"] = string(req.Body)
	}
	raw, _ := json.Marshal(body)
	return &sdksimulator.Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: raw,
	}, nil
}

func isNodeRuntime(runtime string) bool {
	return strings.HasPrefix(strings.ToLower(runtime), "node")
}

// invokeSandboxMode returns the isolation mode for the local simulator's handler
// execution. "docker" runs the handler inside a locked-down container instead of
// the host/daemon process — enabled by RUNFABRIC_INVOKE_SANDBOX=docker on hosted
// daemons (requires a docker runtime). Empty (default) runs in-process, which is
// what the CLI dev/call-local tooling wants on a developer's own machine.
func invokeSandboxMode() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("RUNFABRIC_INVOKE_SANDBOX")))
}

func invokeSandboxImage() string {
	if v := strings.TrimSpace(os.Getenv("RUNFABRIC_INVOKE_SANDBOX_IMAGE")); v != "" {
		return v
	}
	// Same default image family as the build sandbox; must have node on PATH.
	return "public.ecr.aws/docker/library/node:20-bullseye"
}

// dockerInvokeArgs builds the `docker run` argv that runs the node runner in a
// hardened container: no network, read-only root fs, all capabilities dropped,
// no-new-privileges, cpu/memory/pids caps, a small exec-capable tmpfs, and ONLY
// the one job's workDir bind-mounted (read-only) at /work. containerEnv carries
// RF_EVENT/RF_HANDLER/RF_WORKDIR into the container via -e (RF_WORKDIR must be
// the in-container path, /work). This is the same isolation posture as the build
// sandbox, applied to invoke-local so tenant handler code never runs unconfined
// in the daemon process.
func dockerInvokeArgs(workDir, image string, containerEnv []string) []string {
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "256",
		"--memory", "512m",
		"--cpus", "1",
		"--tmpfs", "/tmp:rw,exec,nosuid,size=64m",
		"-v", workDir + ":/work:ro",
		"-w", "/work",
	}
	for _, e := range containerEnv {
		args = append(args, "-e", e)
	}
	args = append(args, image, "node", "-e", nodeRunner)
	return args
}

// invokeNodeHandler spawns a Node.js process, calls the exported handler
// function with a Lambda-compatible HTTP event, and returns the response. When
// RUNFABRIC_INVOKE_SANDBOX=docker, the node process runs inside a hardened
// container (see dockerInvokeArgs) instead of directly on the host.
func invokeNodeHandler(ctx context.Context, req sdksimulator.Request) (*sdksimulator.Response, error) {
	event := map[string]any{
		"httpMethod":            req.Method,
		"path":                  req.Path,
		"headers":               req.Headers,
		"queryStringParameters": req.Query,
		"body":                  nil,
		"isBase64Encoded":       false,
	}
	if len(req.Body) > 0 {
		event["body"] = string(req.Body)
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	var cmd *exec.Cmd
	if invokeSandboxMode() == "docker" {
		// RF_WORKDIR is the in-container mount point; RF_* ride -e, and the
		// docker CLI itself inherits PATH/DOCKER_HOST from the process env.
		containerEnv := []string{
			"RF_EVENT=" + string(eventJSON),
			"RF_HANDLER=" + req.HandlerRef,
			"RF_WORKDIR=/work",
		}
		cmd = exec.CommandContext(ctx, "docker", dockerInvokeArgs(req.WorkDir, invokeSandboxImage(), containerEnv)...)
		cmd.Env = os.Environ()
	} else {
		cmd = exec.CommandContext(ctx, "node", "-e", nodeRunner)
		cmd.Dir = req.WorkDir
		cmd.Env = append(os.Environ(),
			"RF_EVENT="+string(eventJSON),
			"RF_HANDLER="+req.HandlerRef,
			"RF_WORKDIR="+req.WorkDir,
		)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("handler execution failed: %s", msg)
	}

	var result struct {
		StatusCode int               `json:"statusCode"`
		Headers    map[string]string `json:"headers"`
		Body       string            `json:"body"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("invalid handler response: %w (got: %s)", err, stdout.String())
	}

	status := result.StatusCode
	if status == 0 {
		status = 200
	}
	headers := result.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	var bodyRaw json.RawMessage
	if json.Valid([]byte(result.Body)) {
		bodyRaw = json.RawMessage(result.Body)
	} else {
		bodyRaw, _ = json.Marshal(result.Body)
	}

	return &sdksimulator.Response{
		StatusCode: status,
		Headers:    headers,
		Body:       bodyRaw,
	}, nil
}

// BuiltinSimulatorManifests returns simulator metadata entries used by extension manifest catalogs.
func BuiltinSimulatorManifests() []sdkrouter.PluginMeta {
	return []sdkrouter.PluginMeta{
		{ID: "local", Name: "Local Simulator", Description: "Built-in local simulator for call-local/dev"},
	}
}

func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(NewLocalSimulator())
	return reg
}
