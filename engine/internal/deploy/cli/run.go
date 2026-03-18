// Package cli runs provider-native CLIs to perform deploys (wrangler, vercel, fly, gcloud, az, kubectl, etc.).
// Part of internal/deploy; prefer internal/deploy/api for API-based deploy (no CLI required).
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Run runs the provider's CLI to deploy and returns a DeployResult. Saves receipt to root.
func Run(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	runner, ok := runners[provider]
	if !ok {
		return nil, fmt.Errorf("deploy via CLI not implemented for provider %q; add runner in internal/deploy/cli", provider)
	}
	result, err := runner.Deploy(ctx, cfg, stage, root)
	if err != nil {
		return nil, err
	}
	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    result.Artifacts,
		Metadata:     result.Metadata,
		Functions:    make([]state.FunctionDeployment, 0, len(result.Artifacts)),
	}
	for _, a := range result.Artifacts {
		receipt.Functions = append(receipt.Functions, state.FunctionDeployment{Function: a.Function})
	}
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}
	return result, nil
}

// Runner runs the provider CLI and returns a DeployResult.
type Runner interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
}

var runners = map[string]Runner{
	"cloudflare-workers":     &cloudflareRunner{},
	"vercel":                 &vercelRunner{},
	"netlify":                &netlifyRunner{},
	"fly-machines":           &flyRunner{},
	"gcp-functions":          &gcpRunner{},
	"azure-functions":        &azureRunner{},
	"kubernetes":             &kubernetesRunner{},
	"digitalocean-functions": &digitalOceanRunner{},
	"alibaba-fc":             &alibabaRunner{},
	"ibm-openwhisk":          &ibmRunner{},
}

// HasRunner returns whether the provider has a CLI-based deploy runner.
func HasRunner(provider string) bool {
	_, ok := runners[provider]
	return ok
}

func runCmd(ctx context.Context, root, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func extractURL(out string) string {
	// Common patterns: https://..., https://xxx.workers.dev, https://xxx.vercel.app, https://xxx.fly.dev
	re := regexp.MustCompile(`https://[^\s"'\)]+`)
	if m := re.FindString(out); m != "" {
		return m
	}
	return ""
}
