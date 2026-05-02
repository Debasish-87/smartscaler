package cost_test

import (
	"math"
	"testing"

	"smartscaler/pkg/config"
	"smartscaler/pkg/cost"
)

func testEstimator() *cost.Estimator {
	cfg := &config.Config{
		CostPerNodePerHour: 0.096,
		CostPerPodPerHour:  0.01,
		HighCostThreshold:  0.20,
	}
	return cost.NewEstimator(cfg)
}

const floatTolerance = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}


func TestNodeCost(t *testing.T) {
	e := testEstimator()

	tests := []struct {
		nodes    int64
		expected float64
	}{
		{1, 0.096},
		{2, 0.192},
		{0, 0.0},
		{10, 0.96},
	}

	for _, tt := range tests {
		got := e.NodeCost(tt.nodes)
		if !almostEqual(got, tt.expected) {
			t.Errorf("NodeCost(%d): got %.6f, want %.6f", tt.nodes, got, tt.expected)
		}
	}
}

func TestPodCost(t *testing.T) {
	e := testEstimator()

	tests := []struct {
		pods     int32
		expected float64
	}{
		{1, 0.01},
		{5, 0.05},
		{0, 0.0},
		{10, 0.10},
	}

	for _, tt := range tests {
		got := e.PodCost(tt.pods)
		if !almostEqual(got, tt.expected) {
			t.Errorf("PodCost(%d): got %.6f, want %.6f", tt.pods, got, tt.expected)
		}
	}
}

func TestTotalCost(t *testing.T) {
	e := testEstimator()

	got := e.TotalCost(1, 5)
	expected := 0.146
	if !almostEqual(got, expected) {
		t.Errorf("TotalCost(1,5): got %.6f, want %.6f", got, expected)
	}
}

func TestEfficiency_Normal(t *testing.T) {
	e := testEstimator()

	got := e.Efficiency(200, 10)
	expected := 2000.0
	if !almostEqual(got, expected) {
		t.Errorf("Efficiency(200, 10): got %.2f, want %.2f", got, expected)
	}
}

func TestEfficiency_ZeroPods(t *testing.T) {
	e := testEstimator()
	got := e.Efficiency(200, 0)
	if got != 0 {
		t.Errorf("Efficiency with 0 pods should be 0, got %.2f", got)
	}
}

func TestWastedCost_FullUtilization(t *testing.T) {
	e := testEstimator()

	got := e.WastedCost(200, 200, 1, 5)
	if got != 0 {
		t.Errorf("WastedCost at full utilization should be 0, got %.6f", got)
	}
}

func TestWastedCost_ZeroUtilization(t *testing.T) {
	e := testEstimator()

	total := e.TotalCost(1, 5)
	got := e.WastedCost(0, 200, 1, 5)
	if !almostEqual(got, total) {
		t.Errorf("WastedCost at 0 utilization should equal total=%.6f, got %.6f", total, got)
	}
}

func TestWastedCost_ZeroCPULimit(t *testing.T) {
	e := testEstimator()
	got := e.WastedCost(100, 0, 1, 5)
	if got != 0 {
		t.Errorf("WastedCost with 0 limit should be 0, got %.6f", got)
	}
}

func TestOptimalReplicas(t *testing.T) {
	e := testEstimator()

	tests := []struct {
		name        string
		totalCPU    int64
		targetCPU   int64
		current     int32
		min, max    int32
		expectedRep int32
	}{
		{"normal", 400, 100, 4, 2, 10, 4},      
		{"scale_up", 600, 100, 4, 2, 10, 6},    
		{"clamped_to_max", 2000, 100, 4, 2, 10, 10},
		{"clamped_to_min", 50, 100, 4, 2, 10, 2},
		{"zero_target", 400, 0, 4, 2, 10, 4},   
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.OptimalReplicas(tt.totalCPU, tt.targetCPU, tt.current, tt.min, tt.max)
			if got != tt.expectedRep {
				t.Errorf("OptimalReplicas: got %d, want %d", got, tt.expectedRep)
			}
		})
	}
}