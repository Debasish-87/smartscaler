package controller

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"smartscaler/pkg/cost"
	"smartscaler/pkg/decision"
	"smartscaler/pkg/events"
	"smartscaler/pkg/logger"
	"smartscaler/pkg/metrics"
	"smartscaler/pkg/scaler"
	"smartscaler/pkg/telemetry"
	"smartscaler/pkg/utils"
)

const (
	finalizer      = "autoscale.mycompany/finalizer"
	scalerGroup    = "autoscale.mycompany"
	scalerVersion  = "v1"
	scalerResource = "smartscalers"
)

var gvr = schema.GroupVersionResource{
	Group:    scalerGroup,
	Version:  scalerVersion,
	Resource: scalerResource,
}

func (c *Controller) Reconcile(key string) error {
	start := time.Now()
	log := logger.L().With(zap.String("key", key))

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	//  Fetch SmartScaler CR
	obj, err := c.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Debug("SmartScaler not found, skipping", zap.Error(err))
		return nil
	}

	//  Finalizer lifecycle
	if obj.GetDeletionTimestamp().IsZero() {
		if !utils.Contains(obj.GetFinalizers(), finalizer) {
			obj.SetFinalizers(utils.AddUnique(obj.GetFinalizers(), finalizer))
			_, err = c.dynamicClient.Resource(gvr).Namespace(ns).Update(ctx, obj, metav1.UpdateOptions{})
			return err
		}
	} else {
		obj.SetFinalizers(utils.Remove(obj.GetFinalizers(), finalizer))
		_, err = c.dynamicClient.Resource(gvr).Namespace(ns).Update(ctx, obj, metav1.UpdateOptions{})
		return err
	}

	//  Parse spec
	specRaw, ok := obj.Object["spec"]
	if !ok {
		log.Warn("SmartScaler missing spec")
		return nil
	}
	spec, ok := specRaw.(map[string]interface{})
	if !ok {
		log.Warn("SmartScaler spec is not a map")
		return nil
	}

	minVal, ok1 := spec["min"].(int64)
	maxVal, ok2 := spec["max"].(int64)
	target, ok3 := spec["targetDeployment"].(string)
	if !ok1 || !ok2 || !ok3 {
		log.Error("invalid spec fields", zap.Any("spec", spec))
		return nil
	}
	min, max := int32(minVal), int32(maxVal)

	//  Metrics: deployment CPU
	cpuResult, err := metrics.GetDeploymentCPU(c.clientset, ns, target)
	if err != nil {
		log.Warn("deployment CPU metrics unavailable", zap.Error(err))
		telemetry.ReconcileDuration.WithLabelValues(ns, "metrics_error").Observe(time.Since(start).Seconds())
		return nil
	}
	telemetry.DeploymentCPUMilliCores.WithLabelValues(ns, target).Set(float64(cpuResult.SmoothedAvgMilliCPU))

	//  Metrics: node CPU
	nodeResult, err := metrics.GetNodeCPU()
	if err != nil {
		log.Warn("node CPU metrics unavailable", zap.Error(err))
		nodeResult = metrics.NodeCPUResult{}
	}
	telemetry.NodeCPUMilliCores.Set(float64(nodeResult.SmoothedAvgMilliCPU))

	//  Current replica count
	deploy, err := c.clientset.AppsV1().Deployments(ns).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get target deployment %s/%s: %w", ns, target, err)
	}
	current := int32(1)
	if deploy.Spec.Replicas != nil {
		current = *deploy.Spec.Replicas
	}
	telemetry.CurrentReplicas.WithLabelValues(ns, target).Set(float64(current))

	//  Node count & cost
	nodeList, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	nodeCount := int64(len(nodeList.Items))

	estimator := cost.NewEstimator(c.cfg)
	totalCost := estimator.TotalCost(nodeCount, current)
	efficiency := estimator.Efficiency(cpuResult.SmoothedAvgMilliCPU, current)

	telemetry.EstimatedCostUSD.WithLabelValues(ns, target).Set(totalCost)
	telemetry.CostEfficiencyScore.WithLabelValues(ns, target).Set(efficiency)

	//  Decision
	engine := decision.NewEngine(c.cfg)
	d := engine.Decide(decision.Input{
		AvgCPUMilli:  cpuResult.SmoothedAvgMilliCPU,
		NodeCPUMilli: nodeResult.SmoothedAvgMilliCPU,
		Cost:         totalCost,
		Current:      current,
		Min:          min,
		Max:          max,
	})

	telemetry.LastDecision.Reset()
	telemetry.LastDecision.WithLabelValues(ns, target, d.Action, d.Reason).Set(1)

	log.Info("reconcile decision",
		zap.Int64("cpu_milli", cpuResult.SmoothedAvgMilliCPU),
		zap.Int64("node_cpu_milli", nodeResult.SmoothedAvgMilliCPU),
		zap.Float64("cost_usd", totalCost),
		zap.Float64("efficiency", efficiency),
		zap.Int32("current", current),
		zap.Int32("desired", d.Replicas),
		zap.String("action", d.Action),
		zap.String("reason", d.Reason),
	)

	//  No-op
	if d.Replicas == current {
		c.updateStatus(ctx, ns, name, d, current, cpuResult.SmoothedAvgMilliCPU, nodeResult.SmoothedAvgMilliCPU, totalCost)
		telemetry.ReconcileDuration.WithLabelValues(ns, "no_op").Observe(time.Since(start).Seconds())
		return nil
	}

	//  Cooldown
	resourceKey := ns + "/" + name
	c.cooldownMu.RLock()
	lastScale, hasCooldown := c.lastScaleMap[resourceKey]
	c.cooldownMu.RUnlock()

	if hasCooldown && time.Since(lastScale) < c.cfg.CooldownDuration {
		remaining := c.cfg.CooldownDuration - time.Since(lastScale)
		log.Info("cooldown active", zap.Duration("remaining", remaining))
		telemetry.CooldownActive.WithLabelValues(ns, name).Set(1)
		telemetry.ReconcileDuration.WithLabelValues(ns, "cooldown").Observe(time.Since(start).Seconds())
		return nil
	}
	telemetry.CooldownActive.WithLabelValues(ns, name).Set(0)

	//  Scale
	err = scaler.Scale(scaler.ScaleRequest{
		Clientset:      c.clientset,
		Namespace:      ns,
		DeploymentName: target,
		Desired:        d.Replicas,
		Context:        ctx,
	})
	if err != nil {
		c.emitter.EmitError(ns, name, err.Error())
		telemetry.ReconcileDuration.WithLabelValues(ns, "error").Observe(time.Since(start).Seconds())
		return err
	}

	c.cooldownMu.Lock()
	c.lastScaleMap[resourceKey] = time.Now()
	c.cooldownMu.Unlock()

	eventReason := events.EventScaleUp
	if d.Replicas < current {
		eventReason = events.EventScaleDown
	}
	c.emitter.Emit(ns, name, eventReason, d.Action, current, d.Replicas, fmt.Sprintf("cpu=%dm cost=%.4f", cpuResult.SmoothedAvgMilliCPU, totalCost))

	telemetry.ScaleTotal.WithLabelValues(ns, target, d.Action).Inc()
	c.updateStatus(ctx, ns, name, d, d.Replicas, cpuResult.SmoothedAvgMilliCPU, nodeResult.SmoothedAvgMilliCPU, totalCost)
	telemetry.ReconcileDuration.WithLabelValues(ns, "scaled").Observe(time.Since(start).Seconds())

	return nil
}

func (c *Controller) updateStatus(
	ctx context.Context,
	ns, name string,
	d decision.Decision,
	replicas int32,
	cpuMilli, nodeCPUMilli int64,
	cost float64,
) {
	obj, err := c.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.L().Warn("status update: get failed", zap.Error(err))
		return
	}

	obj.Object["status"] = map[string]interface{}{
		"currentReplicas": int64(replicas),
		"lastAction":      d.Action,
		"reason":          d.Reason,
		"avgCPU":          cpuMilli,
		"nodeCPU":         nodeCPUMilli,
		"cost":            cost,
		"lastScaleTime":   time.Now().UTC().Format(time.RFC3339),
	}

	_, err = c.dynamicClient.Resource(gvr).Namespace(ns).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		logger.L().Warn("status update failed", zap.String("key", ns+"/"+name), zap.Error(err))
	}
}
