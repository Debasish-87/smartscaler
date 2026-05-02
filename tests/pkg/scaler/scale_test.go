package scaler_test

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"smartscaler/pkg/scaler"
	"smartscaler/pkg/telemetry"
)

func TestMain(m *testing.M) {
	telemetry.Register()
	m.Run()
}

func makeDeployment(name, namespace string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
}

func fakeClient(objs ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objs...)
}

func TestScale_ScaleUp(t *testing.T) {
	client := fakeClient(makeDeployment("demo-app", "default", 2))

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: 5, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy, _ := client.AppsV1().Deployments("default").
		Get(context.Background(), "demo-app", metav1.GetOptions{})
	if *deploy.Spec.Replicas != 5 {
		t.Errorf("expected 5, got %d", *deploy.Spec.Replicas)
	}
}

func TestScale_ScaleDown(t *testing.T) {
	client := fakeClient(makeDeployment("demo-app", "default", 10))

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: 2, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy, _ := client.AppsV1().Deployments("default").
		Get(context.Background(), "demo-app", metav1.GetOptions{})
	if *deploy.Spec.Replicas != 2 {
		t.Errorf("expected 2, got %d", *deploy.Spec.Replicas)
	}
}

func TestScale_NoOp_AlreadyAtDesired(t *testing.T) {
	client := fakeClient(makeDeployment("demo-app", "default", 5))

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: 5, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("no-op should not error: %v", err)
	}
}

func TestScale_InvalidNegativeReplicas(t *testing.T) {
	client := fakeClient(makeDeployment("demo-app", "default", 3))

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: -1, Context: context.Background(),
	})
	if err == nil {
		t.Error("expected error for negative replicas")
	}
}

func TestScale_DeploymentNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "does-not-exist", Desired: 3, Context: context.Background(),
	})
	if err == nil {
		t.Error("expected error for missing deployment")
	}
}

func TestScale_ZeroReplicas_Allowed(t *testing.T) {
	client := fakeClient(makeDeployment("demo-app", "default", 3))

	err := scaler.Scale(scaler.ScaleRequest{
		Clientset: client, Namespace: "default",
		DeploymentName: "demo-app", Desired: 0, Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("scale to 0 should be allowed: %v", err)
	}

	deploy, _ := client.AppsV1().Deployments("default").
		Get(context.Background(), "demo-app", metav1.GetOptions{})
	if *deploy.Spec.Replicas != 0 {
		t.Errorf("expected 0, got %d", *deploy.Spec.Replicas)
	}
}
