package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/engine/internal/buildcache"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/runtime/build"
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// BuildResult is the result of a build run (shared by CLI build/package and plan/deploy paths).
type BuildResult struct {
	Artifacts []providers.Artifact `json:"artifacts"`
	CacheHit  []string             `json:"cacheHit,omitempty"` // function names that used cache
	Errors    []string             `json:"errors,omitempty"`   // per-function errors if any
}

// BuildOptions configures the shared build.
type BuildOptions struct {
	NoCache       bool   // ignore cache and force rebuild
	OutDir        string // if set, write zips here instead of .runfabric/build (package command)
	FunctionFilter string // if set, only build this function
}

// Build loads config from configPath, builds each function (or the one specified by opts.FunctionFilter)
// using the same runtime/build path as plan/deploy, and optionally uses buildcache to skip work.
// Project root is derived from configPath (filepath.Dir). Returns artifacts and any per-function errors.
func Build(configPath string, opts BuildOptions) (*BuildResult, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	projectRoot := filepath.Dir(configPath)

	defaultBuildDir := filepath.Join(projectRoot, ".runfabric", "build")
	if err := os.MkdirAll(defaultBuildDir, 0o755); err != nil {
		return nil, fmt.Errorf("create build dir: %w", err)
	}
	if opts.OutDir != "" {
		if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
			return nil, fmt.Errorf("create out dir: %w", err)
		}
	}

	var artifacts []providers.Artifact
	var cacheHit []string
	var errs []string

	for name, fn := range cfg.Functions {
		if opts.FunctionFilter != "" && name != opts.FunctionFilter {
			continue
		}
		hash, err := buildcache.HashForFunction(cfg, projectRoot, name)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !opts.NoCache {
			if path, ok := buildcache.Get(projectRoot, name, hash); ok && path != "" {
				if _, statErr := os.Stat(path); statErr == nil {
					cacheHit = append(cacheHit, name)
					outPath := path
					if opts.OutDir != "" {
						outPath = filepath.Join(opts.OutDir, name+".zip")
						if copyErr := copyFile(path, outPath); copyErr != nil {
							errs = append(errs, fmt.Sprintf("%s: copy to out: %v", name, copyErr))
							continue
						}
					}
					artifacts = append(artifacts, providers.Artifact{
						Function:   name,
						Runtime:    fn.Runtime,
						OutputPath: outPath,
					})
					continue
				}
			}
		}
		configSig, err := config.FunctionConfigSignature(fn)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: config signature: %v", name, err))
			continue
		}
		artifact, err := build.PackageNodeFunction(projectRoot, name, fn, configSig)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		_ = buildcache.Put(projectRoot, name, hash, artifact.OutputPath)
		outPath := artifact.OutputPath
		if opts.OutDir != "" {
			outPath = filepath.Join(opts.OutDir, name+".zip")
			if copyErr := copyFile(artifact.OutputPath, outPath); copyErr != nil {
				errs = append(errs, fmt.Sprintf("%s: copy to out: %v", name, copyErr))
				continue
			}
			artifact.OutputPath = outPath
		}
		artifacts = append(artifacts, *artifact)
	}

	return &BuildResult{
		Artifacts: artifacts,
		CacheHit:  cacheHit,
		Errors:    errs,
	}, nil
}
