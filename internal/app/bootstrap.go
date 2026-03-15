package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/internal/backends"
	"github.com/runfabric/runfabric/internal/config"
	appErrs "github.com/runfabric/runfabric/internal/errors"
	"github.com/runfabric/runfabric/internal/planner"
	"github.com/runfabric/runfabric/internal/providers"
	awsprovider "github.com/runfabric/runfabric/providers/aws"
)

type AppContext struct {
	Config   *config.Config
	Registry *providers.Registry
	RootDir  string
	Stage    string
	Backends *backends.Bundle
}

func Bootstrap(configPath, stage string) (*AppContext, error) {
	reg := providers.NewRegistry()
	// AWS: register under both "aws" (legacy) and "aws-lambda" (docs / Trigger Capability Matrix)
	aws := awsprovider.New()
	reg.Register(aws)
	reg.Register(providers.NewNamedProvider("aws-lambda", aws))
	// Fallback providers for matrix providers without a full implementation; doctor/plan/deploy/remove/invoke/logs still resolve.
	for _, name := range matrixProviderNames() {
		if name == "aws-lambda" {
			continue // already registered real AWS above
		}
		reg.Register(providers.NewStubProvider(name))
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigLoad, "failed to load config", err)
	}

	cfg, err = config.Resolve(cfg, stage)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigResolve, "failed to resolve config", err)
	}

	if err := config.Validate(cfg); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeConfigValidation, "config validation failed", err)
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

	bundle, err := backends.NewBundle(context.Background(), backends.Options{
		Kind:            backendKind,
		Root:            rootDir,
		AWSRegion:       cfg.Provider.Region,
		S3Bucket:        s3Bucket,
		S3Prefix:        s3Prefix,
		DynamoTableName: dynamoTable,
	})
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

// matrixProviderNames returns provider names from the Trigger Capability Matrix (for registration).
func matrixProviderNames() []string {
	names := make([]string, 0, len(planner.ProviderCapabilities))
	for name := range planner.ProviderCapabilities {
		names = append(names, name)
	}
	return names
}
