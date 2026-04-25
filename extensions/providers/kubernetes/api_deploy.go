// Package resources (providers/kubernetes) implements internal/deploy/api Runner/Remover/Invoker/Logger for Kubernetes.
package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Runner deploys by creating namespace, Deployment, and Service via client-go.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	restConfig, err := loadKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	namespace := fmt.Sprintf("%s-%s", service, stage)
	for _, provKey := range []string{"Provider", "provider"} {
		if pm, _ := cfg[provKey].(map[string]any); pm != nil {
			for _, nsKey := range []string{"Namespace", "namespace"} {
				if v, ok := pm[nsKey].(string); ok && v != "" {
					namespace = v
					break
				}
			}
			break
		}
	}
	appName := service

	image := ""
	for _, provKey := range []string{"Provider", "provider"} {
		if pm, _ := cfg[provKey].(map[string]any); pm != nil {
			for _, imgKey := range []string{"Image", "image"} {
				if v, ok := pm[imgKey].(string); ok && v != "" {
					image = v
					break
				}
			}
			break
		}
	}
	if image == "" {
		image = sdkprovider.Env("KUBERNETES_IMAGE")
	}

	// If no explicit image and runtime is nodejs, build + push a project image.
	runtime := sdkprovider.ProviderRuntime(cfg)
	if image == "" && strings.EqualFold(runtime, "nodejs") && root != "" {
		built, buildErr := buildAndPushImage(ctx, root, cfg, service, stage)
		if buildErr != nil {
			return nil, fmt.Errorf("build image: %w", buildErr)
		}
		image = built
	}

	if image == "" {
		image = "nginx:alpine"
	}
	containerPort := int32(80)

	// serviceType: yaml cfg takes precedence over env var, ClusterIP is the default.
	// After codec round-trip cfg["Provider"] holds the provider map with key "ServiceType".
	// Check both cases to be safe (matches how sdkprovider.ProviderRuntime works).
	svcType := corev1.ServiceTypeClusterIP
	for _, provKey := range []string{"Provider", "provider"} {
		if pm, _ := cfg[provKey].(map[string]any); pm != nil {
			for _, stKey := range []string{"ServiceType", "serviceType"} {
				if st, ok := pm[stKey].(string); ok && st != "" {
					svcType = corev1.ServiceType(st)
					break
				}
			}
			break
		}
	}
	if svcType == corev1.ServiceTypeClusterIP {
		if st := sdkprovider.Env("KUBERNETES_SERVICE_TYPE"); st != "" {
			svcType = corev1.ServiceType(st)
		}
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return nil, fmt.Errorf("create namespace: %w", err)
	}

	// Create or update GHCR pull secret when a token is available.
	if dockerCfg := ghcrDockerConfigJSON(); dockerCfg != "" {
		pullSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: pullSecretName, Namespace: namespace},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(dockerCfg)},
		}
		_, psErr := clientset.CoreV1().Secrets(namespace).Create(ctx, pullSecret, metav1.CreateOptions{})
		if isAlreadyExists(psErr) {
			existing, getErr := clientset.CoreV1().Secrets(namespace).Get(ctx, pullSecretName, metav1.GetOptions{})
			if getErr == nil {
				pullSecret.ResourceVersion = existing.ResourceVersion
				_, psErr = clientset.CoreV1().Secrets(namespace).Update(ctx, pullSecret, metav1.UpdateOptions{})
			}
		}
		if psErr != nil {
			return nil, fmt.Errorf("pull secret: %w", psErr)
		}
	}
	// Read deploy strategy.
	strategy := ""
	for _, dk := range []string{"Deploy", "deploy"} {
		if dm, _ := cfg[dk].(map[string]any); dm != nil {
			for _, sk := range []string{"Strategy", "strategy"} {
				if s, ok := dm[sk].(string); ok {
					strategy = s
					break
				}
			}
			break
		}
	}

	domain := readDomain(cfg)

	result := sdkprovider.BuildDeployResult("kubernetes", cfg, stage)
	result.Outputs["namespace"] = namespace
	result.Outputs["context"] = restConfig.Host
	result.Metadata["namespace"] = namespace
	result.Metadata["serviceType"] = string(svcType)

	if strategy == "per-function" {
		routes, _ := extractBuildRoutes(cfg)
		seen := map[string]bool{}
		for _, r := range routes {
			fnName := r.Handler
			if seen[fnName] {
				continue
			}
			seen[fnName] = true
			depName := appName + "-" + fnName
			if err := upsertDeployment(ctx, clientset, namespace, depName, image, containerPort,
				[]corev1.EnvVar{{Name: "RUNFABRIC_FN", Value: fnName}}); err != nil {
				return nil, fmt.Errorf("deployment %s: %w", fnName, err)
			}
			if err := upsertService(ctx, clientset, namespace, depName, svcType, containerPort); err != nil {
				return nil, fmt.Errorf("service %s: %w", fnName, err)
			}
			if err := waitUntilDeploymentReady(ctx, clientset, namespace, depName); err != nil {
				return nil, fmt.Errorf("wait %s: %w", fnName, err)
			}
			if domain == "" && svcType == corev1.ServiceTypeLoadBalancer {
				if ip, err := waitUntilServiceHasIP(ctx, clientset, namespace, depName); err == nil {
					result.Outputs["url."+fnName] = "http://" + ip
				}
			}
		}
		if domain != "" {
			ingressName := appName + "-ingress"
			paths := buildIngressRules(appName, routes)
			if err := upsertIngress(ctx, clientset, namespace, ingressName, domain, paths); err != nil {
				return nil, fmt.Errorf("ingress: %w", err)
			}
			if ip, err := waitUntilIngressHasIP(ctx, clientset, namespace, ingressName); err == nil {
				result.Outputs["ingress.ip"] = ip
			}
			result.Outputs["url"] = "http://" + domain
		}
	} else {
		if err := upsertDeployment(ctx, clientset, namespace, appName, image, containerPort, nil); err != nil {
			return nil, fmt.Errorf("deployment: %w", err)
		}
		if err := upsertService(ctx, clientset, namespace, appName, svcType, containerPort); err != nil {
			return nil, fmt.Errorf("service: %w", err)
		}
		if err := waitUntilDeploymentReady(ctx, clientset, namespace, appName); err != nil {
			return nil, fmt.Errorf("wait for deployment %s: %w", appName, err)
		}
		if domain != "" {
			ingressName := appName + "-ingress"
			prefix := networkingv1.PathTypePrefix
			paths := []networkingv1.HTTPIngressPath{{
				Path:     "/",
				PathType: &prefix,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: appName,
						Port: networkingv1.ServiceBackendPort{Number: 80},
					},
				},
			}}
			if err := upsertIngress(ctx, clientset, namespace, ingressName, domain, paths); err != nil {
				return nil, fmt.Errorf("ingress: %w", err)
			}
			if ip, err := waitUntilIngressHasIP(ctx, clientset, namespace, ingressName); err == nil {
				result.Outputs["ingress.ip"] = ip
			}
			result.Outputs["url"] = "http://" + domain
		} else if svcType == corev1.ServiceTypeLoadBalancer {
			if ip, err := waitUntilServiceHasIP(ctx, clientset, namespace, appName); err == nil {
				result.Outputs["url"] = "http://" + ip
			}
		}
	}

	return result, nil
}

func upsertDeployment(ctx context.Context, clientset *kubernetes.Clientset, namespace, name, image string, port int32, env []corev1.EnvVar) error {
	one := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "app",
						Image: image,
						Ports: []corev1.ContainerPort{{ContainerPort: port}},
						Env:   env,
					}},
					ImagePullSecrets: pullSecrets(),
				},
			},
		},
	}
	_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
	if isAlreadyExists(err) {
		existing, getErr := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get: %w", getErr)
		}
		dep.ResourceVersion = existing.ResourceVersion
		_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	}
	return err
}

func upsertService(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string, svcType corev1.ServiceType, port int32) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: map[string]string{"app": name},
			Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intOrStr(port)}},
		},
	}
	_, err := clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if isAlreadyExists(err) {
		existing, getErr := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get: %w", getErr)
		}
		svc.ResourceVersion = existing.ResourceVersion
		svc.Spec.ClusterIP = existing.Spec.ClusterIP
		_, err = clientset.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	}
	return err
}

func loadKubeconfig() (*rest.Config, error) {
	kubeconfig := sdkprovider.Env("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(sdkprovider.Env("HOME"), ".kube", "config")
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
