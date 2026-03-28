package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	"github.com/runfabric/runfabric/platform/core/state/backends"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	extRuntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
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
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigLoad, "failed to load config", err)
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

	extBoundary, err := providerloader.LoadBoundary(providerResolutionOptions(cfg))
	if err != nil {
		return nil, err
	}
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
			if err := maybeInstallMissingPlugin(configPath, manifests.KindRuntime, id, ""); err != nil {
				return nil, err
			}
		}
		if id := config.ExtensionString(cfg, "simulatorPlugin"); id != "" {
			if err := maybeInstallMissingPlugin(configPath, manifests.KindSimulator, id, ""); err != nil {
				return nil, err
			}
		}
		if id := config.ExtensionString(cfg, "routerPlugin"); id != "" {
			if err := maybeInstallMissingPlugin(configPath, manifests.KindRouter, id, ""); err != nil {
				return nil, err
			}
		}
		// Ensure newly installed plugins are visible through the boundary.
		if err := extensions.RefreshExternal(); err != nil {
			return nil, err
		}
	}

	rootDir := filepath.Dir(configPath)
	opts := backendOptionsFromConfigAndEnv(cfg, rootDir, "")
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
	if strings.EqualFold(strings.TrimSpace(cfg.Provider.Source), "external") {
		opts.PreferExternal = true
		if id := strings.TrimSpace(cfg.Provider.Name); id != "" && strings.TrimSpace(cfg.Provider.Version) != "" {
			opts.PinnedVersions = map[string]string{id: strings.TrimSpace(cfg.Provider.Version)}
		}
	}
	return opts
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
