// Package resources (providers/kubernetes) implements internal/deploy/api Runner/Remover/Invoker/Logger for Kubernetes.
package resources

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
)

// Runner deploys by creating namespace, Deployment, and Service via client-go.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	restConfig, err := loadKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	namespace := fmt.Sprintf("%s-%s", cfg.Service, stage)
	appName := cfg.Service
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return nil, fmt.Errorf("create namespace: %w", err)
	}
	one := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": appName}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": appName}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "app", Image: "node:20-alpine",
						Command: []string{"node", "src/index.js"},
						Ports:   []corev1.ContainerPort{{ContainerPort: 8080}},
					}},
				},
			},
		},
	}
	_, err = clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return nil, fmt.Errorf("create deployment: %w", err)
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": appName},
			Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intOrStr(8080)}},
		},
	}
	_, err = clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return nil, fmt.Errorf("create service: %w", err)
	}
	result := apiutil.BuildDeployResult("kubernetes", cfg, stage)
	result.Outputs["namespace"] = namespace
	result.Outputs["context"] = restConfig.Host
	result.Metadata["namespace"] = namespace
	_ = filepath.Join(root, "k8s.yaml")
	return result, nil
}

func loadKubeconfig() (*rest.Config, error) {
	kubeconfig := apiutil.Env("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(apiutil.Env("HOME"), ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return rest.InClusterConfig()
	}
	return cfg, nil
}

func isAlreadyExists(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "AlreadyExists") || strings.Contains(err.Error(), "already exists"))
}

func intOrStr(v int32) intstr.IntOrString { return intstr.FromInt(int(v)) }
