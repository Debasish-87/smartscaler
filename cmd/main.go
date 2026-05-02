package main

import (
	"fmt"

	"smartscaler/pkg/config"
	"smartscaler/pkg/controller"
	"smartscaler/pkg/logger"
	"smartscaler/pkg/telemetry"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	cfg := config.Load()

	logger.Init(cfg.LogLevel, cfg.LogFormat == "json")
	defer logger.Sync()

	log := logger.L()
	log.Info(fmt.Sprintf("SmartScaler Operator %s (%s)", version, commit))

	telemetry.Register()
	telemetry.ServeMetrics(fmt.Sprintf(":%d", cfg.MetricsPort))

	ctrl := controller.New(cfg)
	ctrl.Run()
}
