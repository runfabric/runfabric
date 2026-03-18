package external

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const (
	envHome           = "RUNFABRIC_HOME"
	envPreferExternal = "RUNFABRIC_PREFER_EXTERNAL_PLUGINS"
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
	APIVersion  string `yaml:"apiVersion"`
	Kind        string `yaml:"kind"` // provider | runtime | simulator
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	PluginVer   any    `yaml:"pluginVersion"`
	Executable  string `yaml:"executable"`
	Permissions struct {
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
	// Version strings are directory names (e.g. "0.2.0" or "v0.2.0").
	PinnedVersions map[string]string
	// IncludeInvalid records invalid/skipped plugin entries in the returned report.
	IncludeInvalid bool
}

type DiscoverResult struct {
	Plugins []*manifests.PluginManifest `json:"plugins"`
	Invalid []InvalidPlugin             `json:"invalid,omitempty"`
}

func PreferExternalFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envPreferExternal)))
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
	if _, err := os.Stat(root); err != nil {
		// No plugins dir is not an error.
		return DiscoverResult{}, nil
	}
	res := DiscoverResult{}
	for _, kindDir := range []struct {
		dir  string
		kind manifests.PluginKind
	}{
		{"providers", manifests.KindProvider},
		{"runtimes", manifests.KindRuntime},
		{"simulators", manifests.KindSimulator},
	} {
		found, invalid, _ := discoverKind(filepath.Join(root, kindDir.dir), kindDir.kind, opts)
		res.Plugins = append(res.Plugins, found...)
		if opts.IncludeInvalid {
			res.Invalid = append(res.Invalid, invalid...)
		}
	}
	return res, nil
}

// DiscoverLatest is preserved for backward compatibility.
func DiscoverLatest() ([]*manifests.PluginManifest, error) {
	res, err := Discover(DiscoverOptions{})
	if err != nil {
		return nil, err
	}
	return res.Plugins, nil
}

func discoverKind(kindRoot string, kind manifests.PluginKind, opts DiscoverOptions) ([]*manifests.PluginManifest, []InvalidPlugin, error) {
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
				invalid = append(invalid, InvalidPlugin{Kind: kind, ID: id, Version: bestVer, Path: bestPath, Reason: "failed to read/parse plugin.yaml"})
			}
			continue
		}
		if manifests.PluginKind(m.Kind) != kind {
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
			ID:          m.ID,
			Kind:        kind,
			Name:        m.Name,
			Description: m.Description,
			Permissions: manifests.Permissions{FS: m.Permissions.FS, Env: m.Permissions.Env, Network: m.Permissions.Network, Cloud: m.Permissions.Cloud},
			Source:      "external",
			Version:     m.Version,
			Path:        bestPath,
			Executable:  execPath,
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
			// allow pinned values that omit leading v
			if normalizeDirSemver(v.Name()) == normalizeDirSemver(pinned) {
				return v.Name(), filepath.Join(idPath, v.Name())
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
		norm := normalizeDirSemver(raw)
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

func normalizeDirSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
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
	// Normalize kind synonyms if any are introduced later.
	m.Kind = strings.TrimSpace(m.Kind)
	m.ID = strings.TrimSpace(m.ID)
	m.Version = strings.TrimSpace(m.Version)
	return &m, nil
}
