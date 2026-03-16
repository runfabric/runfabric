package resources

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Logger fetches pod logs via client-go (same cluster as deploy).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	namespace := receipt.Metadata["namespace"]
	if namespace == "" {
		namespace = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	restConfig, err := loadKubeconfig()
	if err != nil {
		return &providers.LogsResult{
			Provider: "kubernetes",
			Function: function,
			Lines:    []string{"kubeconfig: " + err.Error() + " (use: kubectl logs -n " + namespace + " -l app=" + cfg.Service + " --tail=100)"},
		}, nil
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return &providers.LogsResult{Provider: "kubernetes", Function: function, Lines: []string{"kubernetes client: " + err.Error()}}, nil
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=" + cfg.Service})
	if err != nil {
		return &providers.LogsResult{Provider: "kubernetes", Function: function, Lines: []string{"list pods: " + err.Error()}}, nil
	}
	if len(pods.Items) == 0 {
		return &providers.LogsResult{Provider: "kubernetes", Function: function, Lines: []string{"No pods found for app=" + cfg.Service + " in namespace " + namespace}}, nil
	}
	pod := pods.Items[0]
	containerName := "app"
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}
	opts := &corev1.PodLogOptions{Container: containerName, TailLines: int64Ptr(100)}
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts).Stream(ctx)
	if err != nil {
		return &providers.LogsResult{Provider: "kubernetes", Function: function, Lines: []string{"get logs: " + err.Error()}}, nil
	}
	defer stream.Close()
	var lines []string
	sc := bufio.NewScanner(stream)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r\n"))
	}
	if len(lines) == 0 {
		lines = []string{"No log output (pod may still be starting)."}
	}
	return &providers.LogsResult{Provider: "kubernetes", Function: function, Lines: lines}, nil
}

func int64Ptr(n int64) *int64 { return &n }
