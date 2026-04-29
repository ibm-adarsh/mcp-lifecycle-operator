# Prometheus metrics

The controller manager exposes Prometheus metrics at `/metrics` on the metrics server when [`--metrics-bind-address`](https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server#Options) is non-zero. The generated **`config/default`** overlay patches the Deployment with `--metrics-bind-address=:8443` ([`manager_metrics_patch.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/default/manager_metrics_patch.yaml)); combined with `--metrics-secure` defaulting to **true** in [`cmd/main.go`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/cmd/main.go), that serves TLS on 8443. Use `--metrics-secure=false` for plain HTTP (for example with `:8080`). Besides [controller-runtime default metrics](https://book.kubebuilder.io/reference/metrics.html) (workqueues, REST client, leader election, etc.), the MCPServer reconciler registers **custom metrics** under the `mcpserver_` namespace.

## Scraping with Prometheus Operator

A sample `ServiceMonitor` for the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) lives in the repo:

- Manifest: [`config/prometheus/monitor.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/prometheus/monitor.yaml)

It is **not** wired into the default install: enable it by uncommenting the `[PROMETHEUS]` entry (`../prometheus`) in [`config/default/kustomization.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/default/kustomization.yaml), then apply your overlay (for example kube-prometheus-stack) so the operator discovers the monitor.

## Custom metrics reference

All custom metric names are prefixed with `mcpserver_` (Prometheus namespace `mcpserver`).

### `mcpserver_condition_info`

| Property | Value |
|----------|--------|
| **Type** | Gauge |
| **Help** | Current condition state of MCPServer resources. Value is always `1`; use labels to filter. |

**Labels**

| Label | Description |
|-------|-------------|
| `name` | MCPServer resource name |
| `namespace` | MCPServer namespace |
| `type` | Condition type: `Accepted` or `Ready` |
| `status` | Kubernetes condition status: `True`, `False`, or `Unknown` |
| `reason` | Condition reason — intended to match `.status.conditions[]`; see [Gauge vs status](#gauge-vs-mcpserver-status) |

For each `(name, namespace, type)` tuple, at most one time series is active: updating a condition deletes prior series with the same name, namespace, and type so only the current status/reason remains.

When an `MCPServer` is deleted, `cleanupMetrics` removes **only** `mcpserver_condition_info` series for that object. Counter metrics (`*_failures_total`) are not removed; their label sets may still appear in Prometheus after the resource is gone.

**Typical reasons** (from reconciler logic; see [`internal/controller/mcpserver_controller.go`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/internal/controller/mcpserver_controller.go))

- **Accepted:** `Valid`, `Invalid`
- **Ready:** `Available`, `ConfigurationInvalid`, `DeploymentUnavailable`, `ServiceUnavailable`, `ScaledToZero`, `Initializing`, `MCPEndpointUnavailable`

For **Ready**, `status` may be `Unknown` (for example reason `Initializing` while the Deployment has not reported conditions yet).

#### Gauge vs MCPServer status

`recordCondition` runs at fixed points in [`Reconcile`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/internal/controller/mcpserver_controller.go). In rare cases the gauge can **diverge** from the API object’s `.status.conditions`:

1. **Permanent validation error** — `Ready` / `ConfigurationInvalid` is passed to `recordCondition` only **after** `applyStatus` succeeds. If that update fails, the `validation_failures_total` counter and `Accepted` gauge may already have been updated, but the **Ready** gauge may still reflect an older reconcile.
2. **MCP endpoint handshake** — When deployment-level readiness is `Available`, the reconciler runs an MCP handshake and may set status to `MCPEndpointUnavailable` if it fails. That happens **after** `recordCondition` ran with `Available`; **`recordCondition` is not invoked again** in that reconcile, so the Ready gauge can still show `Available` until a later reconcile updates it.

Treat **`MCPServer.status.conditions` as authoritative** for correctness; use this gauge for aggregation and alerting with the above limitations in mind.

Use this metric to count or alert on MCPServers by acceptance/readiness state, for example:

```promql
sum by (namespace, type, status, reason) (mcpserver_condition_info)
```

### `mcpserver_validation_failures_total`

| Property | Value |
|----------|--------|
| **Type** | Counter |
| **Help** | Total number of configuration validation failures. |

**Labels**

| Label | Description |
|-------|-------------|
| `name` | MCPServer resource name |
| `namespace` | MCPServer namespace |
| `reason` | Validation failure reason — today permanent errors use `Invalid` (`ReasonInvalid`) |

Incremented once per reconcile that finishes with a permanent configuration validation error (`ValidationError`). Transient API errors during validation do not increment this counter; the controller retries without applying an `Accepted=False` status update for those failures.

### `mcpserver_deployment_failures_total`

| Property | Value |
|----------|--------|
| **Type** | Counter |
| **Help** | Total number of deployment reconciliation failures. |

**Labels**

| Label | Description |
|-------|-------------|
| `name` | MCPServer resource name |
| `namespace` | MCPServer namespace |
| `reason` | Failure reason (currently `ReconcileError` when `reconcileDeployment` returns an error) |

### `mcpserver_service_failures_total`

| Property | Value |
|----------|--------|
| **Type** | Counter |
| **Help** | Total number of service reconciliation failures. |

**Labels**

| Label | Description |
|-------|-------------|
| `name` | MCPServer resource name |
| `namespace` | MCPServer namespace |
| `reason` | Failure reason (currently `ReconcileError` when `reconcileService` returns an error) |

### `mcpserver_reconcile_phase_duration_seconds`

| Property | Value |
|----------|--------|
| **Type** | Histogram |
| **Help** | Duration of reconciliation phases in seconds. |
| **Buckets** | Prometheus default histogram buckets (`DefBuckets`) |

**Labels**

| Label | Description |
|-------|-------------|
| `phase` | One of `validation`, `deployment`, or `service` |

Observes wall-clock duration for:

- **`validation`** — `validateConfig` (success, permanent validation failure, or transient error — all paths observe once per reconcile attempt that reaches this phase)
- **`deployment`** — `reconcileDeployment` (always observed after the call returns)
- **`service`** — `reconcileService` (always observed after the call returns)

Use `_bucket`, `_sum`, and `_count` suffixes as usual for histogram quantiles and averages.

## Implementation note

Metric definitions and registration live in [`internal/controller/metrics.go`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/internal/controller/metrics.go). This document should stay aligned with that file when metrics are added or changed.
