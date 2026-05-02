package regression_test

import (
	"testing"

	"smartscaler/pkg/config"
	"smartscaler/pkg/cost"
	"smartscaler/pkg/decision"
	"smartscaler/pkg/utils"
)

func TestRegression_BUG001_ThresholdTooLow(t *testing.T) {
	cfg := &config.Config{
		ScaleUpCPUThreshold:   120, 
		ScaleDownCPUThreshold: 60,
		MaxScaleStep:          3,
	}
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 50,
		Current:     2,
		Min:         2,
		Max:         10,
	})

	if d.Action == "scale_up" {
		t.Errorf("BUG-001 regression: CPU=50m should not trigger scale_up with threshold=120m, got action=%s", d.Action)
	}
}

func TestRegression_BUG002_ScaleDownNeverTriggered(t *testing.T) {
	cfg := &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60, 
		MaxScaleStep:          3,
	}
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 45,
		Current:     8,
		Min:         2,
		Max:         10,
	})

	if d.Action != "scale_down" {
		t.Errorf("BUG-002 regression: CPU=45m should trigger scale_down with threshold=60m, got action=%s", d.Action)
	}
}

func TestRegression_BUG003_ScaleUpAboveMax(t *testing.T) {
	cfg := &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60,
		MaxScaleStep:          3,
	}
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 200, 
		Current:     10,  
		Min:         2,
		Max:         10,
	})

	if d.Replicas > 10 {
		t.Errorf("BUG-003 regression: replicas should never exceed max=10, got %d", d.Replicas)
	}
	if d.Action == "scale_up" {
		t.Errorf("BUG-003 regression: should not attempt scale_up when already at max, got action=%s", d.Action)
	}
}

func TestRegression_BUG004_DuplicateFinalizers(t *testing.T) {
	finalizer := "autoscale.mycompany/finalizer"
	list := []string{finalizer}

	result := utils.AddUnique(list, finalizer)

	if len(result) != 1 {
		t.Errorf("BUG-004 regression: duplicate finalizer added, expected len=1, got %d", len(result))
	}
}

func TestRegression_BUG005_EfficiencyDivisionByZero(t *testing.T) {
	cfg := &config.Config{
		CostPerPodPerHour: 0.01,
	}
	e := cost.NewEstimator(cfg)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("BUG-005 regression: Efficiency() panicked with podCount=0: %v", r)
		}
	}()

	result := e.Efficiency(200, 0)
	if result != 0 {
		t.Errorf("BUG-005 regression: Efficiency with 0 pods should be 0, got %.2f", result)
	}
}

func TestRegression_BUG006_ScaleUpPriorityOrder(t *testing.T) {
	cfg := &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60,
		NodePressureThreshold: 800,
		MaxScaleStep:          3,
	}
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{
		AvgCPUMilli:  200, 
		NodeCPUMilli: 900, 
		Current:      3,
		Min:          2,
		Max:          10,
	})

	if d.Action != "scale_up" {
		t.Errorf("BUG-006 regression: pod CPU scale_up should have higher priority, got action=%s", d.Action)
	}
	if d.Reason != "high_avg_cpu" {
		t.Errorf("BUG-006 regression: expected reason=high_avg_cpu, got %s", d.Reason)
	}
}

func TestRegression_BUG007_ScaleDownTooBelowMin(t *testing.T) {
	cfg := &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60,
		MaxScaleStep:          3,
	}
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{
		AvgCPUMilli: 10,
		Current:     2,
		Min:         2,
		Max:         10,
	})

	if d.Replicas < 2 {
		t.Errorf("BUG-007 regression: replicas should never go below min=2, got %d", d.Replicas)
	}
}