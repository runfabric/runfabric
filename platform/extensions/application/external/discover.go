package external

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const (
	envHome           = "RUNFABRIC_HOME"
	envPreferExternal = "RUNFABRIC_PREFER_EXTERNAL_PLUGINS"
	pluginAPIVersion  = "runfabric.io/plugin/v1"
)

// HomeDir returns RUNFABRIC_HOME if set, otherwise "~/.runfabric".
func HomeDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(envHome)); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".runfabric"), nil
}

type pluginYAML struct {
	APIVersion        string   `yaml:"apiVersion"`
	Kind              string   `yaml:"kind"` // provider | runtime | simulator | router | secret-manager | state
	ID                string   `yaml:"id"`
	Name              string   `yaml:"name"`
	Description       string   `yaml:"description"`
	Version           string   `yaml:"version"`
	PluginVer         int      `yaml:"pluginVersion"`
	Executable        string   `yaml:"executable"`
	Capabilities      []string `yaml:"capabilities"`
	SupportsRuntime   []string `yaml:"supportsRuntime"`
	SupportsTriggers  []string `yaml:"supportsTriggers"`
	SupportsResources []string `yaml:"supportsResources"`
	Permissions       struct {
		FS      bool `yaml:"fs"`
		Env     bool `yaml:"env"`
		Network bool `yaml:"network"`
		Cloud   bool `yaml:"cloud"`
	} `yaml:"permissions"`
}

type InvalidPlugin struct {
	Kind    manifests.PluginKind `json:"kind"`
	ID      string               `json:"id"`
	Version string               `json:"version,omitempty"`
	Path    string               `json:"path,omitempty"`
	Reason  string               `json:"reason"`
}

type DiscoverOptions struct {
	// PreferExternal allows external manifests to override built-ins with the same ID.
	PreferExternal bool
	// PinnedVersions optionally pins a specific version per plugin ID.
	// Version strings are strict semver directory names (e.g. "0.2.0").
	PinnedVersions map[string]string
	// IncludeInvalid records invalid/skipped plugin entries in the returned report.
	IncludeInvalid bool
}

type DiscoverResult struct {
	Plugins []*manifests.PluginManifest `json:"plugins"`
	Invalid []InvalidPlugin             `json:"invalid,omitempty"`
}

type discoverCacheEntry struct {
	rootExists bool
	rootMod    int64
	result     DiscoverResult
}

var (
	discoverCacheMu sync.RWMutex
	discoverCache   = map[string]discoverCacheEntry{}
)

func invalidateDiscoverCache() {
	discoverCacheMu.Lock()
	discoverCache = map[string]discoverCacheEntry{}
	discoverCacheMu.Unlock()
}

func PreferExternalFromEnv() bool {
	return EnvTruthy(envPreferExternal)
}

// EnvTruthy reports whether an environment variable is set to a truthy value.
// Accepted values (case-insensitive, trimmed): 1, true, yes.
func EnvTruthy(key string) bool {
	return isTruthyValue(os.Getenv(key))
}

func isTruthyValue(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	return v == "1" || v == "true" || v == "yes"
}

// Discover returns external plugin manifests discovered on disk.
// It selects the latest installed version per (kind,id) using semver unless pinned.
// Invalid plugin dirs/manifests are skipped (best-effort); when opts.IncludeInvalid is true,
// a report entry is returned for each skipped plugin/version with a reason.
func Discover(opts DiscoverOptions) (DiscoverResult, error) {
	home, err := HomeDir()
	if err != nil {
		return DiscoverResult{}, err
	}
	root := filepath.Join(home, "plugins")
	rootExists := false
	rootMod := int64(0)
	if info, statErr := os.Stat(root); statErr == nil {
		rootExists = true
		rootMod = info.ModTime().UnixNano()
	}

	key := discoverCacheKey(home, opts)
	if cached, ok := lookupDiscoverCache(key, rootExists, rootMod); ok {
		return cached, nil
	}

	if !rootExists {
		storeDiscoverCache(key, rootExists, rootMod, DiscoverResult{})
		// No plugins dir is not an error.
		return DiscoverResult{}, nil
	}
	res := DiscoverResult{}
	for _, kind := range []manifests.PluginKind{
		manifests.KindProvider,
		manifests.KindRuntime,
		manifests.KindSimulator,
		manifests.KindRouter,
		manifests.KindSecretManager,
		manifests.KindState,
	} {
		found, invalid := discoverKind(root, kind, opts)
		res.Plugins = append(res.Plugins, found...)
		if opts.IncludeInvalid {
			res.Invalid = append(res.Invalid, invalid...)
		}
	}
	storeDiscoverCache(key, rootExists, rootMod, res)
	return res, nil
}

// DiscoverLatest returns discovered plugins with default discovery options.
func DiscoverLatest() ([]*manifests.PluginManifest, error) {
	res, err := Discover(DiscoverOptions{})
	if err != nil {
		return nil, err
	}
	return res.Plugins, nil
}

func discoverCacheKey(home string, opts DiscoverOptions) string {
	pins := make([]string, 0, len(opts.PinnedVersions))
	for id, version := range opts.PinnedVersions {
		pins = append(pins, id+"="+version)
	}
	sort.Strings(pins)
	return strings.Join([]string{
		home,
		fmt.Sprintf("prefer=%t", opts.PreferExternal),
		fmt.Sprintf("invalid=%t", opts.IncludeInvalid),
		strings.Join(pins, ","),
	}, "|")
}

func lookupDiscoverCache(key string, rootExists bool, rootMod int64) (DiscoverResult, bool) {
	discoverCacheMu.RLock()
	entry, ok := discoverCache[key]
	discoverCacheMu.RUnlock()
	if !ok || entry.rootExists != rootExists || entry.rootMod != rootMod {
		return DiscoverResult{}, false
	}
	return cloneDiscoverResult(entry.result), true
}

func storeDiscoverCache(key string, rootExists bool, rootMod int64, result DiscoverResult) {
	discoverCacheMu.Lock()
	discoverCache[key] = discoverCacheEntry{
		rootExists: rootExists,
		rootMod:    rootMod,
		result:     cloneDiscoverResult(result),
	}
	discoverCacheMu.Unlock()
}

func cloneDiscoverResult(in DiscoverResult) DiscoverResult {
	out := DiscoverResult{
		Plugins: make([]*manifests.PluginManifest, 0, len(in.Plugins)),
		Invalid: append([]InvalidPlugin(nil), in.Invalid...),
	}
	for _, m := range in.Plugins {
		if m == nil {
			continue
		}
		copyManifest := *m
		out.Plugins = append(out.Plugins, &copyManifest)
	}
	return out
}

func discoverKind(root string, kind manifests.PluginKind, opts DiscoverOptions) ([]*manifests.PluginManifest, []InvalidPlugin) {
	bestByID := map[string]*manifests.PluginManifest{}
	bestByIDSemver := map[string]string{}
	var invalid []InvalidPlugin

	kindDir := pluginKindDir(kind)
	if kindDir == "" {
		return nil, nil
	}
	found, localInvalid, err := discoverKindDir(filepath.Join(root, kindDir), kind, opts)
	if err != nil {
		// Missing kind dir is fine.
		if os.IsNotExist(err) {
			return nil, nil
		}
		if opts.IncludeInvalid {
			invalid = append(invalid, InvalidPlugin{
				Kind:   kind,
				Path:   filepath.Join(root, kindDir),
				Reason: fmt.Sprintf("failed to scan kind directory: %v", err),
			})
		}
		return nil, invalid
	}
	if opts.IncludeInvalid {
		invalid = append(invalid, localInvalid...)
	}
	for _, m := range found {
		if m == nil {
			continue
		}
		current, ok := bestByID[m.ID]
		if !ok {
			bestByID[m.ID] = m
			bestByIDSemver[m.ID] = normalizedDirSemver(m.Version)
			continue
		}
		prevNorm := bestByIDSemver[m.ID]
		nextNorm := normalizedDirSemver(m.Version)
		cmp := compareSemverNormalized(nextNorm, prevNorm)
		if cmp > 0 {
			bestByID[m.ID] = m
			bestByIDSemver[m.ID] = nextNorm
			continue
		}
		// Keep deterministic tie-breaker by path.
		if cmp == 0 && current.Path > m.Path {
			bestByID[m.ID] = m
			bestByIDSemver[m.ID] = nextNorm
		}
	}

	out := make([]*manifests.PluginManifest, 0, len(bestByID))
	for _, m := range bestByID {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, invalid
}

func discoverKindDir(kindRoot string, kind manifests.PluginKind, opts DiscoverOptions) ([]*manifests.PluginManifest, []InvalidPlugin, error) {
	entries, err := os.ReadDir(kindRoot)
	if err != nil {
		return nil, nil, err
	}
	var out []*manifests.PluginManifest
	var invalid []InvalidPlugin
	for _, idEnt := range entries {
		if !idEnt.IsDir() {
			continue
		}
		id := idEnt.Name()
		idPath := filepath.Join(kindRoot, id)
		bestVer, bestPath := selectVersionDir(idPath, opts.PinnedVersions[id])
		if bestPath == "" {
			if opts.IncludeInvalid {
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Path: idPath, Reason: "no valid version directories found"})
			}
			continue
		}
		m, err := readPluginYAML(bestPath)
		if err != nil || m == nil {
			if opts.IncludeInvalid {
				reason := "failed to read/parse plugin.yaml"
				if err != nil {
					reason = err.Error()
				}
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: reason})
			}
			continue
		}
		if manifests.NormalizePluginKind(m.Kind) != kind {
			if opts.IncludeInvalid {
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "kind mismatch"})
			}
			continue
		}
		if strings.TrimSpace(m.ID) == "" || m.ID != id {
			if opts.IncludeInvalid {
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "id mismatch"})
			}
			continue
		}
		if strings.TrimSpace(m.Version) == "" || m.Version != bestVer {
			if opts.IncludeInvalid {
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "version mismatch"})
			}
			continue
		}

		execPath := ""
		if strings.TrimSpace(m.Executable) != "" {
			execPath = m.Executable
			if !filepath.IsAbs(execPath) {
				execPath = filepath.Join(bestPath, execPath)
			}
			if _, err := os.Stat(execPath); err != nil {
				// executable missing; skip for now (best-effort)
				if opts.IncludeInvalid {
					invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "executable missing"})
				}
				continue
			}
		} else {
			if opts.IncludeInvalid {
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "executable not specified"})
			}
			continue
		}

		out = append(out, &manifests.PluginManifest{
			ID:                m.ID,
			Kind:              kind,
			Name:              m.Name,
			Description:       m.Description,
			Permissions:       manifests.Permissions{FS: m.Permissions.FS, Env: m.Permissions.Env, Network: m.Permissions.Network, Cloud: m.Permissions.Cloud},
			Capabilities:      append([]string(nil), m.Capabilities...),
			SupportsRuntime:   append([]string(nil), m.SupportsRuntime...),
			SupportsTriggers:  append([]string(nil), m.SupportsTriggers...),
			SupportsResources: append([]string(nil), m.SupportsResources...),
			Source:            "external",
			Version:           m.Version,
			Path:              bestPath,
			Executable:        execPath,
		})
	}
	return out, invalid, nil
}

func selectVersionDir(idPath string, pinned string) (version string, path string) {
	vers, err := os.ReadDir(idPath)
	if err != nil {
		return "", ""
	}
	if strings.TrimSpace(pinned) != "" {
		for _, v := range vers {
			if !v.IsDir() {
				continue
			}
			if v.Name() == pinned {
				return pinned, filepath.Join(idPath, pinned)
			}
		}
		return "", ""
	}
	best := ""
	bestPath := ""
	for _, v := range vers {
		if !v.IsDir() {
			continue
		}
		raw := v.Name()
		norm := normalizedDirSemver(raw)
		if norm == "" {
			continue
		}
		if best == "" || semver.Compare(norm, best) > 0 {
			best = norm
			bestPath = filepath.Join(idPath, raw)
			version = raw
		}
	}
	return version, bestPath
}

func normalizedDirSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return ""
	}
	normalized := "v" + v
	if !semver.IsValid(normalized) {
		return ""
	}
	return normalized
}

func compareSemverNormalized(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	return semver.Compare(a, b)
}

func pluginKindDir(kind manifests.PluginKind) string {
	switch kind {
	case manifests.KindProvider:
		return "providers"
	case manifests.KindRuntime:
		return "runtimes"
	case manifests.KindSimulator:
		return "simulators"
	case manifests.KindRouter:
		return "routers"
	case manifests.KindSecretManager:
		return "secret-managers"
	case manifests.KindState:
		return "states"
	default:
		return ""
	}
}

func readPluginYAML(dir string) (*pluginYAML, error) {
	p := filepath.Join(dir, "plugin.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m pluginYAML
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if strings.TrimSpace(m.APIVersion) != pluginAPIVersion {
		return nil, fmt.Errorf("unsupported plugin apiVersion %q", strings.TrimSpace(m.APIVersion))
	}
	m.Kind = string(manifests.NormalizePluginKind(m.Kind))
	m.ID = strings.TrimSpace(m.ID)
	m.Version = strings.TrimSpace(m.Version)
	if err := validatePluginMetadata(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func validatePluginMetadata(m *pluginYAML) error {
	if m == nil {
		return fmt.Errorf("plugin metadata is required")
	}
	if manifests.NormalizePluginKind(m.Kind) != manifests.KindProvider {
		return nil
	}
	if len(normalizedNonEmpty(m.Capabilities)) == 0 {
		return fmt.Errorf("provider plugin.yaml must declare capabilities")
	}
	if len(normalizedNonEmpty(m.SupportsTriggers)) == 0 {
		return fmt.Errorf("provider plugin.yaml must declare supportsTriggers")
	}
	if len(normalizedNonEmpty(m.SupportsRuntime)) == 0 {
		return fmt.Errorf("provider plugin.yaml must declare supportsRuntime")
	}
	return nil
}

func normalizedNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
