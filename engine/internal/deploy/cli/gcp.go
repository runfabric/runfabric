package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type gcpRunner struct{}

func (r *gcpRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	region := cfg.Provider.Region
	if region == "" {
		region = "us-central1"
	}
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		entryPoint := "handler"
		if fn.Handler != "" {
			entryPoint = strings.Split(fn.Handler, ".")[0]
		}
		funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName)
		// gcloud functions deploy <name> --gen2 --runtime=nodejs20 --region=... --source=. --entry-point=...
		stdout, stderr, err := runCmd(ctx, root, "gcloud", "functions", "deploy", funcName,
			"--gen2", "--runtime=nodejs20", "--region="+region, "--source=.", "--entry-point="+entryPoint,
			"--trigger-http", "--allow-unauthenticated")
		if err != nil {
			return nil, fmt.Errorf("gcloud functions deploy %s: %w\nstderr: %s", fnName, err, stderr)
		}
		url := extractURL(stdout + stderr)
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
		_ = url
	}
	return &providers.DeployResult{
		Provider:     "gcp-functions",
		DeploymentID: fmt.Sprintf("gcp-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"region": region},
		Artifacts:    artifacts,
	}, nil
}
