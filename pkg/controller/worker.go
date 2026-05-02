package controller

import (
	"go.uber.org/zap"
	"smartscaler/pkg/logger"
)

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(item)

	key, ok := item.(string)
	if !ok {
		logger.L().Error("invalid queue item type", zap.Any("item", item))
		c.queue.Forget(item)
		return true
	}

	var reconcileErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Error("reconcile panic recovered",
					zap.String("key", key),
					zap.Any("panic", r),
				)
				c.queue.AddRateLimited(item)
			}
		}()
		reconcileErr = c.Reconcile(key)
	}()

	if reconcileErr != nil {
		logger.L().Error("reconcile error", zap.String("key", key), zap.Error(reconcileErr))
		c.queue.AddRateLimited(item)
	} else {
		c.queue.Forget(item)
	}

	return true
}
