# Metrics

The MCP Lifecycle Operator exposes **Prometheus** metrics from the **controller manager** (the operator Deployment that reconciles `MCPServer` resources). Metrics include those registered by [controller-runtime](https://book.kubebuilder.io/reference/metrics.html) (for example workqueues and the Kubernetes API client) and **custom `mcpserver_*` series** documented below.

This page is aimed at **platform and cluster operators** who scrape Prometheus and tune alerting—not at authors of `MCPServer` manifests alone.

## Metrics endpoint

Metrics are exposed over **HTTPS** at path **`/metrics`** on **port `8443`** on the controller manager metrics **Service** (the Service port is named **`https`**).

After a typical install from the [release `install.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/releases/latest), scrape:

`https://mcp-lifecycle-operator-controller-manager-metrics-service.mcp-lifecycle-operator-system.svc:8443/metrics`

Adjust the Service name and namespace if you change the Kustomize `namePrefix` / `namespace` when deploying.

!!! note "Controller flags (advanced)"
    `--metrics-bind-address` is a **string** flag; the literal value **`0`** turns the metrics server **off** ([`cmd/main.go`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/cmd/main.go)). Anything else (for example **`:8443`**) enables scraping on that address—the default Kustomize overlay sets **`--metrics-bind-address=:8443`** ([`config/default/manager_metrics_patch.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/default/manager_metrics_patch.yaml)). **`--metrics-secure`** defaults to **true**. Adjust these only if you maintain a custom Deployment manifest.

## Custom metrics

Custom metrics use the Prometheus namespace **`mcpserver`** (exported names start with **`mcpserver_`**).

| Metric | Type | Description |
| --- | --- | --- |
| `mcpserver_condition_info` | gauge | Current **Accepted** / **Ready** condition snapshot per `MCPServer`. Value is always `1`; filter by labels. |
| `mcpserver_validation_failures_total` | counter | Total permanent configuration validation failures (`ValidationError`). |
| `mcpserver_deployment_failures_total` | counter | Total failures when reconciling the workload Deployment (`reason` is currently `ReconcileError`). |
| `mcpserver_service_failures_total` | counter | Total failures when reconciling the Service (`reason` is currently `ReconcileError`). |
| `mcpserver_reconcile_phase_duration_seconds` | histogram | Duration of reconciliation phases **validation**, **deployment**, and **service** (seconds; default Prometheus histogram buckets). |

### Labels for `mcpserver_condition_info`

| Label | Description |
| --- | --- |
| `name` | `MCPServer` name |
| `namespace` | `MCPServer` namespace |
| `type` | Condition type: `Accepted` or `Ready` |
| `status` | `True`, `False`, or `Unknown` |
| `reason` | Condition reason (intended to mirror `.status.conditions[]`; see [note below](#gauge-versus-api-status)) |

Only one active series exists per `(name, namespace, type)`. On delete, gauge series for that object are removed; **`*_failures_total` counters are not**—time series may remain in Prometheus.

**Typical reasons:** **Accepted:** `Valid`, `Invalid`. **Ready:** `Available`, `ConfigurationInvalid`, `DeploymentUnavailable`, `ServiceUnavailable`, `ScaledToZero`, `Initializing`, `MCPEndpointUnavailable`. **Ready** may use status **Unknown** (for example `Initializing`).

<span id="gauge-versus-api-status"></span>

!!! note "Gauge versus API status"
    In rare cases the gauge can **diverge** from `MCPServer.status.conditions`: (1) **Permanent validation error** — `Ready` / `ConfigurationInvalid` is recorded only after a successful status write. (2) **MCP handshake** — after `Available`, a failed handshake may set status to `MCPEndpointUnavailable` without a second gauge update in the same reconcile. Prefer **`MCPServer.status` as source of truth** for correctness.

**Example queries**

```promql
sum by (namespace, type, status, reason) (mcpserver_condition_info)
```

### Labels for counters and histogram

**`mcpserver_validation_failures_total`:** `name`, `namespace`, `reason` (permanent errors currently use `Invalid`).

**`mcpserver_deployment_failures_total` / `mcpserver_service_failures_total`:** `name`, `namespace`, `reason`.

**`mcpserver_reconcile_phase_duration_seconds`:** `phase` ∈ `validation`, `deployment`, `service`. Use `_bucket`, `_sum`, and `_count` for quantiles.

## Prometheus Operator

If you use the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator), apply a `ServiceMonitor` that selects the controller-manager metrics Service. Example:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: controller-manager-metrics-monitor
  namespace: mcp-lifecycle-operator-system   # namespace where the operator runs
  labels:
    control-plane: controller-manager
    app.kubernetes.io/name: mcp-lifecycle-operator
spec:
  endpoints:
    - path: /metrics
      port: https
      scheme: https
      bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      tlsConfig:
        insecureSkipVerify: true   # tighten for production (e.g. cert-manager); see repo sample
  selector:
    matchLabels:
      control-plane: controller-manager
      app.kubernetes.io/name: mcp-lifecycle-operator
```

The repository maintains the full sample at [`config/prometheus/monitor.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/prometheus/monitor.yaml). Wire it into your install by uncommenting the **`[PROMETHEUS]`** resource (`../prometheus`) in [`config/default/kustomization.yaml`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/config/default/kustomization.yaml`), or apply an equivalent manifest alongside kube-prometheus-stack. Add labels your Prometheus `ServiceMonitor` selector expects (for example `release: prometheus`).

## Implementation note

Metric registration lives in [`internal/controller/metrics.go`](https://github.com/kubernetes-sigs/mcp-lifecycle-operator/blob/main/internal/controller/metrics.go). Update this page when that file changes.

## Next steps

- **[Introduction](introduction.md)** — Architecture and `MCPServer` overview (including status conditions)
- **[Quickstart](guides/quickstart.md)** — Deploy an MCP server and inspect status
- **[Contributing](contributing/index.md)** — How to contribute
