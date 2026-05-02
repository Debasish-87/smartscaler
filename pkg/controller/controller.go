package controller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"smartscaler/pkg/client"
	"smartscaler/pkg/config"
	"smartscaler/pkg/events"
	"smartscaler/pkg/logger"
	"smartscaler/pkg/telemetry"
)

type Controller struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface

	queue   workqueue.RateLimitingInterface
	emitter *events.Emitter
	cfg     *config.Config

	cooldownMu   sync.RWMutex
	lastScaleMap map[string]time.Time
}

func New(cfg *config.Config) *Controller {
	cs := client.GetClientset()
	return &Controller{
		clientset:     cs,
		dynamicClient: client.GetDynamicClient(),
		queue: workqueue.NewRateLimitingQueueWithConfig(
			workqueue.DefaultControllerRateLimiter(),
			workqueue.RateLimitingQueueConfig{Name: "smartscaler"},
		),
		emitter:      events.NewEmitter(cs),
		cfg:          cfg,
		lastScaleMap: make(map[string]time.Time),
	}
}

func (c *Controller) Run() {
	log := logger.L()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//  Signal handling 
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
		cancel()
	}()

	//  Informers
	scalerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		c.dynamicClient,
		c.cfg.ReconcileInterval,
		metav1.NamespaceAll,
		nil,
	)
	scalerInformer := scalerFactory.ForResource(gvr).Informer()
	scalerInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handle,
		UpdateFunc: func(_, newObj interface{}) { c.handle(newObj) },
		DeleteFunc: c.handle,
	})

	deployFactory := informers.NewSharedInformerFactory(c.clientset, c.cfg.ReconcileInterval)
	deployInformer := deployFactory.Apps().V1().Deployments().Informer()
	deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleDeployment,
		UpdateFunc: func(_, newObj interface{}) { c.handleDeployment(newObj) },
		DeleteFunc: c.handleDeployment,
	})

	scalerFactory.Start(ctx.Done())
	deployFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), scalerInformer.HasSynced, deployInformer.HasSynced) {
		log.Error("cache sync timed out — aborting")
		return
	}
	log.Info("caches synced")

	//  Workers 
	for i := 0; i < c.cfg.WorkerCount; i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}
	log.Info("workers started", zap.Int("count", c.cfg.WorkerCount))

	go wait.JitterUntil(func() {
		list, err := c.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Error("periodic reconcile list error", zap.Error(err))
			return
		}
		for _, item := range list.Items {
			key := item.GetNamespace() + "/" + item.GetName()
			c.queue.Add(key)
		}
		log.Debug("periodic reconcile triggered", zap.Int("scalers", len(list.Items)))
	}, c.cfg.ReconcileInterval, 0.2, true, ctx.Done())

	log.Info("SmartScaler operator started",
		zap.String("metricsAddr", fmt.Sprintf(":%d", c.cfg.MetricsPort)),
		zap.Duration("reconcileInterval", c.cfg.ReconcileInterval),
		zap.Duration("cooldown", c.cfg.CooldownDuration),
	)

	telemetry.Register()

	<-ctx.Done()

	log.Info("draining work queue")
	c.queue.ShutDown()
	log.Info("operator stopped")
}
