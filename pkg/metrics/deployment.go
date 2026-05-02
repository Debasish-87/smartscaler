package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"smartscaler/pkg/client"
)

const (
	metricsStaleness = 30 * time.Second
	historyWindow    = 5
)

var (
	deployMetricsClient *metricsclient.Clientset
	deployClientOnce    sync.Once

	cpuHistory = make(map[string][]int64)
	deployMu   sync.Mutex
)

func getDeployMetricsClient() (*metricsclient.Clientset, error) {
	var initErr error
	deployClientOnce.Do(func() {
		cfg := client.GetKubeConfig()
		deployMetricsClient, initErr = metricsclient.NewForConfig(cfg)
	})
	return deployMetricsClient, initErr
}

type DeploymentCPUResult struct {
	SmoothedAvgMilliCPU int64
	RawAvgMilliCPU      int64
	TotalMilliCPU       int64
	ValidPods           int
}

func GetDeploymentCPU(
	clientset *kubernetes.Clientset,
	namespace, deployment string,
) (DeploymentCPUResult, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := namespace + "/" + deployment

	//  1. Deployment selector
	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deployment, metav1.GetOptions{})
	if err != nil {
		return DeploymentCPUResult{}, fmt.Errorf("get deployment: %w", err)
	}

	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{
		MatchLabels: deploy.Spec.Selector.MatchLabels,
	})

	//  2. Pods
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return DeploymentCPUResult{}, fmt.Errorf("list pods: %w", err)
	}
	if len(podList.Items) == 0 {
		return DeploymentCPUResult{}, fmt.Errorf("no running pods for %s/%s", namespace, deployment)
	}

	//  3. Metrics
	mc, err := getDeployMetricsClient()
	if err != nil {
		return DeploymentCPUResult{}, fmt.Errorf("metrics client: %w", err)
	}

	metricsList, err := mc.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return DeploymentCPUResult{}, fmt.Errorf("fetch pod metrics: %w", err)
	}

	//  4. Build pod-name → CPU map (fresh only)
	podCPU := make(map[string]int64, len(metricsList.Items))
	for _, m := range metricsList.Items {
		if time.Since(m.Timestamp.Time) > metricsStaleness {
			continue
		}
		var cpu int64
		for _, c := range m.Containers {
			cpu += c.Usage.Cpu().MilliValue()
		}
		podCPU[m.Name] = cpu
	}

	//  5. Aggregate
	var totalCPU int64
	validPods := 0
	for _, pod := range podList.Items {
		if cpu, ok := podCPU[pod.Name]; ok {
			totalCPU += cpu
			validPods++
		}
	}
	if validPods == 0 {
		return DeploymentCPUResult{}, fmt.Errorf("no fresh metrics for %s/%s", namespace, deployment)
	}

	rawAvg := totalCPU / int64(validPods)

	//  6. Moving average (thread-safe)
	deployMu.Lock()
	history := cpuHistory[key]
	history = append(history, rawAvg)
	if len(history) > historyWindow {
		history = history[1:]
	}
	cpuHistory[key] = history

	var sum int64
	for _, v := range history {
		sum += v
	}
	smoothed := sum / int64(len(history))
	deployMu.Unlock()

	return DeploymentCPUResult{
		SmoothedAvgMilliCPU: smoothed,
		RawAvgMilliCPU:      rawAvg,
		TotalMilliCPU:       totalCPU,
		ValidPods:           validPods,
	}, nil
}
