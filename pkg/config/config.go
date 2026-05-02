package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Logging
	LogLevel  string 
	LogFormat string 

	// Reconcile intervals
	ReconcileInterval time.Duration 
	CooldownDuration  time.Duration 

	// Decision engine thresholds (milliCPU)
	ScaleUpCPUThreshold   int64 
	ScaleDownCPUThreshold int64 
	NodePressureThreshold int64 

	// Cost thresholds ($/hr)
	HighCostThreshold float64 

	// Cloud cost per unit
	CostPerNodePerHour float64 
	CostPerPodPerHour  float64 

	// Prometheus
	MetricsPort int 

	// Leader election
	LeaderElection          bool   
	LeaderElectionNamespace string 
	LeaderElectionID        string 

	// Workers
	WorkerCount int 

	// Max scale step
	MaxScaleStep int32 
}

func Load() *Config {
	return &Config{
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		LogFormat:               getEnv("LOG_FORMAT", "json"),
		ReconcileInterval:       getDuration("RECONCILE_INTERVAL_SECONDS", 15),
		CooldownDuration:        getDuration("COOLDOWN_SECONDS", 60),
		ScaleUpCPUThreshold:     getInt64("SCALE_UP_CPU_THRESHOLD_MILLICPU", 80),
		ScaleDownCPUThreshold:   getInt64("SCALE_DOWN_CPU_THRESHOLD_MILLICPU", 30),
		NodePressureThreshold:   getInt64("NODE_PRESSURE_THRESHOLD_MILLICPU", 800),
		HighCostThreshold:       getFloat64("HIGH_COST_THRESHOLD_USD", 0.20),
		CostPerNodePerHour:      getFloat64("COST_PER_NODE_PER_HOUR_USD", 0.096), // e.g. t3.medium on AWS
		CostPerPodPerHour:       getFloat64("COST_PER_POD_PER_HOUR_USD", 0.01),
		MetricsPort:             getInt("METRICS_PORT", 8080),
		LeaderElection:          getBool("LEADER_ELECTION_ENABLED", true),
		LeaderElectionNamespace: getEnv("LEADER_ELECTION_NAMESPACE", "kube-system"),
		LeaderElectionID:        getEnv("LEADER_ELECTION_ID", "smartscaler-leader"),
		WorkerCount:             getInt("WORKER_COUNT", 4),
		MaxScaleStep:            int32(getInt("MAX_SCALE_STEP", 3)),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, defaultSeconds int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(defaultSeconds) * time.Second
}

func getInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getFloat64(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}
