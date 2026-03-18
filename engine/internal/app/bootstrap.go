package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/extensions/external"
	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
	awsprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/aws"
	gcpprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/gcp"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	extproviders "github.com/runfabric/runfabric/engine/internal/extensions/providers"
	extRuntime "github.com/runfabric/runfabric/engine/internal/extensions/runtime"
)

type AppContext struct {
	Config   *config.Config
	Registry *providers.Registry
	RootDir  string
	Stage    string
	Backends *backends.Bundle
}

const (
	envNonInteractive       = "RUNFABRIC_NON_INTERACTIVE"
	envAssumeYes            = "RUNFABRIC_ASSUME_YES"
	envAutoInstallExtension = "RUNFABRIC_AUTO_INSTALL_EXTENSIONS"
)

// Bootstrap loads config, resolves and validates it, then optionally applies a provider override for multi-cloud (--provider).
// providerOverride is the key from providerOverrides in runfabric.yml (e.g. "aws", "gcp"); use "" for single-provider config.
func Bootstrap(configPath, stage, providerOverride string) (*AppContext, error) {
	reg := extproviders.NewRegistry()
	aws := awsprovider.New()
	reg.Register(aws)
	reg.Register(extproviders.NewNamedProvider("aws-lambda", aws))
	gcp := gcpprovider.New()
	reg.Register(gcp)
	RegisterAPIProviders(reg)
	// Phase 15c: register any discovered external provider plugins.
	if res, err := external.Discover(external.DiscoverOptions{}); err == nil {
		for _, m := range res.Plugins {
			if m.Kind != "provider" || m.Executable == "" {
				continue
			}
			// Default precedence: builtin wins. External providers can be forced via the registry
			// by explicitly preferring external in the CLI for extension inspection; for lifecycle,
			// use a provider name that is not built-in.
			if _, err := reg.Get(m.ID); err == nil {
				continue
			}
			reg.Register(external.NewExternalProviderAdapter(m.ID, m.Executable))
		}
	}

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

	if shouldAutoInstallExtensions(cfg) {
		// Provider plugin: required by lifecycle operations.
		if err := maybeInstallMissingProvider(reg, cfg, configPath); err != nil {
			return nil, err
		}
		// Other plugin kinds: best-effort install when explicitly referenced in runfabric.yml.
		// These are not required for core lifecycle today but are useful for consistent behavior.
		if id := config.ExtensionString(cfg, "runtimePlugin"); id != "" {
			if err := maybeInstallMissingPlugin(configPath, manifests.KindRuntime, id); err != nil {
				return nil, err
			}
		}
		if id := config.ExtensionString(cfg, "simulatorPlugin"); id != "" {
			if err := maybeInstallMissingPlugin(configPath, manifests.KindSimulator, id); err != nil {
				return nil, err
			}
		}
	}

	rootDir := filepath.Dir(configPath)

	backendKind := os.Getenv("RUNFABRIC_BACKEND")
	s3Bucket := os.Getenv("RUNFABRIC_S3_BUCKET")
	s3Prefix := os.Getenv("RUNFABRIC_S3_PREFIX")
	dynamoTable := os.Getenv("RUNFABRIC_DYNAMODB_TABLE")

	if cfg.Backend != nil {
		if cfg.Backend.Kind != "" {
			backendKind = cfg.Backend.Kind
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
	}

	postgresDSN := ""
	postgresTable := ""
	sqlitePath := ""
	receiptTable := ""
	if cfg.Backend != nil {
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

	opts := backends.Options{
		Kind:            backendKind,
		Root:            rootDir,
		AWSRegion:       cfg.Provider.Region,
		S3Bucket:        s3Bucket,
		S3Prefix:        s3Prefix,
		DynamoTableName: dynamoTable,
		PostgresDSN:     postgresDSN,
		PostgresTable:   postgresTable,
		SqlitePath:      sqlitePath,
		ReceiptTable:    receiptTable,
	}
	bundle, err := backends.NewBundle(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	return &AppContext{
		Config:   cfg,
		Registry: reg,
		RootDir:  rootDir,
		Stage:    stage,
		Backends: bundle,
	}, nil
}

func shouldAutoInstallExtensions(cfg *config.Config) bool {
	if isTruthyEnv(envAutoInstallExtension) {
		return true
	}
	return config.AutoInstallExtensions(cfg)
}

func isTruthyEnv(k string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	return v == "1" || v == "true" || v == "yes"
}

func maybeInstallMissingProvider(reg *providers.Registry, cfg *config.Config, configPath string) error {
	name := strings.TrimSpace(cfg.Provider.Name)
	if name == "" {
		return nil
	}
	if _, err := reg.Get(name); err == nil {
		return nil
	}

	// Attempt install (if allowed). If it fails or is declined, keep the original error behavior.
	if err := maybeInstallMissingPlugin(configPath, manifests.KindProvider, name); err != nil {
		return err
	}

	// Re-discover to find executable path, then register adapter.
	res, _ := external.Discover(external.DiscoverOptions{})
	for _, m := range res.Plugins {
		if m.Kind == "provider" && m.ID == name && strings.TrimSpace(m.Executable) != "" {
			reg.Register(external.NewExternalProviderAdapter(name, m.Executable))
			break
		}
	}
	if _, err := reg.Get(name); err != nil {
		return err
	}
	return nil
}

func maybeInstallMissingPlugin(configPath string, kind manifests.PluginKind, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	// Already installed?
	res, _ := external.Discover(external.DiscoverOptions{})
	for _, m := range res.Plugins {
		if m.Kind == kind && m.ID == id {
			return nil
		}
	}

	nonInteractive := isTruthyEnv(envNonInteractive)
	assumeYes := isTruthyEnv(envAssumeYes)

	registryURL := ""
	registryToken := ""
	if rc := external.LoadRunfabricrc(filepath.Dir(configPath)); rc.RegistryURL != "" {
		registryURL = rc.RegistryURL
	}
	if rc := external.LoadRunfabricrc(filepath.Dir(configPath)); rc.RegistryToken != "" {
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
	return nil
}

func pluginKindLabel(k manifests.PluginKind) string {
	switch k {
	case manifests.KindProvider:
		return "Provider"
	case manifests.KindRuntime:
		return "Runtime"
	case manifests.KindSimulator:
		return "Simulator"
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
	kind := os.Getenv("RUNFABRIC_BACKEND")
	s3Bucket := os.Getenv("RUNFABRIC_S3_BUCKET")
	s3Prefix := os.Getenv("RUNFABRIC_S3_PREFIX")
	dynamoTable := os.Getenv("RUNFABRIC_DYNAMODB_TABLE")
	postgresDSN := ""
	postgresTable := "runfabric_receipts"
	sqlitePath := ".runfabric/state.db"
	receiptTable := ""
	if ctx.Config.Backend != nil {
		if ctx.Config.Backend.Kind != "" {
			kind = ctx.Config.Backend.Kind
		}
		if ctx.Config.Backend.S3Bucket != "" {
			s3Bucket = ctx.Config.Backend.S3Bucket
		}
		if ctx.Config.Backend.S3Prefix != "" {
			s3Prefix = ctx.Config.Backend.S3Prefix
		}
		if ctx.Config.Backend.LockTable != "" {
			dynamoTable = ctx.Config.Backend.LockTable
		}
		postgresTable = ctx.Config.Backend.PostgresTable
		if postgresTable == "" {
			postgresTable = "runfabric_receipts"
		}
		sqlitePath = ctx.Config.Backend.SqlitePath
		if sqlitePath == "" {
			sqlitePath = ".runfabric/state.db"
		}
		receiptTable = ctx.Config.Backend.ReceiptTable
		if ctx.Config.Backend.Kind == "postgres" && ctx.Config.Backend.PostgresConnectionStringEnv != "" {
			postgresDSN = os.Getenv(ctx.Config.Backend.PostgresConnectionStringEnv)
		}
	}
	if kindOverride != "" {
		kind = kindOverride
	}
	return backends.Options{
		Kind:            kind,
		Root:            ctx.RootDir,
		AWSRegion:       ctx.Config.Provider.Region,
		S3Bucket:        s3Bucket,
		S3Prefix:        s3Prefix,
		DynamoTableName: dynamoTable,
		PostgresDSN:     postgresDSN,
		PostgresTable:   postgresTable,
		SqlitePath:      sqlitePath,
		ReceiptTable:    receiptTable,
	}
}
