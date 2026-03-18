package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type cloudflareRunner struct{}

func (r *cloudflareRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	name := fmt.Sprintf("%s-%s", cfg.Service, stage)
	main := "src/index.js"
	if len(cfg.Functions) > 0 {
		for _, fn := range cfg.Functions {
			if fn.Handler != "" {
				main = strings.Replace(fn.Handler, ".", "/", 1) + ".js"
				if !strings.HasSuffix(main, ".js") {
					main += ".js"
				}
				break
			}
		}
	}
	wranglerToml := fmt.Sprintf(`name = "%s"
main = "%s"
compatibility_date = "2024-01-01"
`, name, main)
	tomlPath := filepath.Join(root, "wrangler.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		if err := os.WriteFile(tomlPath, []byte(wranglerToml), 0o644); err != nil {
			return nil, fmt.Errorf("write wrangler.toml: %w", err)
		}
	}
	stdout, stderr, err := runCmd(ctx, root, "npx", "wrangler", "deploy", "--minify")
	if err != nil {
		return nil, fmt.Errorf("wrangler deploy: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	if url == "" {
		url = fmt.Sprintf("https://%s.workers.dev", name)
	}
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
	}
	return &providers.DeployResult{
		Provider:     "cloudflare-workers",
		DeploymentID: fmt.Sprintf("cf-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"worker": name},
	}, nil
}
