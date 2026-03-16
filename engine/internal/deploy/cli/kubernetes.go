package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
)

type kubernetesRunner struct{}

func (r *kubernetesRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	namespace := fmt.Sprintf("%s-%s", cfg.Service, stage)
	manifestPath := filepath.Join(root, "k8s.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Generate minimal deployment + service so kubectl apply works
		manifest := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: app
        image: node:20-alpine
        command: ["node", "src/index.js"]
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
  - port: 80
    targetPort: 8080
`, namespace, cfg.Service, namespace, cfg.Service, cfg.Service, cfg.Service, namespace, cfg.Service)
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
			return nil, fmt.Errorf("write k8s.yaml: %w", err)
		}
	}
	stdout, stderr, err := runCmd(ctx, root, "kubectl", "apply", "-f", manifestPath)
	if err != nil {
		return nil, fmt.Errorf("kubectl apply: %w\nstderr: %s", err, stderr)
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
		Provider:     "kubernetes",
		DeploymentID: fmt.Sprintf("k8s-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"namespace": namespace, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"namespace": namespace},
	}, nil
}
