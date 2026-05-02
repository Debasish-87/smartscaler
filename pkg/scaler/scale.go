package scaler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"smartscaler/pkg/logger"
	"smartscaler/pkg/telemetry"
)

type ScaleRequest struct {
	// Clientset      *kubernetes.Clientset
	Clientset      kubernetes.Interface
	Namespace      string
	DeploymentName string
	Desired        int32
	Context        context.Context
}

func Scale(req ScaleRequest) error {
	log := logger.L().With(
		zap.String("namespace", req.Namespace),
		zap.String("deployment", req.DeploymentName),
		zap.Int32("desired", req.Desired),
	)

	if req.Desired < 0 {
		return fmt.Errorf("invalid desired replicas: %d", req.Desired)
	}

	ctx, cancel := context.WithTimeout(req.Context, 10*time.Second)
	defer cancel()

	err := retry.RetryOnConflict(wait.Backoff{
		Steps:    5,
		Duration: 200 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	}, func() error {
		latest, err := req.Clientset.AppsV1().
			Deployments(req.Namespace).
			Get(ctx, req.DeploymentName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get deployment: %w", err)
		}

		current := int32(1)
		if latest.Spec.Replicas != nil {
			current = *latest.Spec.Replicas
		}

		if current == req.Desired {
			log.Debug("no-op: already at desired replicas", zap.Int32("current", current))
			return nil
		}

		latest.Spec.Replicas = &req.Desired

		_, err = req.Clientset.AppsV1().
			Deployments(req.Namespace).
			Update(ctx, latest, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		telemetry.ScaleErrors.WithLabelValues(req.Namespace, req.DeploymentName).Inc()
		log.Error("scale failed", zap.Error(err))
		return fmt.Errorf("scale %s/%s: %w", req.Namespace, req.DeploymentName, err)
	}

	log.Info("scale succeeded",
		zap.Int32("replicas", req.Desired),
	)
	return nil
}
