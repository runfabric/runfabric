package runtimes

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
	sdkruntime "github.com/runfabric/runfabric/plugin-sdk/go/runtime"
)

type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]sdkruntime.Plugin
}

func NewRegistry() *Registry {
	return &Registry{runtimes: map[string]sdkruntime.Plugin{}}
}

func (r *Registry) Register(rt sdkruntime.Plugin) error {
	if rt == nil {
		return fmt.Errorf("runtime plugin is nil")
	}
	id := strings.TrimSpace(rt.Meta().ID)
	if id == "" {
		return fmt.Errorf("runtime plugin id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runtimes[id] = rt
	return nil
}

func (r *Registry) Get(runtimeID string) (sdkruntime.Plugin, error) {
	id := NormalizeRuntimeID(runtimeID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	rt, ok := r.runtimes[id]
	if !ok {
		return nil, fmt.Errorf("runtime plugin %q is not registered", strings.TrimSpace(runtimeID))
	}
	return rt, nil
}

func (r *Registry) List() []sdkrouter.PluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdkrouter.PluginMeta, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		out = append(out, rt.Meta())
	}
	return out
}

func NormalizeRuntimeID(runtime string) string {
	raw := strings.ToLower(strings.TrimSpace(runtime))
	switch {
	case strings.HasPrefix(raw, "nodejs"):
		return "nodejs"
	case strings.HasPrefix(raw, "python"):
		return "python"
	default:
		return strings.TrimSpace(runtime)
	}
}

type nodeRuntime struct{}

// NewNodeRuntime returns the built-in Node.js runtime plugin.
func NewNodeRuntime() sdkruntime.Plugin {
	return nodeRuntime{}
}

func (r nodeRuntime) Meta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime plugin (build/invoke)"}
}

func (r nodeRuntime) Build(_ context.Context, req sdkruntime.BuildRequest) (*sdkprovider.Artifact, error) {
	return packageNodeFunction(req.Root, req.FunctionName, req.Function, req.ConfigSignature)
}

func (r nodeRuntime) Invoke(_ context.Context, req sdkruntime.InvokeRequest) (*sdkruntime.InvokeResult, error) {
	out, err := runNodeHandler(req.Root, req.Function.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &sdkruntime.InvokeResult{Output: out}, nil
}

type pythonRuntime struct{}

// NewPythonRuntime returns the built-in Python runtime plugin.
func NewPythonRuntime() sdkruntime.Plugin {
	return pythonRuntime{}
}

func (r pythonRuntime) Meta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{ID: "python", Name: "Python", Description: "Python runtime plugin (build/invoke)"}
}

func (r pythonRuntime) Build(_ context.Context, req sdkruntime.BuildRequest) (*sdkprovider.Artifact, error) {
	return packagePythonFunction(req.Root, req.FunctionName, req.Function, req.ConfigSignature)
}

func (r pythonRuntime) Invoke(_ context.Context, req sdkruntime.InvokeRequest) (*sdkruntime.InvokeResult, error) {
	out, err := runPythonHandler(req.Root, req.Function.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &sdkruntime.InvokeResult{Output: out}, nil
}

// BuiltinRuntimeManifests returns runtime metadata entries used by extension manifest catalogs.
func BuiltinRuntimeManifests() []sdkrouter.PluginMeta {
	return []sdkrouter.PluginMeta{
		{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime (build and invoke)"},
		{ID: "python", Name: "Python", Description: "Python runtime (build and invoke)"},
	}
}

// NewBuiltinRegistry returns a runtime registry pre-populated with built-in Node.js and Python runtimes.
func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(NewNodeRuntime())
	_ = reg.Register(NewPythonRuntime())
	return reg
}

func packageNodeFunction(root, functionName string, fn sdkruntime.FunctionSpec, configSignature string) (*sdkprovider.Artifact, error) {
	handlerFile, archiveName, err := resolveNodeHandlerFile(fn.Handler)
	if err != nil {
		return nil, err
	}
	sourcePath := filepath.Join(root, handlerFile)
	outputPath := filepath.Join(root, ".runfabric", "build", functionName+".zip")
	zipResult, err := zipSingleFile(sourcePath, outputPath, archiveName)
	if err != nil {
		return nil, err
	}
	return &sdkprovider.Artifact{Function: functionName, Runtime: fn.Runtime, SourcePath: sourcePath, OutputPath: zipResult.OutputPath, SHA256: zipResult.SHA256, SizeBytes: zipResult.SizeBytes, ConfigSignature: configSignature}, nil
}

func packagePythonFunction(root, functionName string, fn sdkruntime.FunctionSpec, configSignature string) (*sdkprovider.Artifact, error) {
	handlerFile, archiveName, err := resolvePythonHandlerFile(fn.Handler)
	if err != nil {
		return nil, err
	}
	sourcePath := filepath.Join(root, handlerFile)
	outputPath := filepath.Join(root, ".runfabric", "build", functionName+".zip")
	zipResult, err := zipSingleFile(sourcePath, outputPath, archiveName)
	if err != nil {
		return nil, err
	}
	return &sdkprovider.Artifact{Function: functionName, Runtime: fn.Runtime, SourcePath: sourcePath, OutputPath: zipResult.OutputPath, SHA256: zipResult.SHA256, SizeBytes: zipResult.SizeBytes, ConfigSignature: configSignature}, nil
}

func resolveNodeHandlerFile(handler string) (sourcePath string, archiveName string, err error) {
	if strings.TrimSpace(handler) == "" {
		return "", "", fmt.Errorf("empty handler")
	}
	parts := strings.Split(handler, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid handler %q, expected file.export", handler)
	}
	filePart := parts[0]
	archiveName = filepath.Base(filePart) + ".js"
	return filePart + ".js", archiveName, nil
}

func resolvePythonHandlerFile(handler string) (sourcePath string, archiveName string, err error) {
	if strings.TrimSpace(handler) == "" {
		return "", "", fmt.Errorf("empty handler")
	}
	parts := strings.Split(handler, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid handler %q, expected file.export", handler)
	}
	filePart := parts[0]
	archiveName = filepath.Base(filePart) + ".py"
	return filePart + ".py", archiveName, nil
}

type zipResult struct {
	OutputPath string
	SHA256     string
	SizeBytes  int64
}

func zipSingleFile(sourcePath, outputPath, archiveName string) (*zipResult, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}
	defer outFile.Close()
	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()
	src, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat source file: %w", err)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, fmt.Errorf("create zip header: %w", err)
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	entry, err := zipWriter.CreateHeader(header)
	if err != nil {
		return nil, fmt.Errorf("create zip entry: %w", err)
	}
	if _, err := io.Copy(entry, src); err != nil {
		return nil, fmt.Errorf("write zip entry: %w", err)
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return nil, fmt.Errorf("close zip file: %w", err)
	}
	b, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read output zip: %w", err)
	}
	sum := sha256.Sum256(b)
	return &zipResult{OutputPath: outputPath, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(b))}, nil
}

func runNodeHandler(root, handlerPath string, event []byte) ([]byte, error) {
	modulePath, functionName, err := splitHandler(handlerPath)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveNodeModuleFile(root, modulePath)
	if err != nil {
		return nil, err
	}
	script := `'use strict';
const path = require('path');
const { pathToFileURL } = require('url');

(async () => {
  const file = process.argv[2];
  const fnName = process.argv[3];
  const payloadRaw = process.argv[4] || 'null';
  const payload = JSON.parse(payloadRaw);

  let mod;
  try {
    mod = await import(pathToFileURL(path.resolve(file)).href);
  } catch (importErr) {
    try {
      mod = require(path.resolve(file));
    } catch (requireErr) {
      throw importErr;
    }
  }

  let fn = mod && mod[fnName];
  if (!fn && mod && mod.default && typeof mod.default === 'object') {
    fn = mod.default[fnName];
  }
  if (!fn && fnName === 'default' && mod && typeof mod.default === 'function') {
    fn = mod.default;
  }
  if (typeof fn !== 'function') {
    throw new Error('handler function not found: ' + fnName);
  }

  const out = await fn(payload, {});
  process.stdout.write(JSON.stringify(out === undefined ? null : out));
})().catch((err) => {
  process.stderr.write(String(err && err.stack ? err.stack : err));
  process.exit(1);
});`
	payload := string(event)
	if strings.TrimSpace(payload) == "" {
		payload = "null"
	}
	cmd := exec.Command("node", "-e", script, resolved, functionName, payload)
	if strings.TrimSpace(root) != "" {
		cmd.Dir = root
	}
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return nil, fmt.Errorf("node handler invoke failed (%s.%s): %w", modulePath, functionName, runErr)
	}
	return out, nil
}

func runPythonHandler(root, handlerPath string, event []byte) ([]byte, error) {
	modulePath, functionName, err := splitPythonHandler(handlerPath)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePythonModuleFile(root, modulePath)
	if err != nil {
		return nil, err
	}
	payload := string(event)
	if strings.TrimSpace(payload) == "" {
		payload = "null"
	}
	script := `import importlib.util
import json
import pathlib
import sys

module_path = pathlib.Path(sys.argv[1]).resolve()
function_name = sys.argv[2]
payload = json.loads(sys.argv[3])

spec = importlib.util.spec_from_file_location("rf_handler_module", module_path)
if spec is None or spec.loader is None:
    raise RuntimeError(f"failed to load module at {module_path}")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

fn = getattr(module, function_name, None)
if not callable(fn):
    raise RuntimeError(f"handler function not found: {function_name}")

result = fn(payload, None)
sys.stdout.write(json.dumps(result))`
	pythonExec := pythonExecutable(root)
	cmd := exec.Command(pythonExec, "-c", script, resolved, functionName, payload)
	if strings.TrimSpace(root) != "" {
		cmd.Dir = root
	}
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return nil, fmt.Errorf("python handler invoke failed (%s.%s): %w", modulePath, functionName, runErr)
	}
	return out, nil
}

func splitHandler(handlerPath string) (string, string, error) {
	h := strings.TrimSpace(handlerPath)
	idx := strings.LastIndex(h, ".")
	if idx <= 0 || idx == len(h)-1 {
		return "", "", fmt.Errorf("invalid node handler %q (expected module.function)", handlerPath)
	}
	return h[:idx], h[idx+1:], nil
}

func resolveNodeModuleFile(root, modulePath string) (string, error) {
	base := modulePath
	if strings.TrimSpace(root) != "" && !filepath.IsAbs(base) {
		base = filepath.Join(root, base)
	}
	ext := strings.ToLower(filepath.Ext(base))
	candidates := make([]string, 0, 8)
	if ext != "" {
		candidates = append(candidates, base)
	} else {
		for _, e := range []string{".js", ".mjs", ".cjs", ".ts", ".jsx", ".tsx"} {
			candidates = append(candidates, base+e)
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("node handler module %q not found (checked %v)", modulePath, candidates)
}

func splitPythonHandler(handlerPath string) (string, string, error) {
	h := strings.TrimSpace(handlerPath)
	idx := strings.LastIndex(h, ".")
	if idx <= 0 || idx == len(h)-1 {
		return "", "", fmt.Errorf("invalid python handler %q (expected module.function)", handlerPath)
	}
	return h[:idx], h[idx+1:], nil
}

func resolvePythonModuleFile(root, modulePath string) (string, error) {
	base := modulePath
	if strings.TrimSpace(root) != "" && !filepath.IsAbs(base) {
		base = filepath.Join(root, base)
	}
	ext := strings.ToLower(filepath.Ext(base))
	candidates := make([]string, 0, 4)
	if ext == ".py" {
		candidates = append(candidates, base)
	} else {
		candidates = append(candidates, base+".py")
		candidates = append(candidates, filepath.Join(base, "__init__.py"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("python handler module %q not found (checked %v)", modulePath, candidates)
}

func pythonExecutable(root string) string {
	if strings.TrimSpace(root) != "" {
		venvPython := filepath.Join(root, ".venv", "bin", "python")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	return "python"
}

func BuildKey(parts map[string]any) (string, error) {
	b, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
