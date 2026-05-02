package decision

import (
	"smartscaler/pkg/config"
)

type Input struct {
	AvgCPUMilli  int64   
	NodeCPUMilli int64   
	MemoryBytes  int64   
	Cost         float64 
	Current      int32   
	Min          int32   
	Max          int32   
}

type Decision struct {
	Replicas int32
	Action   string
	Reason   string
}

type Engine struct {
	cfg *config.Config
}

func NewEngine(cfg *config.Config) *Engine {
	return &Engine{cfg: cfg}
}

func (e *Engine) Decide(in Input) Decision {

	//  SAFETY BOUNDS 

	if in.Current < in.Min {
		return Decision{Replicas: in.Min, Action: "correct_min_violation", Reason: "current_below_min"}
	}
	if in.Current > in.Max {
		return Decision{Replicas: in.Max, Action: "correct_max_violation", Reason: "current_above_max"}
	}

	//  SCALE UP: pod-level CPU overload 

	if in.AvgCPUMilli > e.cfg.ScaleUpCPUThreshold && in.Current < in.Max {
		step := int32(1)

		// Aggressive multi-step scale when severely overloaded
		if in.AvgCPUMilli > e.cfg.ScaleUpCPUThreshold*2 {
			step = e.cfg.MaxScaleStep
		} else if in.AvgCPUMilli > int64(float64(e.cfg.ScaleUpCPUThreshold)*1.5) {
			step = 2
		}

		target := clamp(in.Current+step, in.Min, in.Max)
		return Decision{Replicas: target, Action: "scale_up", Reason: "high_avg_cpu"}
	}

	//  SCALE UP: node-level pressure 

	if in.NodeCPUMilli > e.cfg.NodePressureThreshold && in.Current < in.Max {
		target := clamp(in.Current+1, in.Min, in.Max)
		return Decision{Replicas: target, Action: "node_pressure_scale", Reason: "node_cpu_high"}
	}

	//  SCALE DOWN: CPU low AND cost high → prioritise cost savings 

	if in.AvgCPUMilli < e.cfg.ScaleDownCPUThreshold &&
		in.Cost > e.cfg.HighCostThreshold &&
		in.Current > in.Min {

		target := clamp(in.Current-1, in.Min, in.Max)
		return Decision{Replicas: target, Action: "cost_optimized_scale_down", Reason: "low_cpu_high_cost"}
	}

	//  SCALE DOWN: normal under-utilisation 

	if in.AvgCPUMilli < e.cfg.ScaleDownCPUThreshold && in.Current > in.Min {
		target := clamp(in.Current-1, in.Min, in.Max)
		return Decision{Replicas: target, Action: "scale_down", Reason: "low_avg_cpu"}
	}

	//  NO-OP 

	return Decision{Replicas: in.Current, Action: "no_change", Reason: "within_threshold"}
}

func clamp(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
