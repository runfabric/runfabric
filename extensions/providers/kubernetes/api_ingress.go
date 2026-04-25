package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// buildIngressRules builds path-based Ingress rules for per-function deployments.
// Routes with the same stripped path prefix map to the first function seen for that prefix.
func buildIngressRules(appName string, routes []buildRoute) []networkingv1.HTTPIngressPath {
	prefix := networkingv1.PathTypePrefix
	seen := map[string]bool{}
	var paths []networkingv1.HTTPIngressPath
	for _, r := range routes {
		svcName := appName + "-" + r.Handler
		staticPath := stripPathParams(r.Path)
		key := staticPath + "|" + svcName
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, networkingv1.HTTPIngressPath{
			Path:     staticPath,
			PathType: &prefix,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: svcName,
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
		})
	}
	return paths
}

// stripPathParams returns the static path prefix up to the first path parameter.
// /todos/{id} → /todos/   /todos → /todos
func stripPathParams(path string) string {
	if idx := strings.Index(path, "{"); idx >= 0 {
		return path[:idx]
	}
	return path
}

// upsertIngress creates or updates a Kubernetes Ingress for the given host and paths.
func upsertIngress(ctx context.Context, clientset *kubernetes.Clientset, namespace, name, host string, paths []networkingv1.HTTPIngressPath) error {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"managed-by": "runfabric",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
					},
				},
			},
		},
	}
	_, err := clientset.NetworkingV1().Ingresses(namespace).Create(ctx, ing, metav1.CreateOptions{})
	if isAlreadyExists(err) {
		existing, getErr := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get: %w", getErr)
		}
		ing.ResourceVersion = existing.ResourceVersion
		_, err = clientset.NetworkingV1().Ingresses(namespace).Update(ctx, ing, metav1.UpdateOptions{})
	}
	return err
}

// waitUntilIngressHasIP polls until the Ingress controller assigns an external IP or hostname.
// Budget: 24 attempts × 5s = 2 minutes.
func waitUntilIngressHasIP(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, error) {
	for attempt := 0; attempt < 24; attempt++ {
		ing, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("get ingress %s/%s: %w", namespace, name, err)
		}
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				return lb.IP, nil
			}
			if lb.Hostname != "" {
				return lb.Hostname, nil
			}
		}
		if attempt < 23 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return "", fmt.Errorf("timed out waiting for Ingress IP on %s/%s", namespace, name)
}
