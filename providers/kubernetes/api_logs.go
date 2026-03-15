package resources

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Logger returns the kubectl command to fetch logs (K8s logs require cluster access).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	namespace := receipt.Metadata["namespace"]
	if namespace == "" {
		namespace = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	cmd := fmt.Sprintf("kubectl logs -n %s -l app=%s --tail=100", namespace, cfg.Service)
	return &providers.LogsResult{
		Provider: "kubernetes",
		Function: function,
		Lines:    []string{"Fetch logs with: " + cmd},
	}, nil
}
