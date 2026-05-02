package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"smartscaler/pkg/logger"
)

func (c *Controller) handle(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.L().Error("handle: failed to get key", zap.Error(err))
		return
	}
	c.queue.Add(key)
}

func (c *Controller) handleDeployment(obj interface{}) {
	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		if tombstone, ok2 := obj.(cache.DeletedFinalStateUnknown); ok2 {
			deploy, ok = tombstone.Obj.(*appsv1.Deployment)
		}
		if !ok {
			logger.L().Error("handleDeployment: unexpected object type", zap.Any("obj", obj))
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	list, err := c.dynamicClient.Resource(gvr).Namespace(deploy.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.L().Error("handleDeployment: list SmartScalers", zap.Error(err))
		return
	}

	for _, item := range list.Items {
		specRaw, ok := item.Object["spec"]
		if !ok {
			continue
		}
		spec, ok := specRaw.(map[string]interface{})
		if !ok {
			continue
		}
		targetRaw, ok := spec["targetDeployment"]
		if !ok {
			continue
		}
		target, ok := targetRaw.(string)
		if !ok {
			continue
		}
		if target != deploy.Name {
			continue
		}

		key, err := cache.MetaNamespaceKeyFunc(&item)
		if err != nil {
			logger.L().Error("handleDeployment: key error", zap.Error(err))
			continue
		}
		c.queue.Add(key)
	}
}
