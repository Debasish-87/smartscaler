package telemetry

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once sync.Once

	ScaleTotal *prometheus.CounterVec

	ScaleErrors *prometheus.CounterVec

	CurrentReplicas *prometheus.GaugeVec

	DeploymentCPUMilliCores *prometheus.GaugeVec

	NodeCPUMilliCores prometheus.Gauge

	EstimatedCostUSD *prometheus.GaugeVec

	CostEfficiencyScore *prometheus.GaugeVec

	ReconcileDuration *prometheus.HistogramVec

	CooldownActive *prometheus.GaugeVec

	LastDecision *prometheus.GaugeVec
)

func Register() {
	once.Do(func() {
		ScaleTotal = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "smartscaler",
			Name:      "scale_operations_total",
			Help:      "Total number of scale operations performed.",
		}, []string{"namespace", "deployment", "action"})

		ScaleErrors = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "smartscaler",
			Name:      "scale_errors_total",
			Help:      "Total number of failed scale operations.",
		}, []string{"namespace", "deployment"})

		CurrentReplicas = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "current_replicas",
			Help:      "Current replica count managed by SmartScaler.",
		}, []string{"namespace", "deployment"})

		DeploymentCPUMilliCores = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "deployment_cpu_millicores",
			Help:      "Smoothed average CPU usage per pod (milliCPU) for the target deployment.",
		}, []string{"namespace", "deployment"})

		NodeCPUMilliCores = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "node_cpu_millicores",
			Help:      "Smoothed average node CPU usage (milliCPU) across the cluster.",
		})

		EstimatedCostUSD = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "estimated_cost_usd_per_hour",
			Help:      "Estimated hourly cost (USD) for the workload.",
		}, []string{"namespace", "deployment"})

		CostEfficiencyScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "cost_efficiency_score",
			Help:      "CPU milliCPU per dollar ratio — higher is better.",
		}, []string{"namespace", "deployment"})

		ReconcileDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "smartscaler",
			Name:      "reconcile_duration_seconds",
			Help:      "Duration of each reconcile loop in seconds.",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		}, []string{"namespace", "result"})

		CooldownActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "cooldown_active",
			Help:      "1 if the scaler is in cooldown, 0 otherwise.",
		}, []string{"namespace", "scaler"})

		LastDecision = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "smartscaler",
			Name:      "last_decision_info",
			Help:      "Last decision info. Value is always 1; use labels for context.",
		}, []string{"namespace", "deployment", "action", "reason"})
	})
}

func ServeMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	go func() {
		server := &http.Server{Addr: addr, Handler: mux}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("metrics server failed: " + err.Error())
		}
	}()
}
