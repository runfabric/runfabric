// Package python provides Python runtime execution (run handler, invoke).
package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner runs Python handlers (e.g. via python -c or module).
type Runner struct {
	Root string
}

// Run executes the handler with the given event payload.
func (r *Runner) Run(handlerPath string, event []byte) ([]byte, error) {
	modulePath, functionName, err := splitPythonHandler(handlerPath)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePythonModuleFile(r.Root, modulePath)
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

	pythonExec := r.pythonExecutable()
	cmd := exec.Command(pythonExec, "-c", script, resolved, functionName, payload)
	if strings.TrimSpace(r.Root) != "" {
		cmd.Dir = r.Root
	}
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return nil, fmt.Errorf("python handler invoke failed (%s.%s): %w", modulePath, functionName, runErr)
	}
	return out, nil
}

func (r *Runner) pythonExecutable() string {
	if strings.TrimSpace(r.Root) != "" {
		venvPython := filepath.Join(r.Root, ".venv", "bin", "python")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	return "python"
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
