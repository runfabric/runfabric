package kubernetes

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// waitUntilDeploymentReady polls until the Deployment has at least one available replica.
// Budget: 30 attempts × 5s = 2.5 minutes.
func waitUntilDeploymentReady(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	for attempt := 0; attempt < 30; attempt++ {
		dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get deployment %s/%s: %w", namespace, name, err)
		}
		if dep.Status.AvailableReplicas >= 1 {
			return nil
		}
		if attempt < 29 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return fmt.Errorf("timed out waiting for deployment %s/%s to have available replicas", namespace, name)
}
