package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"smartscaler/pkg/client"
)

var (
	nodeMetricsClient *metricsclient.Clientset
	nodeClientOnce    sync.Once

	nodeHistory []int64
	nodeMu      sync.Mutex
)

func getNodeMetricsClient() (*metricsclient.Clientset, error) {
	var initErr error
	nodeClientOnce.Do(func() {
		cfg := client.GetKubeConfig()
		nodeMetricsClient, initErr = metricsclient.NewForConfig(cfg)
	})
	return nodeMetricsClient, initErr
}

type NodeCPUResult struct {
	SmoothedAvgMilliCPU int64
	RawAvgMilliCPU      int64
	ValidNodes          int
}

func GetNodeCPU() (NodeCPUResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mc, err := getNodeMetricsClient()
	if err != nil {
		return NodeCPUResult{}, fmt.Errorf("metrics client: %w", err)
	}

	nodes, err := mc.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return NodeCPUResult{}, fmt.Errorf("node metrics fetch: %w", err)
	}
	if len(nodes.Items) == 0 {
		return NodeCPUResult{}, fmt.Errorf("no node metrics available")
	}

	var totalCPU int64
	validNodes := 0
	for _, n := range nodes.Items {
		if time.Since(n.Timestamp.Time) > metricsStaleness {
			continue
		}
		totalCPU += n.Usage.Cpu().MilliValue()
		validNodes++
	}
	if validNodes == 0 {
		return NodeCPUResult{}, fmt.Errorf("no fresh node metrics")
	}

	rawAvg := totalCPU / int64(validNodes)

	nodeMu.Lock()
	nodeHistory = append(nodeHistory, rawAvg)
	if len(nodeHistory) > historyWindow {
		nodeHistory = nodeHistory[1:]
	}
	var sum int64
	for _, v := range nodeHistory {
		sum += v
	}
	smoothed := sum / int64(len(nodeHistory))
	nodeMu.Unlock()

	return NodeCPUResult{
		SmoothedAvgMilliCPU: smoothed,
		RawAvgMilliCPU:      rawAvg,
		ValidNodes:          validNodes,
	}, nil
}
