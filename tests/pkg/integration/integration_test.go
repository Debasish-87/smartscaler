package integration_test

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"smartscaler/pkg/config"
	"smartscaler/pkg/cost"
	"smartscaler/pkg/decision"
	"smartscaler/pkg/scaler"
	"smartscaler/pkg/telemetry"
)

func TestMain(m *testing.M) {
	telemetry.Register()
	m.Run()
}

func testCfg() *config.Config {
	return &config.Config{
		ScaleUpCPUThreshold:   120,
		ScaleDownCPUThreshold: 60,
		NodePressureThreshold: 800,
		HighCostThreshold:     0.15,
		CostPerNodePerHour:    0.096,
		CostPerPodPerHour:     0.01,
		CooldownDuration:      30 * time.Second,
		MaxScaleStep:          3,
	}
}

func makeDeploy(replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
}

func newFakeClient(objs ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objs...)
}

func doScale(t *testing.T, client kubernetes.Interface, desired int32) {
	t.Helper()
	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: desired, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("scale failed: %v", err)
	}
}

func getReplicas(t *testing.T, client kubernetes.Interface) int32 {
	t.Helper()
	deploy, err := client.AppsV1().Deployments("default").
		Get(context.Background(), "demo-app", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	return *deploy.Spec.Replicas
}

func TestIntegration_FullScaleUpCycle(t *testing.T) {
	cfg := testCfg()
	client := newFakeClient(makeDeploy(2))
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{AvgCPUMilli: 180, Current: 2, Min: 2, Max: 10})
	if d.Action != "scale_up" {
		t.Fatalf("expected scale_up, got %s", d.Action)
	}

	doScale(t, client, d.Replicas)

	if got := getReplicas(t, client); got != d.Replicas {
		t.Errorf("expected %d, got %d", d.Replicas, got)
	}
	t.Logf("✓ Scale-up: 2 → %d (CPU=180m)", d.Replicas)
}

func TestIntegration_FullScaleDownCycle(t *testing.T) {
	cfg := testCfg()
	client := newFakeClient(makeDeploy(8))
	engine := decision.NewEngine(cfg)

	d := engine.Decide(decision.Input{AvgCPUMilli: 40, Current: 8, Min: 2, Max: 10})
	if d.Action != "scale_down" && d.Action != "cost_optimized_scale_down" {
		t.Fatalf("expected scale_down or cost_optimized_scale_down, got %s", d.Action)
	}

	doScale(t, client, d.Replicas)

	if got := getReplicas(t, client); got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
	t.Logf("✓ Scale-down: 8 → %d (CPU=40m, action=%s)", d.Replicas, d.Action)
}

func TestIntegration_CostTrackingAfterScale(t *testing.T) {
	cfg := testCfg()
	estimator := cost.NewEstimator(cfg)

	costBefore := estimator.TotalCost(1, 2)
	costAfter := estimator.TotalCost(1, 8)

	if costAfter <= costBefore {
		t.Errorf("cost should increase: before=%.4f after=%.4f", costBefore, costAfter)
	}

	engine := decision.NewEngine(cfg)
	d := engine.Decide(decision.Input{
		AvgCPUMilli: 40,
		Cost:        costAfter,
		Current:     8,
		Min:         2,
		Max:         10,
	})

	if d.Action != "cost_optimized_scale_down" {
		t.Errorf("expected cost_optimized_scale_down, got %s (cost=%.4f threshold=%.4f)",
			d.Action, costAfter, cfg.HighCostThreshold)
	}
	t.Logf("✓ Cost: before=%.4f after=%.4f action=%s", costBefore, costAfter, d.Action)
}

func TestIntegration_BoundsNeverViolated(t *testing.T) {
	cfg := testCfg()
	engine := decision.NewEngine(cfg)
	client := newFakeClient(makeDeploy(2))

	cases := []struct {
		cpu     int64
		current int32
	}{
		{200, 10}, {10, 2}, {500, 10}, {0, 2},
	}

	for _, c := range cases {
		d := engine.Decide(decision.Input{
			AvgCPUMilli: c.cpu, Current: c.current, Min: 2, Max: 10,
		})
		if d.Replicas < 2 || d.Replicas > 10 {
			t.Errorf("BOUNDS VIOLATED: cpu=%d current=%d → replicas=%d",
				c.cpu, c.current, d.Replicas)
		}
		doScale(t, client, d.Replicas)
	}
	t.Log("✓ Bounds never violated")
}

func TestIntegration_DecisionIdempotent(t *testing.T) {
	cfg := testCfg()
	engine := decision.NewEngine(cfg)

	input := decision.Input{AvgCPUMilli: 180, Current: 3, Min: 2, Max: 10}
	first := engine.Decide(input)

	for i := 0; i < 5; i++ {
		d := engine.Decide(input)
		if d.Action != first.Action || d.Replicas != first.Replicas {
			t.Errorf("run %d not idempotent: got %s/%d want %s/%d",
				i, d.Action, d.Replicas, first.Action, first.Replicas)
		}
	}
	t.Logf("✓ Idempotent: always %s → %d", first.Action, first.Replicas)
}
