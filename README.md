# SmartScaler

A Kubernetes operator that automatically adjusts Deployment replica counts based on
per-pod CPU utilisation, cluster-level node pressure, and a configurable cost model.
Implemented as a custom controller with a `SmartScaler` CRD, leader election,
Prometheus instrumentation, and a structured reconcile loop.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quickstart](#quickstart)
- [Live Output](#live-output)
- [Architecture](#architecture)
- [Reconcile Loop](#reconcile-loop)
- [Decision Engine](#decision-engine)
- [Metrics Collection](#metrics-collection)
- [Cost Model](#cost-model)
- [Scale Executor](#scale-executor)
- [CRD Reference](#crd-reference)
- [Configuration](#configuration)
- [Prometheus Metrics](#prometheus-metrics)
- [Observability](#observability)
- [Manual Deployment](#manual-deployment)
- [Development](#development)
- [Testing](#testing)
- [Uninstall](#uninstall)
- [Directory Structure](#directory-structure)

---

## Prerequisites

| Tool | Minimum Version |
|---|---|
| Go | 1.22 |
| Docker | 24+ |
| kubectl | 1.28+ |
| minikube | 1.32+ |
| metrics-server | enabled in cluster |

---

## Quickstart

```bash
make setup
# or directly
./scripts/setup.sh
```

After setup completes:

```bash
kubectl get smartscalers -o wide -w
kubectl get deployment demo-app -w
make observe
```

---

## Live Output

### Setup Script

The setup script builds the Docker image, applies CRD and RBAC manifests,
deploys the operator, and creates the demo workload and SmartScaler resource.

```
$ ./scripts/setup.sh

[MINIKUBE] Ensuring cluster is running

[BUILD] Building image: smartscaler:dev
  => [builder 5/7] RUN go mod download                                    53.1s
  => [builder 7/7] RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build   105.1s
  => [stage-1 3/3] COPY --from=builder /out/smartscaler /smartscaler       0.1s
✓ Image built: smartscaler:dev

[CRD] Applying CRD
  customresourcedefinition.apiextensions.k8s.io/smartscalers.autoscale.mycompany unchanged
✓ CRD established

[RBAC] Applying RBAC
  serviceaccount/smartscaler-sa unchanged
  clusterrole.rbac.authorization.k8s.io/smartscaler-role unchanged
  clusterrolebinding.rbac.authorization.k8s.io/smartscaler-binding unchanged
✓ RBAC applied

[OPERATOR] Deploying SmartScaler operator
  deployment.apps/smartscaler-operator unchanged
  deployment "smartscaler-operator" successfully rolled out
✓ Operator is running

[WORKLOAD] Deploying demo application
  deployment "demo-app" successfully rolled out
✓ demo-app is running

[SCALER] Applying SmartScaler custom resource
✓ SmartScaler resource created

[STATUS] Current cluster state

NAME                                    READY   STATUS    RESTARTS     AGE
smartscaler-operator-6fb64b8c58-7g9rj   1/1     Running   1 (8h ago)   9h
smartscaler-operator-6fb64b8c58-p2ctv   1/1     Running   1 (8h ago)   9h

NAME          TARGET     REPLICAS   CPU(M)   NODECPU(M)   COST($/H)   ACTION     REASON         AGE
demo-scaler   demo-app   3          105      3672         0.116       scale_up   high_avg_cpu   9h
```

The operator detected 105m average pod CPU above the 120m scale-up threshold and
issued an initial scale-up from the minimum of 2 replicas.

---

### Scaling Event — 2 to 10 Replicas

The operator scaled incrementally over several reconcile cycles (one replica per
cycle at 15s intervals) until the deployment reached the configured maximum.

```
$ kubectl get all

# Mid-scale — 5 replicas
NAME                            READY   STATUS    RESTARTS        AGE
pod/demo-app-54cb66f88d-7g2sh   1/1     Running   1 (4m53s ago)   9h
pod/demo-app-54cb66f88d-fxc2l   1/1     Running   0               110s
pod/demo-app-54cb66f88d-mp5ns   1/1     Running   1 (4m53s ago)   9h
pod/demo-app-54cb66f88d-x9fzj   1/1     Running   0               44s
pod/demo-app-54cb66f88d-xmlxq   1/1     Running   0               44s

NAME                       READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/demo-app   5/5     5            5           9h

# Max replicas reached — 10
NAME                            READY   STATUS    RESTARTS        AGE
pod/demo-app-54cb66f88d-76rvx   1/1     Running   0               2m10s
pod/demo-app-54cb66f88d-7g2sh   1/1     Running   1 (8m55s ago)   9h
pod/demo-app-54cb66f88d-7hwwm   1/1     Running   0               3m12s
pod/demo-app-54cb66f88d-8twr7   1/1     Running   0               2m11s
pod/demo-app-54cb66f88d-b2fkr   1/1     Running   0               70s
pod/demo-app-54cb66f88d-fxc2l   1/1     Running   0               5m52s
pod/demo-app-54cb66f88d-mp5ns   1/1     Running   1 (8m55s ago)   9h
pod/demo-app-54cb66f88d-nfg7g   1/1     Running   0               3m12s
pod/demo-app-54cb66f88d-x9fzj   1/1     Running   0               4m46s
pod/demo-app-54cb66f88d-xmlxq   1/1     Running   0               4m46s

NAME                       READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/demo-app   10/10   10           10          9h

NAME                                  DESIRED   CURRENT   READY   AGE
replicaset.apps/demo-app-54cb66f88d   10        10        10      9h
replicaset.apps/demo-app-7666bd7fc4   0         0         0       9h
```

---

### Live Dashboard — make observe

The observe script refreshes every 3 seconds with full cluster state.

```
Every 3.0s                                               Sat May  2 12:08:40 2026

╔═══════════════════════════════════════════════════════╗
║           SMARTSCALER LIVE DASHBOARD                  ║
╚═══════════════════════════════════════════════════════╝

── SmartScalers ─────────────────────────────────────────────────────────────────
NAMESPACE   NAME          TARGET     REPLICAS   CPU(M)   NODECPU(M)   COST($/H)   ACTION      REASON
default     demo-scaler   demo-app   10         117      0            0.196       no_change   within_threshold

── Pods ─────────────────────────────────────────────────────────────────────────
NAME                        READY   STATUS    RESTARTS      AGE
demo-app-54cb66f88d-76rvx   1/1     Running   0             5m51s
demo-app-54cb66f88d-7g2sh   1/1     Running   1 (12m ago)   9h
demo-app-54cb66f88d-7hwwm   1/1     Running   0             6m53s
demo-app-54cb66f88d-8twr7   1/1     Running   0             5m52s
demo-app-54cb66f88d-b2fkr   1/1     Running   0             4m51s
demo-app-54cb66f88d-fxc2l   1/1     Running   0             9m33s
demo-app-54cb66f88d-mp5ns   1/1     Running   1 (12m ago)   9h
demo-app-54cb66f88d-nfg7g   1/1     Running   0             6m53s
demo-app-54cb66f88d-x9fzj   1/1     Running   0             8m27s
demo-app-54cb66f88d-xmlxq   1/1     Running   0             8m27s

── CPU (pods) ───────────────────────────────────────────────────────────────────
NAME                        CPU(cores)   MEMORY(bytes)
demo-app-54cb66f88d-76rvx   122m         2Mi
demo-app-54cb66f88d-7g2sh   119m         2Mi
demo-app-54cb66f88d-7hwwm   114m         2Mi
demo-app-54cb66f88d-8twr7   119m         2Mi
demo-app-54cb66f88d-b2fkr   111m         2Mi
demo-app-54cb66f88d-fxc2l   113m         2Mi
demo-app-54cb66f88d-mp5ns   124m         2Mi
demo-app-54cb66f88d-nfg7g   120m         2Mi
demo-app-54cb66f88d-x9fzj   120m         2Mi
demo-app-54cb66f88d-xmlxq   113m         2Mi

── CPU (nodes) ──────────────────────────────────────────────────────────────────
NAME       CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
minikube   1541m        38%      1127Mi          14%
```

At this point the operator shows `no_change / within_threshold` because the smoothed
average CPU (117m) is below the scale-up threshold (120m) and above the scale-down
threshold (60m). The CPU never drops low enough to trigger scale-down because the
demo workload's `low` intensity loop consumes ~115m per pod continuously.

---

## Architecture

SmartScaler runs as a Kubernetes Deployment with two replicas and leader election.
Only the elected leader actively reconciles. The operator is structured as a
unidirectional data pipeline that executes on every reconcile tick.

```
                        Kubernetes API Server
                                  |
                                  |  Watch (Informer)
                                  v
  ┌─────────────────────────────────────────────────────────────┐
  │                       Controller                            │
  │                                                             │
  │   Informer ──► Work Queue ──► Worker Pool (4 goroutines)    │
  │                                      |                      │
  │                               Reconcile(key)                │
  └───────────────────────────────────── | ─────────────────────┘
                                         |
           ┌─────────────────────────────┼───────────────────────┐
           |                             |                       |
           v                             v                       v
  ┌─────────────────┐    ┌───────────────────────┐   ┌──────────────────┐
  │  Metrics Layer  │    │    Decision Engine    │   │  Scale Executor  │
  │                 │    │                       │   │                  │
  │ GetDeploymentCPU│    │  Decide(Input)        │   │  Scale(req)      │
  │ GetNodeCPU      │    │  1. Safety bounds     │   │  - Get deploy    │
  │                 │    │  2. Scale up (pod CPU)│   │  - Set replicas  │
  │ 5-sample moving │    │  3. Scale up (node)   │   │  - Retry x5      │
  │ average         │    │  4. Scale down (cost) │   │  - Exp backoff   │
  │                 │    │  5. Scale down (CPU)  │   │                  │
  │ Staleness check │    │  6. No-op             │   │                  │
  │ 30s threshold   │    │                       │   │                  │
  └────────┬────────┘    └──────────┬────────────┘   └────────┬─────────┘
           |                        |                         |
           └────────────────────────┴─────────────────────────┘
                                    |
           ┌─────────────────────── ┼ ───────────────────────────┐
           |                        |                            |
           v                        v                            v
  ┌─────────────────┐  ┌────────────────────────┐  ┌─────────────────────┐
  │ Cost Estimator  │  │    Status Writer       │  │     Telemetry       │
  │                 │  │                        │  │                     │
  │ NodeCost        │  │  UpdateStatus() on CR  │  │  Prometheus         │
  │ PodCost         │  │  lastAction, reason    │  │  Counters / Gauges  │
  │ TotalCost       │  │  avgCPU, nodeCPU       │  │  Histograms         │
  │ Efficiency      │  │  cost, lastScaleTime   │  │                     │
  └─────────────────┘  └────────────────────────┘  └─────────────────────┘
```

### Component Responsibilities

| Package | Responsibility |
|---|---|
| `pkg/controller` | Informer, work queue, worker goroutines, reconcile dispatch, cooldown map, leader election |
| `pkg/decision` | Stateless pure function. Takes CPU/node/cost/replicas, returns action and target. |
| `pkg/scaler` | Kubernetes API call to update Deployment replicas with conflict-safe retry |
| `pkg/metrics` | Fetch pod and node CPU from metrics-server. 5-sample moving average. Staleness filter. |
| `pkg/cost` | Hourly cost from node count and pod count. Efficiency score and wasted cost. |
| `pkg/telemetry` | Prometheus metric registration. HTTP server at `/metrics`, `/healthz`, `/readyz`. |
| `pkg/events` | Kubernetes Events on scale up, scale down, and errors |
| `pkg/config` | Load all configuration from environment variables with typed defaults |
| `pkg/client` | Singleton Kubernetes client. In-cluster config first, falls back to kubeconfig. |

---

## Reconcile Loop

Every cycle begins when a `namespace/name` key is dequeued from the work queue.
The loop runs within a 15-second context timeout.

```
Step 1  Fetch SmartScaler CR
        Retrieve object via dynamic client.
        If not found (deleted), return without error.
        Register or remove finalizer based on deletion timestamp.

Step 2  Collect Pod CPU Metrics
        Call metrics.GetDeploymentCPU.
        Fetch metrics from metrics-server for Running pods of target Deployment.
        Discard samples older than 30 seconds (staleness filter).
        Apply 5-sample moving average per deployment key (thread-safe).

Step 3  Collect Node CPU Metrics
        Call metrics.GetNodeCPU.
        Aggregate across all nodes with fresh metrics.
        If unavailable, continue with zero value rather than failing the cycle.

Step 4  Estimate Cost
        TotalCost = (nodeCount x costPerNode) + (replicas x costPerPod)
        Compute efficiency as CPU-per-dollar ratio.
        Write both to Prometheus gauges.

Step 5  Run Decision Engine
        Pass inputs to engine.Decide().
        Returns desired replica count, action string, reason string.
        If desired == current: skip to status update (no-op path).

Step 6  Check Cooldown
        Look up last scale time for this resource key in in-memory map.
        If within cooldown window: skip, set cooldown_active gauge to 1, return.
        Cooldown is per-resource, protected by a read-write mutex.

Step 7  Execute Scale
        Call scaler.Scale().
        Update Deployment replica count with conflict-safe retry.
        5 attempts, 200ms base, 2x exponential backoff, 10% jitter.
        On success: update cooldown map, emit Kubernetes Event.

Step 8  Write Status
        Patch SmartScaler CR status subresource with current replicas,
        last action, reason, smoothed CPU, cost, and RFC3339 timestamp.
```

### Code Path

```go
// pkg/controller/reconcile.go

func (c *Controller) Reconcile(key string) error {
    ns, name, _ := cache.SplitMetaNamespaceKey(key)

    // Step 1: fetch CR, handle finalizer
    obj, err := c.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, ...)

    // Step 2-3: collect metrics
    cpuResult, _ := metrics.GetDeploymentCPU(c.clientset, ns, target)
    nodeResult, _ := metrics.GetNodeCPU()

    // Step 4: cost
    totalCost := estimator.TotalCost(nodeCount, current)

    // Step 5: decision
    d := engine.Decide(decision.Input{
        AvgCPUMilli:  cpuResult.SmoothedAvgMilliCPU,
        NodeCPUMilli: nodeResult.SmoothedAvgMilliCPU,
        Cost:         totalCost,
        Current:      current,
        Min:          min,
        Max:          max,
    })

    // no-op early return
    if d.Replicas == current {
        c.updateStatus(...)
        return nil
    }

    // Step 6: cooldown check
    if hasCooldown && time.Since(lastScale) < c.cfg.CooldownDuration {
        return nil
    }

    // Step 7: scale + Step 8: status
    scaler.Scale(scaler.ScaleRequest{...})
    c.lastScaleMap[resourceKey] = time.Now()
    c.emitter.Emit(...)
    c.updateStatus(...)
}
```

---

## Decision Engine

The decision engine is a pure function with no side effects or external dependencies.
It receives a snapshot of current state and returns a deterministic `Decision`.
This design makes it independently unit-testable without a cluster.

### Input and Output

```go
// pkg/decision/engine.go

type Input struct {
    AvgCPUMilli  int64   // smoothed per-pod average CPU (milliCPU)
    NodeCPUMilli int64   // smoothed cluster-average node CPU (milliCPU)
    MemoryBytes  int64   // reserved — not yet used in decisions
    Cost         float64 // estimated $/hr for this workload
    Current      int32   // current replica count
    Min          int32   // minimum replicas from SmartScaler spec
    Max          int32   // maximum replicas from SmartScaler spec
}

type Decision struct {
    Replicas int32  // desired replica count
    Action   string // machine-readable action label
    Reason   string // machine-readable reason label
}
```

### Priority Order

Rules are evaluated top-to-bottom. First match returns immediately.

| Priority | Condition | Action | Reason |
|---|---|---|---|
| 1 — Safety | `current < min` | `correct_min_violation` | `current_below_min` |
| 1 — Safety | `current > max` | `correct_max_violation` | `current_above_max` |
| 2 — Pod CPU | `avgCPU > scaleUpThreshold AND current < max` | `scale_up` | `high_avg_cpu` |
| 3 — Node | `nodeCPU > nodePressureThreshold AND current < max` | `node_pressure_scale` | `node_cpu_high` |
| 4 — Cost+CPU | `avgCPU < scaleDownThreshold AND cost > highCostThreshold AND current > min` | `cost_optimized_scale_down` | `low_cpu_high_cost` |
| 5 — CPU Low | `avgCPU < scaleDownThreshold AND current > min` | `scale_down` | `low_avg_cpu` |
| 6 — No-op | none of the above | `no_change` | `within_threshold` |

### Multi-Step Scale-Up

Step size increases with CPU severity to recover faster from sudden load spikes.

```go
// pkg/decision/engine.go

if in.AvgCPUMilli > e.cfg.ScaleUpCPUThreshold && in.Current < in.Max {
    step := int32(1)

    if in.AvgCPUMilli > e.cfg.ScaleUpCPUThreshold*2 {
        step = e.cfg.MaxScaleStep  // severely overloaded — +3 replicas (default)
    } else if in.AvgCPUMilli > int64(float64(e.cfg.ScaleUpCPUThreshold)*1.5) {
        step = 2                   // moderately overloaded — +2 replicas
    }
    // 1x – 1.5x threshold: +1 replica

    target := clamp(in.Current+step, in.Min, in.Max)
    return Decision{Replicas: target, Action: "scale_up", Reason: "high_avg_cpu"}
}
```

> Per-resource overrides: if `scaleUpThresholdMilliCPU` or `scaleDownThresholdMilliCPU`
> are set on the SmartScaler spec, the reconciler injects them into the engine config
> before calling `Decide`. Global environment variable thresholds are defaults only.

---

## Metrics Collection

Both pod and node metrics are fetched from the Kubernetes metrics-server via the
`k8s.io/metrics` client. Raw values pass through a per-key sliding window before
reaching the decision engine.

### Deployment CPU Pipeline

```go
// pkg/metrics/deployment.go

// 1. Resolve label selector from Deployment spec
selector := metav1.FormatLabelSelector(&metav1.LabelSelector{
    MatchLabels: deploy.Spec.Selector.MatchLabels,
})

// 2. List only Running pods matching the selector
podList, _ := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
    LabelSelector: selector,
    FieldSelector: "status.phase=Running",
})

// 3. Fetch pod metrics — discard stale samples (> 30s old)
for _, m := range metricsList.Items {
    if time.Since(m.Timestamp.Time) > metricsStaleness {
        continue
    }
    podCPU[m.Name] = sumContainerCPU(m.Containers)
}

// 4. Raw average over valid pods
rawAvg := totalCPU / int64(validPods)

// 5. 5-sample moving average (thread-safe, per deployment key)
history = append(history, rawAvg)
if len(history) > historyWindow { // historyWindow = 5
    history = history[1:]
}
smoothed := sum(history) / int64(len(history))
```

If all pod metrics for a deployment are stale, the reconcile cycle is skipped with
a warning log. The operator never acts on stale data.

### Node CPU Pipeline

```go
// pkg/metrics/node.go

for _, n := range nodes.Items {
    if time.Since(n.Timestamp.Time) > metricsStaleness {
        continue
    }
    totalCPU += n.Usage.Cpu().MilliValue()
    validNodes++
}
rawAvg := totalCPU / int64(validNodes)
// same 5-sample moving average applied
```

Node CPU uses a separate global history slice with its own mutex.

---

## Cost Model

```go
// pkg/cost/cost.go

// Total hourly cost
func (e *Estimator) TotalCost(nodeCount int64, podCount int32) float64 {
    return float64(nodeCount)*e.cfg.CostPerNodePerHour +
           float64(podCount)*e.cfg.CostPerPodPerHour
}

// Efficiency — CPU per dollar, higher is better utilisation
func (e *Estimator) Efficiency(avgCPUMilli int64, podCount int32) float64 {
    cpuPerPod := float64(avgCPUMilli) / float64(podCount)
    return cpuPerPod / e.cfg.CostPerPodPerHour
}

// WastedCost — fraction of spend on idle capacity
func (e *Estimator) WastedCost(avgCPUMilli, cpuLimitMilli int64,
                               nodeCount int64, podCount int32) float64 {
    utilization := float64(avgCPUMilli) / float64(cpuLimitMilli)
    return e.TotalCost(nodeCount, podCount) * (1 - utilization)
}
```

Default rates: node = $0.096/hr (AWS t3.medium on-demand), pod = $0.01/hr.
Override via `COST_PER_NODE_PER_HOUR_USD` and `COST_PER_POD_PER_HOUR_USD`.

---

## Scale Executor

```go
// pkg/scaler/scale.go

err := retry.RetryOnConflict(wait.Backoff{
    Steps:    5,
    Duration: 200 * time.Millisecond,
    Factor:   2.0,  // exponential backoff
    Jitter:   0.1,  // 10% jitter
}, func() error {
    // Re-fetch latest version before every attempt
    latest, _ := req.Clientset.AppsV1().Deployments(req.Namespace).
        Get(ctx, req.DeploymentName, metav1.GetOptions{})

    if current == req.Desired {
        return nil // already at desired, no-op
    }

    latest.Spec.Replicas = &req.Desired
    _, err = req.Clientset.AppsV1().Deployments(req.Namespace).
        Update(ctx, latest, metav1.UpdateOptions{})
    return err // RetryOnConflict retries on HTTP 409
})
```

Re-fetching before each attempt ensures the write uses the latest resource version,
avoiding 409 Conflict errors from concurrent modifications.

---

## CRD Reference

API group: `autoscale.mycompany/v1` | Kind: `SmartScaler` | Short name: `ss`

### Spec Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `targetDeployment` | string | Yes | — | Name of the Deployment to manage (same namespace). |
| `min` | integer | Yes | — | Minimum replica count (>= 1). Hard floor — never goes below this. |
| `max` | integer | Yes | — | Maximum replica count (>= min). Enforced by CEL admission rule. |
| `cooldownSeconds` | integer | No | 60 | Seconds between consecutive scale events. 0 uses global `COOLDOWN_SECONDS`. |
| `scaleUpThresholdMilliCPU` | integer | No | global | Per-pod CPU (mCPU) above which scale-up fires. Overrides global. |
| `scaleDownThresholdMilliCPU` | integer | No | global | Per-pod CPU (mCPU) below which scale-down is considered. Overrides global. |

CRD admission validation enforces `self.min <= self.max` via CEL. Invalid objects
are rejected by the API server before reaching the operator.

### Status Fields

| Field | Type | Description |
|---|---|---|
| `currentReplicas` | integer | Replica count as last written by the operator. |
| `lastAction` | string | `scale_up`, `scale_down`, `no_change`, `cost_optimized_scale_down`, `node_pressure_scale`, `correct_min_violation`, `correct_max_violation` |
| `reason` | string | `high_avg_cpu`, `low_avg_cpu`, `within_threshold`, `node_cpu_high`, `low_cpu_high_cost`, `current_below_min`, `current_above_max` |
| `avgCPU` | integer | Smoothed per-pod average CPU at last reconcile (milliCPU). 5-sample moving average. |
| `nodeCPU` | integer | Smoothed cluster-average node CPU at last reconcile (milliCPU). |
| `cost` | number | Estimated hourly cost (USD) at last reconcile replica count. |
| `lastScaleTime` | string (RFC3339) | Timestamp of last successful scale. Used to enforce cooldown. |

### Printer Columns

```
$ kubectl get smartscalers -o wide

NAME          TARGET     REPLICAS   CPU(M)   NODECPU(M)   COST($/H)   ACTION      REASON             AGE
demo-scaler   demo-app   10         117      0            0.196       no_change   within_threshold   9h
```

### Sample Resource

```yaml
apiVersion: autoscale.mycompany/v1
kind: SmartScaler
metadata:
  name: demo-scaler
  namespace: default
spec:
  targetDeployment: demo-app
  min: 2
  max: 10
  cooldownSeconds: 30                   # overrides global 60s cooldown
  scaleUpThresholdMilliCPU:   120       # scale up above 120m per pod
  scaleDownThresholdMilliCPU:  60       # scale down below 60m per pod
```

---

## Configuration

All configuration is loaded from environment variables at startup. Set these in
`deploy/operator/operator.yaml` under the operator container's `env` block.

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `info` | Verbosity: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` for log ingestion, `text` for local development |
| `RECONCILE_INTERVAL_SECONDS` | `15` | Reconcile loop frequency per resource (seconds) |
| `COOLDOWN_SECONDS` | `60` | Global minimum seconds between consecutive scale events |
| `SCALE_UP_CPU_THRESHOLD_MILLICPU` | `80` | Global scale-up trigger (mCPU per pod) |
| `SCALE_DOWN_CPU_THRESHOLD_MILLICPU` | `30` | Global scale-down trigger (mCPU per pod) |
| `NODE_PRESSURE_THRESHOLD_MILLICPU` | `800` | Node CPU (mCPU) above which node-pressure scale-up fires |
| `HIGH_COST_THRESHOLD_USD` | `0.20` | $/hr above which cost-optimised scale-down is preferred |
| `COST_PER_NODE_PER_HOUR_USD` | `0.096` | Hourly node cost (e.g. AWS t3.medium on-demand) |
| `COST_PER_POD_PER_HOUR_USD` | `0.01` | Hourly per-pod cost overhead |
| `METRICS_PORT` | `8080` | Port for `/metrics`, `/healthz`, `/readyz` |
| `LEADER_ELECTION_ENABLED` | `true` | Enable leader election for HA. Disable only for single-replica dev. |
| `LEADER_ELECTION_NAMESPACE` | `kube-system` | Namespace where the leader election Lease is created |
| `LEADER_ELECTION_ID` | `smartscaler-leader` | Name of the leader election Lease object |
| `WORKER_COUNT` | `4` | Concurrent reconcile goroutines |
| `MAX_SCALE_STEP` | `3` | Max replica increase per scale-up event (at 2x threshold breach) |

---

## Prometheus Metrics

All metrics are in the `smartscaler_` namespace and served at `:8080/metrics`.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `smartscaler_scale_operations_total` | Counter | namespace, deployment, action | Total successful scale operations. |
| `smartscaler_scale_errors_total` | Counter | namespace, deployment | Total failed scale operations after all retries. |
| `smartscaler_current_replicas` | Gauge | namespace, deployment | Current replica count. Updated every reconcile. |
| `smartscaler_deployment_cpu_millicores` | Gauge | namespace, deployment | Smoothed (5-sample) per-pod CPU in milliCPU. Exact value passed to decision engine. |
| `smartscaler_node_cpu_millicores` | Gauge | — | Smoothed cluster-average node CPU in milliCPU. |
| `smartscaler_estimated_cost_usd_per_hour` | Gauge | namespace, deployment | Estimated hourly cost in USD. |
| `smartscaler_cost_efficiency_score` | Gauge | namespace, deployment | CPU milliCPU per dollar per hour. Higher = better utilisation. |
| `smartscaler_reconcile_duration_seconds` | Histogram | namespace, result | Reconcile duration. `result`: `no_op`, `scaled`, `cooldown`, `error`, `metrics_error`. Buckets: 10ms–5s. |
| `smartscaler_cooldown_active` | Gauge | namespace, scaler | 1 if in cooldown, 0 otherwise. |
| `smartscaler_last_decision_info` | Gauge | namespace, deployment, action, reason | Always 1. Labels carry last decision context for Grafana. |

---

## Observability

```bash
# Prometheus metrics
make port-forward
# open http://localhost:8080/metrics

# Live terminal dashboard
make observe

# Trigger scale-up (high CPU loop — 800k iterations, 50ms sleep)
kubectl set env deployment/demo-app LOAD_INTENSITY=high

# Trigger scale-down (low CPU loop — 80k iterations, 500ms sleep)
kubectl set env deployment/demo-app LOAD_INTENSITY=low

# Stress test
make stress CPU=500 DURATION=60
```

Grafana dashboard: `deploy/monitoring/grafana-dashboard.json`
ServiceMonitor (Prometheus Operator): `deploy/monitoring/servicemonitor.yaml`
Alerting rules: `deploy/monitoring/alerts.yaml`

---

## Manual Deployment

```bash
minikube start --memory=4096 --cpus=4 --driver=docker
minikube addons enable metrics-server

make minikube-load

kubectl apply -f deploy/crd/crd.yaml
kubectl wait --for=condition=Established \
  crd/smartscalers.autoscale.mycompany --timeout=30s

kubectl apply -f deploy/rbac/rbac.yaml

kubectl apply -f deploy/operator/operator.yaml
kubectl rollout status -n kube-system \
  deployment/smartscaler-operator --timeout=120s

kubectl apply -f deploy/workloads/deployment.yaml
kubectl apply -f deploy/samples/scaler.yaml
```

---

## Development

```bash
make build      # compile to ./bin/smartscaler
make fmt        # gofmt -s -w .
make vet        # go vet ./...
make lint       # golangci-lint run ./...
make tidy       # go mod tidy
make restart    # rollout restart operator pod
```

---

## Testing

```bash
make test
# equivalent to:
go test -race -count=1 -timeout=60s ./...
```

Coverage includes unit tests for `scaler`, `decision`, `cost`, and `utils`, plus
integration and regression tests under `tests/`.

---

## Uninstall

```bash
make uninstall   # removes CR, workload, operator, RBAC, CRD
minikube stop    # optional — stops the cluster
```

---

## Directory Structure

```
.
├── cmd/
│   └── main.go                    # Entrypoint
├── pkg/
│   ├── config/                    # Environment-based configuration
│   ├── controller/                # Reconcile loop, worker pool, event handlers
│   ├── decision/                  # Scaling decision engine (pure function)
│   ├── scaler/                    # Kubernetes API scaling with conflict retry
│   ├── metrics/                   # Pod and node CPU collection and smoothing
│   ├── cost/                      # Cost estimation and efficiency scoring
│   ├── telemetry/                 # Prometheus metric registration
│   ├── events/                    # Kubernetes event emission
│   ├── logger/                    # Structured logger (zap)
│   ├── client/                    # Singleton Kubernetes client
│   └── utils/                     # Shared helpers
├── deploy/
│   ├── crd/                       # CustomResourceDefinition
│   ├── rbac/                      # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── operator/                  # Deployment, PodDisruptionBudget, Service
│   ├── workloads/                 # Demo application Deployment
│   ├── samples/                   # Example SmartScaler resource
│   └── monitoring/                # Grafana dashboard, ServiceMonitor, alerts
├── tests/                         # Unit, integration, and regression tests
├── scripts/
│   ├── setup.sh                   # Full cluster bootstrap
│   ├── observe.sh                 # Live terminal dashboard
│   └── stress.sh                  # CPU stress test
├── Dockerfile
├── Makefile
└── .golangci.yml
```