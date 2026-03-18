package resources

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Remover deletes the namespace (cascades to deployment and service).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	restConfig, err := loadKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	namespace := receipt.Metadata["namespace"]
	if namespace == "" {
		namespace = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	if err := clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil && !isAlreadyGone(err) {
		return nil, fmt.Errorf("kubernetes delete namespace: %w", err)
	}
	return &providers.RemoveResult{Provider: "kubernetes", Removed: true}, nil
}

func isAlreadyGone(err error) bool {
	s := err.Error()
	return strings.Contains(s, "NotFound") || strings.Contains(s, "not found")
}
