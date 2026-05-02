package cost

import (
	"smartscaler/pkg/config"
)

type Estimator struct {
	cfg *config.Config
}

func NewEstimator(cfg *config.Config) *Estimator {
	return &Estimator{cfg: cfg}
}

func (e *Estimator) NodeCost(nodeCount int64) float64 {
	return float64(nodeCount) * e.cfg.CostPerNodePerHour
}

func (e *Estimator) PodCost(podCount int32) float64 {
	return float64(podCount) * e.cfg.CostPerPodPerHour
}

func (e *Estimator) TotalCost(nodeCount int64, podCount int32) float64 {
	return e.NodeCost(nodeCount) + e.PodCost(podCount)
}

func (e *Estimator) Efficiency(avgCPUMilli int64, podCount int32) float64 {
	if podCount == 0 || e.cfg.CostPerPodPerHour == 0 {
		return 0
	}
	cpuPerPod := float64(avgCPUMilli) / float64(podCount)
	return cpuPerPod / e.cfg.CostPerPodPerHour
}

func (e *Estimator) WastedCost(avgCPUMilli, cpuLimitMilli int64, nodeCount int64, podCount int32) float64 {
	total := e.TotalCost(nodeCount, podCount)
	if cpuLimitMilli == 0 {
		return 0
	}
	utilization := float64(avgCPUMilli) / float64(cpuLimitMilli)
	if utilization > 1 {
		utilization = 1
	}
	return total * (1 - utilization)
}

func (e *Estimator) OptimalReplicas(totalCPUMilli, targetCPUMilli int64, current, min, max int32) int32 {
	if targetCPUMilli <= 0 {
		return current
	}
	optimal := int32((totalCPUMilli + targetCPUMilli - 1) / targetCPUMilli)
	if optimal < min {
		optimal = min
	}
	if optimal > max {
		optimal = max
	}
	return optimal
}
