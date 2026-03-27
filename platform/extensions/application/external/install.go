package external

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"gopkg.in/yaml.v3"
)

type InstallOptions struct {
	ID          string
	Kind        manifests.PluginKind // provider|runtime|simulator|router; required for source installs
	Version     string               // optional: expected version (best-effort)
	Source      string               // URL or local file path
	RegistryURL string               // optional; used when Source is empty
	AuthToken   string               // optional; used when Source is empty
	CoreVersion string               // required for registry resolve path; defaults to "dev"
}

type InstallResult struct {
	Plugin *manifests.PluginManifest `json:"plugin"`
}

func Install(opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.ID) == "" {
		return nil, fmt.Errorf("install: id required")
	}
	if strings.TrimSpace(opts.Source) == "" {
		coreVersion := strings.TrimSpace(opts.CoreVersion)
		if coreVersion == "" {
			coreVersion = "dev"
		}
		return InstallFromRegistry(
			InstallFromRegistryOptions{
				RegistryURL: opts.RegistryURL,
				AuthToken:   opts.AuthToken,
				ID:          opts.ID,
				Version:     opts.Version,
			},
			coreVersion,
		)
	}
	if !manifests.IsSupportedPluginKind(opts.Kind) {
		return nil, fmt.Errorf("install: kind must be provider, runtime, simulator, or router")
	}

	home, err := HomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(home, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	archivePath, err := fetchToCache(cacheDir, opts.Source, opts.ID)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp(cacheDir, "runfabric-ext-install-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractArchive(archivePath, tmpDir); err != nil {
		return nil, err
	}

	// Plugin contents must include a plugin.yaml at archive root OR under a single top-level directory.
	pluginDir, m, err := locateAndParsePluginYAML(tmpDir)
	if err != nil {
		return nil, err
	}
	if manifests.NormalizePluginKind(m.Kind) != opts.Kind {
		return nil, fmt.Errorf("install: plugin kind mismatch: got %q want %q", m.Kind, opts.Kind)
	}
	if m.ID != opts.ID {
		return nil, fmt.Errorf("install: plugin id mismatch: got %q want %q", m.ID, opts.ID)
	}
	if opts.Version != "" && m.Version != opts.Version {
		return nil, fmt.Errorf("install: plugin version mismatch: got %q want %q", m.Version, opts.Version)
	}

	execPath := m.Executable
	if strings.TrimSpace(execPath) == "" {
		return nil, fmt.Errorf("install: plugin.yaml missing executable")
	}
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(pluginDir, execPath)
	}
	if _, err := os.Stat(execPath); err != nil {
		return nil, fmt.Errorf("install: executable not found: %s", execPath)
	}

	// Optional checksum verification: checksums.txt with lines "sha256  <filename>"
	// Only verifies files listed; missing checksums.txt is ok.
	_ = verifyChecksumsIfPresent(pluginDir)

	dest := pluginInstallDir(home, opts.Kind, opts.ID, m.Version)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dest)
	if err := copyDir(pluginDir, dest); err != nil {
		return nil, err
	}

	// Re-resolve executable path in dest for manifest.
	destExec := m.Executable
	if !filepath.IsAbs(destExec) {
		destExec = filepath.Join(dest, destExec)
	}

	pm := &manifests.PluginManifest{
		ID:          m.ID,
		Kind:        manifests.NormalizePluginKind(m.Kind),
		Name:        m.Name,
		Description: m.Description,
		Permissions: manifests.Permissions{FS: m.Permissions.FS, Env: m.Permissions.Env, Network: m.Permissions.Network, Cloud: m.Permissions.Cloud},
		Source:      "external",
		Version:     m.Version,
		Path:        dest,
		Executable:  destExec,
	}
	invalidateDiscoverCache()
	return &InstallResult{Plugin: pm}, nil
}

func pluginInstallDir(home string, kind manifests.PluginKind, id, version string) string {
	kindDir := ""
	dirs := pluginKindDirs(kind)
	if len(dirs) > 0 {
		kindDir = dirs[0]
	}
	return filepath.Join(home, "plugins", kindDir, id, version)
}

type UninstallOptions struct {
	ID      string
	Kind    manifests.PluginKind // optional; if empty, remove across all kinds
	Version string               // optional; if empty, remove all versions
}

func Uninstall(opts UninstallOptions) error {
	if strings.TrimSpace(opts.ID) == "" {
		return fmt.Errorf("uninstall: id required")
	}
	home, err := HomeDir()
	if err != nil {
		return err
	}
	kinds := []manifests.PluginKind{manifests.KindProvider, manifests.KindRuntime, manifests.KindSimulator, manifests.KindRouter}
	if opts.Kind != "" {
		kinds = []manifests.PluginKind{opts.Kind}
	}
	var removed bool
	for _, k := range kinds {
		base := pluginInstallDir(home, k, opts.ID, "x")
		base = filepath.Dir(base) // .../<kind>/<id>
		if opts.Version != "" {
			p := filepath.Join(base, opts.Version)
			if _, err := os.Stat(p); err == nil {
				removed = true
				if err := os.RemoveAll(p); err != nil {
					return err
				}
			}
			continue
		}
		if _, err := os.Stat(base); err == nil {
			removed = true
			if err := os.RemoveAll(base); err != nil {
				return err
			}
		}
	}
	if !removed {
		return fmt.Errorf("uninstall: %q not found", opts.ID)
	}
	invalidateDiscoverCache()
	return nil
}

func fetchToCache(cacheDir, source, id string) (string, error) {
	base := filepath.Base(source)
	if base == "." || base == "/" || base == "" {
		base = "plugin"
	}
	dest := filepath.Join(cacheDir, fmt.Sprintf("%s-%s", id, base))

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source) //nolint:gosec
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("download: %s", resp.Status)
		}
		f, err := os.Create(dest)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(f, resp.Body); err != nil {
			return "", err
		}
		return dest, nil
	}

	// Local path
	srcAbs, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	in, err := os.Open(srcAbs)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return dest, nil
}

func extractArchive(archivePath, destDir string) error {
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGz(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", archivePath)
	}
}

func extractZip(path, dest string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		fp := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fp, dest+string(os.PathSeparator)) && fp != dest {
			return fmt.Errorf("zip: invalid path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		mode := f.Mode()
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			return err
		}
		_ = out.Close()
		_ = rc.Close()
	}
	return nil
}

func extractTarGz(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		fp := filepath.Join(dest, h.Name)
		if !strings.HasPrefix(fp, dest+string(os.PathSeparator)) && fp != dest {
			return fmt.Errorf("tar: invalid path: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fp, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(h.Mode)
			out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
		}
	}
	return nil
}

func locateAndParsePluginYAML(root string) (pluginDir string, m *pluginYAML, err error) {
	try := []string{
		root,
	}
	entries, _ := os.ReadDir(root)
	if len(entries) == 1 && entries[0].IsDir() {
		try = append(try, filepath.Join(root, entries[0].Name()))
	}
	for _, dir := range try {
		py := filepath.Join(dir, "plugin.yaml")
		if _, err := os.Stat(py); err != nil {
			continue
		}
		data, err := os.ReadFile(py)
		if err != nil {
			return "", nil, err
		}
		var pm pluginYAML
		if err := yaml.Unmarshal(data, &pm); err != nil {
			return "", nil, err
		}
		pm.Kind = strings.TrimSpace(pm.Kind)
		pm.ID = strings.TrimSpace(pm.ID)
		pm.Version = strings.TrimSpace(pm.Version)
		if err := validatePluginMetadata(&pm); err != nil {
			return "", nil, err
		}
		return dir, &pm, nil
	}
	return "", nil, fmt.Errorf("install: plugin.yaml not found in archive root")
}

func verifyChecksumsIfPresent(dir string) error {
	p := filepath.Join(dir, "checksums.txt")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		want := parts[0]
		name := parts[len(parts)-1]
		fp := filepath.Join(dir, name)
		b, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		got := hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, want) {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := info.Mode()
		return os.WriteFile(target, b, mode)
	})
}
