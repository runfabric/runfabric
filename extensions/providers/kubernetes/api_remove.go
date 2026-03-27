package kubernetes

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the namespace (cascades to deployment and service).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	service := sdkprovider.Service(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	restConfig, err := loadKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	namespace := rv.Metadata["namespace"]
	if namespace == "" {
		namespace = fmt.Sprintf("%s-%s", service, stage)
	}
	if err := clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil && !isAlreadyGone(err) {
		return nil, fmt.Errorf("kubernetes delete namespace: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "kubernetes", Removed: true}, nil
}

func isAlreadyGone(err error) bool {
	s := err.Error()
	return strings.Contains(s, "NotFound") || strings.Contains(s, "not found")
}
