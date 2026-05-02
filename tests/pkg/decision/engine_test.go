package decision_test

import (
	"testing"

	"smartscaler/pkg/config"
	"smartscaler/pkg/decision"
)

func testConfig() *config.Config {
	return &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60,
		NodePressureThreshold: 800,
		HighCostThreshold:     0.20,
		MaxScaleStep:          3,
	}
}


func TestDecide_NoChange_WithinThreshold(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 90, 
		Current:     5,
		Min:         2,
		Max:         10,
	})

	if d.Action != "no_change" {
		t.Errorf("expected no_change, got %s (reason: %s)", d.Action, d.Reason)
	}
	if d.Replicas != 5 {
		t.Errorf("expected replicas=5, got %d", d.Replicas)
	}
}

func TestDecide_ScaleUp_HighCPU(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 150, 
		Current:     2,
		Min:         2,
		Max:         10,
	})

	if d.Action != "scale_up" {
		t.Errorf("expected scale_up, got %s", d.Action)
	}
	if d.Replicas != 3 { 
		t.Errorf("expected replicas=3, got %d", d.Replicas)
	}
	if d.Reason != "high_avg_cpu" {
		t.Errorf("expected reason=high_avg_cpu, got %s", d.Reason)
	}
}

func TestDecide_ScaleUp_AggressiveStep_2x(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 185, 
		Current:     2,
		Min:         2,
		Max:         10,
	})

	if d.Action != "scale_up" {
		t.Errorf("expected scale_up, got %s", d.Action)
	}
	if d.Replicas != 4 { 
		t.Errorf("expected replicas=4 (step=2), got %d", d.Replicas)
	}
}

func TestDecide_ScaleUp_AggressiveStep_MaxStep(t *testing.T) {

	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 250, 
		Current:     2,
		Min:         2,
		Max:         10,
	})

	if d.Replicas != 5 { 
		t.Errorf("expected replicas=5 (step=3), got %d", d.Replicas)
	}
}

func TestDecide_ScaleUp_CappedAtMax(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 150,
		Current:     10, 
		Min:         2,
		Max:         10,
	})

	if d.Action != "no_change" {
		t.Errorf("at max replicas, should not scale_up, got %s", d.Action)
	}
	if d.Replicas != 10 {
		t.Errorf("replicas should stay at max=10, got %d", d.Replicas)
	}
}

func TestDecide_ScaleDown_LowCPU(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 30, 
		Current:     5,
		Min:         2,
		Max:         10,
	})

	if d.Action != "scale_down" {
		t.Errorf("expected scale_down, got %s", d.Action)
	}
	if d.Replicas != 4 { 
		t.Errorf("expected replicas=4, got %d", d.Replicas)
	}
}

func TestDecide_ScaleDown_CappedAtMin(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 30,
		Current:     2, 
		Min:         2,
		Max:         10,
	})

	if d.Action != "no_change" {
		t.Errorf("at min replicas, should not scale_down, got %s", d.Action)
	}
	if d.Replicas != 2 {
		t.Errorf("replicas should stay at min=2, got %d", d.Replicas)
	}
}

func TestDecide_CostOptimizedScaleDown(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 30,   
		Cost:        0.50, 
		Current:     8,
		Min:         2,
		Max:         10,
	})

	if d.Action != "cost_optimized_scale_down" {
		t.Errorf("expected cost_optimized_scale_down, got %s", d.Action)
	}
	if d.Reason != "low_cpu_high_cost" {
		t.Errorf("expected reason=low_cpu_high_cost, got %s", d.Reason)
	}
}

func TestDecide_NodePressureScaleUp(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli:  90,   
		NodeCPUMilli: 900,  
		Current:      3,
		Min:          2,
		Max:          10,
	})

	if d.Action != "node_pressure_scale" {
		t.Errorf("expected node_pressure_scale, got %s", d.Action)
	}
	if d.Reason != "node_cpu_high" {
		t.Errorf("expected reason=node_cpu_high, got %s", d.Reason)
	}
}

func TestDecide_CorrectMinViolation(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 90,
		Current:     1, 
		Min:         2,
		Max:         10,
	})

	if d.Action != "correct_min_violation" {
		t.Errorf("expected correct_min_violation, got %s", d.Action)
	}
	if d.Replicas != 2 {
		t.Errorf("expected replicas=2, got %d", d.Replicas)
	}
}

func TestDecide_CorrectMaxViolation(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 90,
		Current:     15, 
		Min:         2,
		Max:         10,
	})

	if d.Action != "correct_max_violation" {
		t.Errorf("expected correct_max_violation, got %s", d.Action)
	}
	if d.Replicas != 10 {
		t.Errorf("expected replicas=10, got %d", d.Replicas)
	}
}


func TestDecide_TableDriven(t *testing.T) {
	engine := decision.NewEngine(testConfig())

	tests := []struct {
		name           string
		input          decision.Input
		expectedAction string
		expectedReplicas int32
	}{
		{
			name:             "exact_at_scaleup_threshold_no_change",
			input:            decision.Input{AvgCPUMilli: 120, Current: 5, Min: 2, Max: 10},
			expectedAction:   "no_change",
			expectedReplicas: 5,
		},
		{
			name:             "exact_at_scaledown_threshold_no_change",
			input:            decision.Input{AvgCPUMilli: 60, Current: 5, Min: 2, Max: 10},
			expectedAction:   "no_change",
			expectedReplicas: 5,
		},
		{
			name:             "single_replica_min_max",
			input:            decision.Input{AvgCPUMilli: 200, Current: 1, Min: 1, Max: 1},
			expectedAction:   "no_change",
			expectedReplicas: 1,
		},
		{
			name:             "zero_cpu_scale_down",
			input:            decision.Input{AvgCPUMilli: 0, Current: 5, Min: 2, Max: 10},
			expectedAction:   "scale_down",
			expectedReplicas: 4,
		},
		{
			name:             "scaleup_step_clamped_to_max",
			input:            decision.Input{AvgCPUMilli: 300, Current: 9, Min: 2, Max: 10},
			expectedAction:   "scale_up",
			expectedReplicas: 10, 
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := engine.Decide(tt.input)
			if d.Action != tt.expectedAction {
				t.Errorf("action: got %s, want %s", d.Action, tt.expectedAction)
			}
			if d.Replicas != tt.expectedReplicas {
				t.Errorf("replicas: got %d, want %d", d.Replicas, tt.expectedReplicas)
			}
		})
	}
}