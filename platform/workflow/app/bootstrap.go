package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	secretpolicy "github.com/runfabric/runfabric/platform/core/policy/secrets"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	extRuntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
	"github.com/runfabric/runfabric/platform/state/backends"
)

type AppContext struct {
	Config     *config.Config
	Registry   *providers.Registry
	Extensions ExtensionsConnector
	RootDir    string
	Stage      string
	Backends   *backends.Bundle
}

const (
	envNonInteractive       = "RUNFABRIC_NON_INTERACTIVE"
	envAssumeYes            = "RUNFABRIC_ASSUME_YES"
	envAutoInstallExtension = "RUNFABRIC_AUTO_INSTALL_EXTENSIONS"
)

// Bootstrap loads config, resolves and validates it, then optionally applies a provider override for multi-cloud (--provider).
// providerOverride is the key from providerOverrides in runfabric.yml (e.g. "aws", "gcp"); use "" for single-provider config.
func Bootstrap(configPath, stage, providerOverride string) (*AppContext, error) {
	if err := loadDotEnvForConfig(configPath); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigLoad, "failed to load .env", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigLoad, "failed to load config", err)
	}

	extBoundary, err := providerloader.LoadBoundary(providerResolutionOptions(cfg))
	if err != nil {
		return nil, err
	}

	resetSecretResolver := secretpolicy.SetReferenceResolver(nil)
	defer resetSecretResolver()
	resetSecretManagerSchemes := secretpolicy.SetSecretManagerRefSchemes(nil)
	defer resetSecretManagerSchemes()
	if err := configureSecretManagerResolver(cfg, configPath, extBoundary); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigResolve, "failed to configure secret manager", err)
	}

	cfg, err = config.Resolve(cfg, stage)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigResolve, "failed to resolve config", err)
	}

	if err := config.ApplyProviderOverride(cfg, providerOverride); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigValidation, err.Error(), err)
	}
	if err := config.Validate(cfg); err != nil {
		if providerOverride != "" {
			return nil, appErrs.Wrap(appErrs.CodeConfigValidation, "config validation failed after provider override", err)
		}
		return nil, appErrs.Wrap(appErrs.CodeConfigValidation, "config validation failed", err)
	}

	normalizedProviderID, err := normalizePluginIDByKind(manifests.KindProvider, cfg.Provider.Name, extBoundary.PluginRegistry())
	if err != nil {
		return nil, err
	}
	cfg.Provider.Name = normalizedProviderID
	extensions := newExtensionsConnectorFromBoundary(extBoundary)
	reg := extBoundary.ProviderRegistry()
	if strings.EqualFold(strings.TrimSpace(cfg.Provider.Source), "external") &&
		!isInstalledExternalPlugin(manifests.KindProvider, cfg.Provider.Name, cfg.Provider.Version) &&
		!shouldAutoInstallExtensions(cfg) {
		if strings.TrimSpace(cfg.Provider.Version) != "" {
			return nil, fmt.Errorf(
				"provider.source is external but plugin %q version %q is not installed; run `runfabric extension install %s --version %s` or enable extensions.autoInstallExtensions",
				cfg.Provider.Name,
				cfg.Provider.Version,
				cfg.Provider.Name,
				cfg.Provider.Version,
			)
		}
		return nil, fmt.Errorf(
			"provider.source is external but plugin %q is not installed; run `runfabric extension install %s` or enable extensions.autoInstallExtensions",
			cfg.Provider.Name,
			cfg.Provider.Name,
		)
	}

	if shouldAutoInstallExtensions(cfg) {
		// Provider plugin: required by lifecycle operations.
		if err := maybeInstallMissingProvider(extensions, cfg, configPath); err != nil {
			return nil, err
		}
		// Other plugin kinds: best-effort install when explicitly referenced in runfabric.yml.
		// These are not required for core lifecycle today but are useful for consistent behavior.
		if id := config.ExtensionString(cfg, "runtimePlugin"); id != "" {
			normalizedID, nerr := normalizePluginIDByKind(manifests.KindRuntime, id, extBoundary.PluginRegistry())
			if nerr != nil {
				return nil, nerr
			}
			if !isBuiltinPlugin(extBoundary.PluginRegistry(), manifests.KindRuntime, normalizedID) {
				if err := maybeInstallMissingPlugin(configPath, manifests.KindRuntime, normalizedID, ""); err != nil {
					return nil, err
				}
			}
		}
		if id := config.ExtensionString(cfg, "simulatorPlugin"); id != "" {
			normalizedID, nerr := normalizePluginIDByKind(manifests.KindSimulator, id, extBoundary.PluginRegistry())
			if nerr != nil {
				return nil, nerr
			}
			if !isBuiltinPlugin(extBoundary.PluginRegistry(), manifests.KindSimulator, normalizedID) {
				if err := maybeInstallMissingPlugin(configPath, manifests.KindSimulator, normalizedID, ""); err != nil {
					return nil, err
				}
			}
		}
		if id := config.ExtensionString(cfg, "routerPlugin"); id != "" {
			normalizedID, nerr := normalizePluginIDByKind(manifests.KindRouter, id, extBoundary.PluginRegistry())
			if nerr != nil {
				return nil, nerr
			}
			if !isBuiltinPlugin(extBoundary.PluginRegistry(), manifests.KindRouter, normalizedID) {
				if err := maybeInstallMissingPlugin(configPath, manifests.KindRouter, normalizedID, ""); err != nil {
					return nil, err
				}
			}
		}
		if id := config.ExtensionString(cfg, "secretManagerPlugin"); id != "" {
			normalizedID, nerr := normalizePluginIDByKind(manifests.KindSecretManager, id, extBoundary.PluginRegistry())
			if nerr != nil {
				return nil, nerr
			}
			if err := maybeInstallMissingPlugin(
				configPath,
				manifests.KindSecretManager,
				normalizedID,
				config.ExtensionString(cfg, "secretManagerPluginVersion"),
			); err != nil {
				return nil, err
			}
		}
		if id := config.ExtensionString(cfg, "statePlugin"); id != "" {
			normalizedID, nerr := normalizePluginIDByKind(manifests.KindState, id, extBoundary.PluginRegistry())
			if nerr != nil {
				return nil, nerr
			}
			if !isBuiltinPlugin(extBoundary.PluginRegistry(), manifests.KindState, normalizedID) {
				if err := maybeInstallMissingPlugin(
					configPath,
					manifests.KindState,
					normalizedID,
					config.ExtensionString(cfg, "statePluginVersion"),
				); err != nil {
					return nil, err
				}
			}
		}
		// Ensure newly installed plugins are visible through the boundary.
		if err := extensions.RefreshExternal(); err != nil {
			return nil, err
		}
	}

	rootDir := filepath.Dir(configPath)
	stateKindOverride, err := resolveStateBackendKind(cfg, extBoundary)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigResolve, "failed to configure state backend extension", err)
	}
	opts := backendOptionsFromConfigAndEnv(cfg, rootDir, stateKindOverride)
	bundle, err := backends.NewBundle(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	return &AppContext{
		Config:     cfg,
		Registry:   reg,
		Extensions: extensions,
		RootDir:    rootDir,
		Stage:      stage,
		Backends:   bundle,
	}, nil
}

func resolveStateBackendKind(cfg *config.Config, extBoundary *resolution.Boundary) (string, error) {
	if cfg == nil {
		return "", nil
	}
	if extBoundary == nil {
		return "", fmt.Errorf("state plugin resolution boundary is required")
	}
	reg := extBoundary.PluginRegistry()
	id := strings.TrimSpace(config.ExtensionString(cfg, "statePlugin"))
	if id == "" {
		return "", nil
	}
	normalizedID, err := normalizePluginIDByKind(manifests.KindState, id, reg)
	if err != nil {
		return "", err
	}
	id = normalizedID
	version := strings.TrimSpace(config.ExtensionString(cfg, "statePluginVersion"))

	if reg != nil {
		if plugin := reg.Get(id); plugin != nil && plugin.Kind == manifests.KindState {
			if version == "" || plugin.Source != "external" || strings.EqualFold(strings.TrimSpace(plugin.Version), version) {
				if kind, ok := providerpolicy.BackendKindFromPlugin(id, plugin.Capabilities); ok {
					if strings.EqualFold(strings.TrimSpace(plugin.Source), "external") || providerpolicy.IsExternalOnlyState(id) {
						if _, ferr := extBoundary.ResolveStateBundleFactory(kind); ferr != nil {
							return "", fmt.Errorf("state plugin %q resolved backend kind %q but no external adapter is registered: %w", id, kind, ferr)
						}
					}
					return kind, nil
				}
				return "", fmt.Errorf(
					"state plugin %q is registered but not mapped to a backend.kind; declare a capability like backend:<kind>",
					id,
				)
			}
		}
	}

	if !isInstalledExternalPlugin(manifests.KindState, id, version) {
		if version != "" {
			return "", fmt.Errorf(
				"state plugin %q version %q is not installed; run `runfabric extension install %s --kind state --version %s` or enable extensions.autoInstallExtensions",
				id,
				version,
				id,
				version,
			)
		}
		return "", fmt.Errorf(
			"state plugin %q is not installed; run `runfabric extension install %s --kind state` or enable extensions.autoInstallExtensions",
			id,
			id,
		)
	}

	if kind, ok := providerpolicy.BackendKindFromPluginID(id); ok {
		if providerpolicy.IsExternalOnlyState(id) {
			if _, ferr := extBoundary.ResolveStateBundleFactory(kind); ferr != nil {
				return "", fmt.Errorf("state plugin %q is configured external-only but no external adapter is registered for backend kind %q: %w", id, kind, ferr)
			}
		}
		return kind, nil
	}
	return "", fmt.Errorf(
		"unsupported extensions.statePlugin %q; backend kind could not be inferred from plugin id/name or capabilities",
		id,
	)
}

func normalizePluginIDByKind(kind manifests.PluginKind, raw string, reg *manifests.PluginRegistry) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", nil
	}
	if reg == nil {
		return id, nil
	}
	resolved, err := resolvePluginIDFromManifests(kind, id, reg.List(kind))
	if err != nil {
		return "", err
	}
	if resolved != "" {
		return resolved, nil
	}
	return id, nil
}

func resolvePluginIDFromManifests(kind manifests.PluginKind, raw string, items []*manifests.PluginManifest) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	lookupKeys := pluginLookupKeys(kind, raw)
	if len(lookupKeys) == 0 {
		return "", nil
	}
	candidates := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		if hasLookupIntersection(lookupKeys, pluginLookupKeys(kind, item.ID)) || hasLookupIntersection(lookupKeys, pluginLookupKeys(kind, item.Name)) {
			candidates[item.ID] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}
	if len(candidates) == 1 {
		for id := range candidates {
			return id, nil
		}
	}
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return "", fmt.Errorf("ambiguous %s plugin %q; matches: %s", kind, strings.TrimSpace(raw), strings.Join(ids, ", "))
}

func pluginLookupKeys(kind manifests.PluginKind, raw string) map[string]struct{} {
	keys := map[string]struct{}{}
	addLookupKey(keys, raw)
	normalized := normalizedLookupToken(raw)
	if normalized != "" {
		keys[normalized] = struct{}{}
	}
	if kind == manifests.KindState {
		providerpolicy.ExpandLookupAliases(keys)
	}
	return keys
}

func addLookupKey(dst map[string]struct{}, raw string) {
	v := normalizedLookupToken(raw)
	if v != "" {
		dst[v] = struct{}{}
	}
}

func normalizedLookupToken(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	replaced := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, v)
	replaced = strings.Trim(replaced, "-")
	for strings.Contains(replaced, "--") {
		replaced = strings.ReplaceAll(replaced, "--", "-")
	}
	return strings.TrimSpace(replaced)
}

func hasLookupIntersection(a, b map[string]struct{}) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for key := range a {
		if _, ok := b[key]; ok {
			return true
		}
	}
	return false
}

func isBuiltinPlugin(reg *manifests.PluginRegistry, kind manifests.PluginKind, id string) bool {
	if reg == nil || strings.TrimSpace(id) == "" {
		return false
	}
	plugin := reg.Get(id)
	if plugin == nil || plugin.Kind != kind {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(plugin.Source), "external")
}

func configureSecretManagerResolver(cfg *config.Config, configPath string, extBoundary *resolution.Boundary) error {
	if cfg == nil {
		return nil
	}
	if extBoundary == nil {
		return fmt.Errorf("secret manager resolution boundary is required")
	}
	reg := extBoundary.PluginRegistry()
	knownSchemes := secretManagerSchemesFromRegistry(reg)
	secretpolicy.SetSecretManagerRefSchemes(knownSchemes)
	id, err := normalizePluginIDByKind(
		manifests.KindSecretManager,
		config.ExtensionString(cfg, "secretManagerPlugin"),
		reg,
	)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	version := strings.TrimSpace(config.ExtensionString(cfg, "secretManagerPluginVersion"))
	if shouldAutoInstallExtensions(cfg) {
		if err := maybeInstallMissingPlugin(configPath, manifests.KindSecretManager, id, version); err != nil {
			return err
		}
	}
	if err := extBoundary.RefreshExternal(); err != nil {
		return err
	}
	reg = extBoundary.PluginRegistry()
	knownSchemes = secretManagerSchemesFromRegistry(reg)
	secretpolicy.SetSecretManagerRefSchemes(knownSchemes)
	if !isInstalledExternalPlugin(manifests.KindSecretManager, id, version) {
		if version != "" {
			return fmt.Errorf(
				"secret manager plugin %q version %q is not installed; run `runfabric extension install %s --kind secret-manager --version %s` or enable extensions.autoInstallExtensions",
				id,
				version,
				id,
				version,
			)
		}
		return fmt.Errorf(
			"secret manager plugin %q is not installed; run `runfabric extension install %s --kind secret-manager` or enable extensions.autoInstallExtensions",
			id,
			id,
		)
	}

	manifest := reg.Get(id)
	if manifest == nil || manifest.Kind != manifests.KindSecretManager {
		if version != "" {
			return fmt.Errorf("secret manager plugin %q version %q is installed but not discoverable", id, version)
		}
		return fmt.Errorf("secret manager plugin %q is installed but not discoverable", id)
	}
	if version != "" && !strings.EqualFold(strings.TrimSpace(manifest.Version), version) {
		return fmt.Errorf("secret manager plugin %q version %q is installed but not discoverable", id, version)
	}
	selectedSchemes := secretManagerSchemesFromCapabilities(manifest.Capabilities)
	if len(selectedSchemes) > 0 {
		secretpolicy.SetSecretManagerRefSchemes(append(knownSchemes, selectedSchemes...))
	}
	adapter, err := extBoundary.ResolveSecretManager(id)
	if err != nil {
		if version != "" {
			return fmt.Errorf("secret manager plugin %q version %q is installed but not discoverable: %w", id, version, err)
		}
		return fmt.Errorf("secret manager plugin %q is installed but not discoverable: %w", id, err)
	}
	secretpolicy.SetReferenceResolver(func(ref string) (string, error) {
		return adapter.ResolveSecret(context.Background(), ref)
	})
	return nil
}

func secretManagerSchemesFromRegistry(reg *manifests.PluginRegistry) []string {
	if reg == nil {
		return nil
	}
	items := reg.List(manifests.KindSecretManager)
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		for _, scheme := range secretManagerSchemesFromCapabilities(item.Capabilities) {
			if _, ok := seen[scheme]; ok {
				continue
			}
			seen[scheme] = struct{}{}
			out = append(out, scheme)
		}
		for _, scheme := range secretManagerSchemesFromText(item.Description) {
			if _, ok := seen[scheme]; ok {
				continue
			}
			seen[scheme] = struct{}{}
			out = append(out, scheme)
		}
	}
	sort.Strings(out)
	return out
}

func secretManagerSchemesFromCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range capabilities {
		c := strings.TrimSpace(strings.ToLower(raw))
		if c == "" {
			continue
		}
		candidates := []string{}
		if strings.HasPrefix(c, "scheme:") {
			candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(c, "scheme:")))
		}
		if strings.HasPrefix(c, "ref-scheme:") {
			candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(c, "ref-scheme:")))
		}
		if idx := strings.Index(c, "://"); idx > 0 {
			candidates = append(candidates, strings.TrimSpace(c[:idx]))
		}
		for _, scheme := range candidates {
			scheme = strings.TrimSpace(strings.TrimSuffix(scheme, "://"))
			if scheme == "" {
				continue
			}
			if _, ok := seen[scheme]; ok {
				continue
			}
			seen[scheme] = struct{}{}
			out = append(out, scheme)
		}
	}
	return out
}

func secretManagerSchemesFromText(input string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(input)))
	if len(fields) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, field := range fields {
		if idx := strings.Index(field, "://"); idx > 0 {
			scheme := strings.TrimSpace(field[:idx])
			scheme = strings.TrimLeft(scheme, "([{\"'")
			scheme = strings.TrimRight(scheme, ".,;:!?)]}\"'")
			if scheme == "" {
				continue
			}
			if _, ok := seen[scheme]; ok {
				continue
			}
			seen[scheme] = struct{}{}
			out = append(out, scheme)
		}
	}
	return out
}

func shouldAutoInstallExtensions(cfg *config.Config) bool {
	if isTruthyEnv(envAutoInstallExtension) {
		return true
	}
	return config.AutoInstallExtensions(cfg)
}

func isTruthyEnv(k string) bool {
	return external.EnvTruthy(k)
}

func providerResolutionOptions(cfg *config.Config) providerloader.LoadOptions {
	opts := providerloader.LoadOptions{IncludeExternal: true}
	if cfg == nil {
		return opts
	}
	pinned := map[string]string{}
	if strings.EqualFold(strings.TrimSpace(cfg.Provider.Source), "external") {
		opts.PreferExternal = true
		if id := strings.TrimSpace(cfg.Provider.Name); id != "" && strings.TrimSpace(cfg.Provider.Version) != "" {
			pinned[id] = strings.TrimSpace(cfg.Provider.Version)
		}
	}
	if id := strings.TrimSpace(config.ExtensionString(cfg, "secretManagerPlugin")); id != "" {
		if v := strings.TrimSpace(config.ExtensionString(cfg, "secretManagerPluginVersion")); v != "" {
			pinned[id] = v
		}
	}
	if id := strings.TrimSpace(config.ExtensionString(cfg, "statePlugin")); id != "" {
		if v := strings.TrimSpace(config.ExtensionString(cfg, "statePluginVersion")); v != "" {
			pinned[id] = v
		}
	}
	if len(pinned) > 0 {
		opts.PinnedVersions = pinned
	}
	return opts
}

func loadDotEnvForConfig(configPath string) error {
	configDir := filepath.Dir(configPath)
	if strings.TrimSpace(configDir) == "" {
		return nil
	}
	path := filepath.Join(configDir, ".env")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(line[eq+1:])
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env from .env line %d: %w", lineNum, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func maybeInstallMissingProvider(extensions ExtensionsConnector, cfg *config.Config, configPath string) error {
	name := strings.TrimSpace(cfg.Provider.Name)
	if name == "" {
		return nil
	}
	wantExternal := strings.EqualFold(strings.TrimSpace(cfg.Provider.Source), "external")

	if !wantExternal {
		if _, err := extensions.ResolveProvider(name); err == nil {
			return nil
		}
	} else if isInstalledExternalPlugin(manifests.KindProvider, name, cfg.Provider.Version) {
		// Ensure adapter registration reflects on-disk plugins.
		if err := extensions.RefreshExternal(); err != nil {
			return err
		}
		if _, err := extensions.ResolveProvider(name); err == nil {
			return nil
		}
	}

	// Attempt install (if allowed). If it fails or is declined, keep the original error behavior.
	if err := maybeInstallMissingPlugin(configPath, manifests.KindProvider, name, cfg.Provider.Version); err != nil {
		return err
	}

	// Re-discover to find executable path, then register adapter.
	if err := extensions.RefreshExternal(); err != nil {
		return err
	}
	if _, err := extensions.ResolveProvider(name); err != nil {
		return err
	}
	return nil
}

func maybeInstallMissingPlugin(configPath string, kind manifests.PluginKind, id, version string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	version = strings.TrimSpace(version)

	// Already installed?
	if isInstalledExternalPlugin(kind, id, version) {
		return nil
	}

	nonInteractive := isTruthyEnv(envNonInteractive)
	assumeYes := isTruthyEnv(envAssumeYes)

	registryURL := ""
	registryToken := ""
	rc := external.LoadRunfabricrc(filepath.Dir(configPath))
	if rc.RegistryURL != "" {
		registryURL = rc.RegistryURL
	}
	if rc.RegistryToken != "" {
		registryToken = rc.RegistryToken
	}
	if v := external.RegistryURLFromEnv(); v != "" {
		registryURL = v
	}
	if v := external.RegistryTokenFromEnv(); v != "" {
		registryToken = v
	}

	if !assumeYes {
		if nonInteractive {
			return fmt.Errorf("%s %q not installed (auto-install enabled, but non-interactive). Install it with `runfabric extension install %s` or re-run with -y", kind, id, id)
		}
		regShown := registryURL
		if strings.TrimSpace(regShown) == "" {
			regShown = external.DefaultRegistryURL()
		}
		ok, err := promptYesNo(fmt.Sprintf("%s %q is not installed. Install from registry %s?", pluginKindLabel(kind), id, regShown))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s %q not installed", kind, id)
		}
	}

	ir, err := external.InstallFromRegistry(
		external.InstallFromRegistryOptions{
			RegistryURL: registryURL,
			AuthToken:   registryToken,
			ID:          id,
			Version:     version,
		},
		extRuntime.Version,
	)
	if err != nil {
		return err
	}
	if ir == nil || ir.Plugin == nil {
		return fmt.Errorf("auto-install: install returned empty result")
	}
	if ir.Plugin.Kind != kind {
		return fmt.Errorf("auto-install: resolved %q as %q (expected %s)", id, ir.Plugin.Kind, kind)
	}
	if version != "" && !strings.EqualFold(ir.Plugin.Version, version) {
		return fmt.Errorf("auto-install: resolved %q version %q (expected %q)", id, ir.Plugin.Version, version)
	}
	return nil
}

func isInstalledExternalPlugin(kind manifests.PluginKind, id, version string) bool {
	ok, err := resolution.HasInstalledExternalPlugin(kind, id, version)
	return err == nil && ok
}

func pluginKindLabel(k manifests.PluginKind) string {
	switch k {
	case manifests.KindProvider:
		return "Provider"
	case manifests.KindRuntime:
		return "Runtime"
	case manifests.KindSimulator:
		return "Simulator"
	case manifests.KindRouter:
		return "Router"
	case manifests.KindSecretManager:
		return "Secret manager"
	case manifests.KindState:
		return "State"
	default:
		return string(k)
	}
}

func promptYesNo(q string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", strings.TrimSpace(q))
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		// Best-effort: treat read error as "no".
		return false, err
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s == "y" || s == "yes", nil
}

// BackendOptionsForKind returns backends.Options built from ctx.Config and env, with Kind set to kindOverride (use "" for config default).
// Used by state migrate to build source and target bundles.
func BackendOptionsForKind(ctx *AppContext, kindOverride string) backends.Options {
	return backendOptionsFromConfigAndEnv(ctx.Config, ctx.RootDir, kindOverride)
}

func backendOptionsFromConfigAndEnv(cfg *config.Config, rootDir, kindOverride string) backends.Options {
	kind := os.Getenv("RUNFABRIC_BACKEND")
	s3Bucket := os.Getenv("RUNFABRIC_S3_BUCKET")
	s3Prefix := os.Getenv("RUNFABRIC_S3_PREFIX")
	dynamoTable := os.Getenv("RUNFABRIC_DYNAMODB_TABLE")
	postgresDSN := ""
	postgresTable := "runfabric_receipts"
	sqlitePath := ".runfabric/state.db"
	receiptTable := ""
	awsRegion := ""

	if cfg != nil {
		awsRegion = cfg.Provider.Region
	}

	if cfg != nil && cfg.Backend != nil {
		if cfg.Backend.Kind != "" {
			kind = cfg.Backend.Kind
		}
		if cfg.Backend.S3Bucket != "" {
			s3Bucket = cfg.Backend.S3Bucket
		}
		if cfg.Backend.S3Prefix != "" {
			s3Prefix = cfg.Backend.S3Prefix
		}
		if cfg.Backend.LockTable != "" {
			dynamoTable = cfg.Backend.LockTable
		}
		postgresTable = cfg.Backend.PostgresTable
		if postgresTable == "" {
			postgresTable = "runfabric_receipts"
		}
		sqlitePath = cfg.Backend.SqlitePath
		if sqlitePath == "" {
			sqlitePath = ".runfabric/state.db"
		}
		receiptTable = cfg.Backend.ReceiptTable
		if cfg.Backend.Kind == "postgres" && cfg.Backend.PostgresConnectionStringEnv != "" {
			postgresDSN = os.Getenv(cfg.Backend.PostgresConnectionStringEnv)
		}
	}
	if kindOverride != "" {
		kind = kindOverride
	}
	return backends.Options{
		Kind:            kind,
		Root:            rootDir,
		AWSRegion:       awsRegion,
		S3Bucket:        s3Bucket,
		S3Prefix:        s3Prefix,
		DynamoTableName: dynamoTable,
		PostgresDSN:     postgresDSN,
		PostgresTable:   postgresTable,
		SqlitePath:      sqlitePath,
		ReceiptTable:    receiptTable,
	}
}
