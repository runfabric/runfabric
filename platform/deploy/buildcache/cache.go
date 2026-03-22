// Package buildcache provides per-function build artifact caching by content hash
// (runfabric.yml slice + source files) so plan/deploy can skip redundant packaging.
package buildcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"gopkg.in/yaml.v3"
)

const cacheDirName = ".runfabric/cache"

// HashForFunction computes a stable content hash for the given function: relevant
// config (service, provider.runtime, function entry) plus the function's handler
// and common dependency files (package.json, requirements.txt, go.mod) under rootDir.
func HashForFunction(cfg *config.Config, rootDir, functionName string) (string, error) {
	h := sha256.New()

	// Include service and provider runtime so runtime/version changes invalidate cache.
	_, _ = h.Write([]byte(cfg.Service))
	_, _ = h.Write([]byte("\x00"))
	_, _ = h.Write([]byte(cfg.Provider.Runtime))
	_, _ = h.Write([]byte("\x00"))

	fn, ok := cfg.Functions[functionName]
	if !ok {
		return "", fmt.Errorf("function %q not in config", functionName)
	}

	// Serialize function config slice (handler, events, env, etc.) in stable order.
	fnMap := map[string]any{
		"handler": fn.Handler,
		"runtime": fn.Runtime,
		"events":  fn.Events,
	}
	if len(fn.Environment) > 0 {
		envKeys := make([]string, 0, len(fn.Environment))
		for k := range fn.Environment {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)
		envMap := make(map[string]any)
		for _, k := range envKeys {
			envMap[k] = fn.Environment[k]
		}
		fnMap["environment"] = envMap
	}
	fnYAML, err := yaml.Marshal(fnMap)
	if err != nil {
		return "", err
	}
	_, _ = h.Write(fnYAML)
	_, _ = h.Write([]byte("\x00"))

	// Hash handler file and common dependency manifests.
	pathsToHash := []string{}
	if fn.Handler != "" {
		handlerPath := filepath.FromSlash(fn.Handler)
		// If handler looks like "path/file.export", hash file path with common extensions; otherwise hash as-is.
		if i := len(handlerPath) - 1; i >= 0 {
			for i >= 0 && handlerPath[i] != '.' && handlerPath[i] != '/' && handlerPath[i] != filepath.Separator {
				i--
			}
			if i > 0 && handlerPath[i] == '.' {
				base := handlerPath[:i]
				for _, ext := range []string{".js", ".ts", ".mjs", ".cjs", ".py", ".go"} {
					pathsToHash = append(pathsToHash, base+ext)
				}
			} else {
				pathsToHash = append(pathsToHash, handlerPath)
			}
		} else {
			pathsToHash = append(pathsToHash, handlerPath)
		}
	}
	pathsToHash = append(pathsToHash, "package.json", "package-lock.json", "requirements.txt", "go.mod")

	seen := map[string]bool{}
	for _, p := range pathsToHash {
		if seen[p] {
			continue
		}
		seen[p] = true
		abs := filepath.Join(rootDir, p)
		data, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = h.Write([]byte(p))
				_, _ = h.Write([]byte("\x00<absent>\x00"))
				continue
			}
			return "", fmt.Errorf("read %s: %w", p, err)
		}
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte("\x00"))
		_, _ = h.Write(data)
		_, _ = h.Write([]byte("\x00"))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Entry is stored per function and hash.
type Entry struct {
	Hash         string `json:"hash"`
	FunctionName string `json:"function_name"`
	ArtifactPath string `json:"artifact_path,omitempty"` // optional path to built artifact
}

// Get returns the cached artifact path for the given function and hash, if any.
func Get(projectDir, functionName, hash string) (artifactPath string, ok bool) {
	dir := filepath.Join(projectDir, cacheDirName, functionName)
	f := filepath.Join(dir, hash+".json")
	data, err := os.ReadFile(f)
	if err != nil {
		return "", false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return "", false
	}
	if e.Hash != hash {
		return "", false
	}
	return e.ArtifactPath, true
}

// Put stores a cache entry for the function and hash. artifactPath can be empty
// when we only want to record that this hash was built (path filled later).
func Put(projectDir, functionName, hash, artifactPath string) error {
	dir := filepath.Join(projectDir, cacheDirName, functionName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	e := Entry{Hash: hash, FunctionName: functionName, ArtifactPath: artifactPath}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, hash+".json"), data, 0o644)
}

// Clear removes all cache entries for the project (or only for functionName if non-empty).
func Clear(projectDir, functionName string) error {
	base := filepath.Join(projectDir, cacheDirName)
	if functionName != "" {
		return os.RemoveAll(filepath.Join(base, functionName))
	}
	return os.RemoveAll(base)
}
