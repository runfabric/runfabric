// Package node provides Node.js runtime execution (run handler, invoke).
package node

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner runs Node handlers (e.g. via node or ts-node).
type Runner struct {
	Root string
}

// Run executes the handler with the given event payload.
func (r *Runner) Run(handlerPath string, event []byte) ([]byte, error) {
	modulePath, functionName, err := splitHandler(handlerPath)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveNodeModuleFile(r.Root, modulePath)
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
	if strings.TrimSpace(r.Root) != "" {
		cmd.Dir = r.Root
	}
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return nil, fmt.Errorf("node handler invoke failed (%s.%s): %w", modulePath, functionName, runErr)
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
