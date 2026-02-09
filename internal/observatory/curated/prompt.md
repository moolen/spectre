# Well-Known Metrics Research for Spectre Observatory

You are helping me build a comprehensive database of well-known Prometheus metrics and their signal role classifications. This will be used to improve automatic classification in Spectre's observability core.

## Objective

Research and compile an exhaustive list of metrics from various exporters, runtimes, applications, and frameworks. For each metric, classify it into a signal role and assess its importance for incident response.

## Signal Role Taxonomy

Classify each metric into exactly one of these roles:

| Role | Description | Examples |
|------|-------------|----------|
| **availability** | Is the thing up/reachable? | `up`, `kube_pod_status_phase`, `pg_up` |
| **latency** | How long do operations take? | `http_request_duration_seconds`, `etcd_request_duration_seconds` |
| **errors** | What's failing? | `http_requests_total{status=~"5.."}`, `grpc_server_handled_total{code!="OK"}` |
| **traffic** | How much load/throughput? | `http_requests_total`, `kafka_messages_in_total` |
| **saturation** | How full are resources? | `container_memory_usage_bytes`, `node_filesystem_avail_bytes` |
| **churn** | How much instability? | `kube_pod_container_status_restarts_total`, `process_start_time_seconds` |
| **novelty** | What's new or unusual? | `kube_pod_created`, `process_num_fds` (when used for leak detection) |

When a metric could serve multiple roles depending on context, pick the **primary/most common use case** and note alternatives in the `notes` field.

## Output Format

Produce one JSON file per research batch with this structure:

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T12:00:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/tree/main/docs"
  ],
  "metrics": [
    {
      "name": "kube_pod_status_phase",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.95,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "pod", "phase"],
      "common_promql_patterns": [
        "sum by (namespace) (kube_pod_status_phase{phase=\"Failed\"})",
        "kube_pod_status_phase{phase=~\"Pending|Unknown\"} > 0"
      ],
      "notes": "Primary metric for pod lifecycle state. phase label values: Pending, Running, Succeeded, Failed, Unknown",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Exact metric name |
| `name_pattern` | string\|null | no | Regex pattern if metric has variants (e.g., `http_request_duration.*`) |
| `signal_role` | string | yes | One of: availability, latency, errors, traffic, saturation, churn, novelty |
| `confidence` | float | yes | 0.0–1.0. How confident is this classification? 0.95+ for clearly documented, 0.7+ for inferred from name/usage |
| `importance` | float | yes | 0.0–1.0. How critical is this metric for incident response? Would you look at this in the first 5 minutes of an outage? |
| `source` | string | yes | Format: `org/exporter` or `ecosystem/component` (e.g., `prometheus/node_exporter`, `kubernetes/kubelet`) |
| `metric_type` | string | yes | One of: counter, gauge, histogram, summary, info, stateset, unknown |
| `labels_of_interest` | array | no | Labels useful for filtering, grouping, or K8s resource correlation |
| `common_promql_patterns` | array | no | Idiomatic PromQL queries using this metric |
| `notes` | string | no | Additional context, caveats, or alternative interpretations |
| `deprecated` | bool | yes | Is this metric deprecated or superseded? |
| `disabled_by_default` | bool | yes | Does this require explicit opt-in to collect? |

## Research Sources

For each batch, consult (in order of priority):

1. **Official documentation** — metric references, instrumentation guides
2. **Source code** — metric definitions in exporters, `prometheus.NewCounterVec()` calls, etc.
3. **GitHub repos** — README, docs folders, CHANGELOG for deprecations
4. **Prometheus mixins** — https://monitoring.mixins.dev/ — curated alert rules and dashboards
5. **Grafana dashboard catalog** — https://grafana.com/grafana/dashboards/ — real-world usage patterns
6. **OpenMetrics/OpenTelemetry specs** — for standard conventions

Record all sources consulted in `sources_consulted`.

## Research Batches

Execute these batches sequentially. After completing each batch, present the JSON output for my review before proceeding to the next.

### Batch 1: Kubernetes Core
**Subagent focus**: Control plane and core cluster metrics

Research:
- kubelet metrics (`kubelet_*`)
- kube-apiserver metrics (`apiserver_*`)
- kube-scheduler metrics (`scheduler_*`)
- kube-controller-manager metrics (`workqueue_*`, etc.)
- etcd metrics (`etcd_*`)
- kube-state-metrics (`kube_*`)
- cadvisor container metrics (`container_*`)

Key sources:
- https://kubernetes.io/docs/reference/instrumentation/metrics/
- https://github.com/kubernetes/kube-state-metrics/tree/main/docs
- https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md

---

### Batch 2: Node & Infrastructure
**Subagent focus**: Host-level and infrastructure metrics

Research:
- node_exporter (`node_*`)
- process-exporter (`namedprocess_*`)
- systemd exporter (`systemd_*`)
- cAdvisor standalone (if different from kubelet-embedded)

Key sources:
- https://github.com/prometheus/node_exporter
- https://github.com/ncabatoff/process-exporter
- https://prometheus.io/docs/guides/node-exporter/

---

### Batch 3: Language Runtimes
**Subagent focus**: Application runtime metrics across languages

Research:
- **Go**: default prometheus client metrics (`go_*`, `process_*`, `promhttp_*`)
- **Java/JVM**: Micrometer (`jvm_*`, `process_*`), JMX exporter patterns
- **Node.js**: prom-client default metrics (`nodejs_*`, `process_*`)
- **Python**: prometheus_client defaults (`python_*`, `process_*`)
- **.NET**: prometheus-net (`dotnet_*`, `process_*`)

Key sources:
- https://github.com/prometheus/client_golang
- https://micrometer.io/docs/concepts
- https://github.com/siimon/prom-client
- https://github.com/prometheus/client_python

---

### Batch 4: CNCF Ecosystem
**Subagent focus**: Cloud-native tooling metrics

Research:
- Prometheus (`prometheus_*`)
- Grafana (`grafana_*`)
- Flux (`gotk_*`, `flux_*`)
- ArgoCD (`argocd_*`)
- cert-manager (`certmanager_*`)
- external-secrets (`externalsecret_*`)
- Cilium (`cilium_*`, `hubble_*`)
- CoreDNS (`coredns_*`)
- Envoy (`envoy_*`)
- Istio (`istio_*`, `pilot_*`, `galley_*`)
- Linkerd (`linkerd_*`, `request_total`)
- KEDA (`keda_*`)
- Crossplane (`crossplane_*`)

Key sources:
- Official docs for each project
- https://monitoring.mixins.dev/

---

### Batch 5: Databases
**Subagent focus**: Database exporter metrics

Research:
- PostgreSQL (`pg_*`) — postgres_exporter
- MySQL (`mysql_*`) — mysqld_exporter
- Redis (`redis_*`) — redis_exporter
- MongoDB (`mongodb_*`) — mongodb_exporter
- Elasticsearch (`elasticsearch_*`) — elasticsearch_exporter

Key sources:
- https://github.com/prometheus-community/postgres_exporter
- https://github.com/prometheus/mysqld_exporter
- https://github.com/oliver006/redis_exporter
- https://github.com/percona/mongodb_exporter
- https://github.com/prometheus-community/elasticsearch_exporter

---

### Batch 6: Message Queues & Storage
**Subagent focus**: Async infrastructure metrics

Research:
- Kafka (`kafka_*`) — kafka_exporter, JMX exporter patterns
- RabbitMQ (`rabbitmq_*`)
- NATS (`nats_*`, `gnatsd_*`)
- MinIO (`minio_*`)
- Rook/Ceph (`ceph_*`, `rook_*`)

Key sources:
- https://github.com/danielqsj/kafka_exporter
- https://www.rabbitmq.com/prometheus.html
- https://github.com/nats-io/prometheus-nats-exporter
- https://min.io/docs/minio/linux/operations/monitoring/metrics-and-alerts.html

---

### Batch 7: HTTP & Networking
**Subagent focus**: Ingress, proxy, and HTTP metrics

Research:
- nginx (`nginx_*`, `nginxexporter_*`)
- HAProxy (`haproxy_*`)
- Traefik (`traefik_*`)
- ingress-nginx (`nginx_ingress_controller_*`)
- Generic HTTP patterns (OpenTelemetry HTTP semantic conventions)
- Generic gRPC patterns

Key sources:
- https://github.com/nginxinc/nginx-prometheus-exporter
- https://www.haproxy.com/documentation/haproxy-configuration-tutorials/alerts-and-monitoring/prometheus/
- https://doc.traefik.io/traefik/observability/metrics/prometheus/
- https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/
- https://opentelemetry.io/docs/specs/semconv/http/http-metrics/

---

### Batch 8: Conventions & Patterns
**Subagent focus**: Cross-cutting naming conventions and methodologies

Research:
- OpenMetrics conventions (`_total`, `_bucket`, `_sum`, `_count`, `_info`, `_created`)
- OpenTelemetry semantic conventions (metrics)
- RED method standard patterns
- USE method standard patterns
- Google SRE golden signals patterns
- Common anti-patterns to flag (e.g., high-cardinality labels)

Key sources:
- https://openmetrics.io/
- https://opentelemetry.io/docs/specs/semconv/
- https://www.weave.works/blog/the-red-method-key-metrics-for-microservices-architecture/
- https://www.brendangregg.com/usemethod.html
- https://sre.google/sre-book/monitoring-distributed-systems/

This batch should produce a special output format — patterns rather than specific metrics:

```json
{
  "batch": "conventions-patterns",
  "patterns": [
    {
      "pattern": "_total$",
      "pattern_type": "suffix",
      "inferred_metric_type": "counter",
      "inferred_signal_role": null,
      "role_disambiguation": "Depends on metric name: *_errors_total → errors, *_requests_total → traffic",
      "confidence": 0.9,
      "source": "openmetrics",
      "notes": "OpenMetrics convention for counters"
    },
    {
      "pattern": "histogram_quantile\\(.*_bucket\\)",
      "pattern_type": "promql",
      "inferred_signal_role": "latency",
      "confidence": 0.85,
      "notes": "histogram_quantile on _bucket metrics almost always indicates latency"
    }
  ]
}
```

---

## Execution Instructions

1. **Spawn subagents** for parallel research. Maximum 4-6 concurrent subagents.

2. **Each subagent** should:
   - Focus on one batch or a subset of a batch
   - Consult the specified sources
   - Produce JSON in the exact format specified
   - Include `sources_consulted` for traceability
   - Flag any metrics where classification is ambiguous

3. **After each batch completes**, present the output for my review. I may:
   - Approve and continue to next batch
   - Request corrections or additions
   - Ask for deeper research on specific exporters

4. **Handle conflicts** as follows:
   - Same metric name, different exporters, different meanings → separate entries with `source` disambiguation
   - Same metric, genuinely ambiguous role → pick primary use case, note alternatives in `notes`
   - Deprecated metric superseded by new metric → include both, link via `notes`

5. **Quality checks** before presenting each batch:
   - All required fields populated
   - `confidence` and `importance` are calibrated (not everything is 0.9+)
   - `signal_role` is one of the seven valid values
   - `metric_type` is one of: counter, gauge, histogram, summary, info, stateset, unknown
   - JSON is valid and parseable

## Example Output (Partial)

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T14:30:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md",
    "https://monitoring.mixins.dev/kubernetes/"
  ],
  "metrics": [
    {
      "name": "up",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 1.0,
      "importance": 1.0,
      "source": "prometheus/scrape",
      "metric_type": "gauge",
      "labels_of_interest": ["job", "instance"],
      "common_promql_patterns": [
        "up == 0",
        "avg by (job) (up)"
      ],
      "notes": "Universal Prometheus scrape health metric. 1 = target up, 0 = target down. First thing to check in any outage.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "kube_deployment_status_replicas_unavailable",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.9,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "deployment"],
      "common_promql_patterns": [
        "kube_deployment_status_replicas_unavailable > 0",
        "sum by (namespace) (kube_deployment_status_replicas_unavailable)"
      ],
      "notes": "Number of unavailable replicas. Non-zero indicates deployment health issue.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "container_cpu_usage_seconds_total",
      "name_pattern": null,
      "signal_role": "saturation",
      "confidence": 0.9,
      "importance": 0.85,
      "source": "google/cadvisor",
      "metric_type": "counter",
      "labels_of_interest": ["namespace", "pod", "container", "cpu"],
      "common_promql_patterns": [
        "rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])",
        "sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])) / sum by (namespace, pod) (kube_pod_container_resource_limits{resource=\"cpu\"})"
      ],
      "notes": "CPU time consumed. Use rate() and compare against limits for saturation. Exclude empty container label to avoid cgroup aggregates.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_duration_seconds",
      "name_pattern": null,
      "signal_role": "latency",
      "confidence": 0.95,
      "importance": 0.85,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "histogram",
      "labels_of_interest": ["verb", "resource", "subresource", "scope"],
      "common_promql_patterns": [
        "histogram_quantile(0.99, sum by (verb, le) (rate(apiserver_request_duration_seconds_bucket[5m])))",
        "histogram_quantile(0.99, rate(apiserver_request_duration_seconds_bucket{verb!~\"WATCH|LIST\"}[5m]))"
      ],
      "notes": "API server request latency. Exclude WATCH/LIST for meaningful percentiles. High latency here affects entire cluster.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_total",
      "name_pattern": null,
      "signal_role": "traffic",
      "confidence": 0.9,
      "importance": 0.7,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "counter",
      "labels_of_interest": ["verb", "resource", "code", "component"],
      "common_promql_patterns": [
        "sum(rate(apiserver_request_total[5m])) by (verb)",
        "sum(rate(apiserver_request_total{code=~\"5..\"}[5m]))"
      ],
      "notes": "API server request count. Can also be used for errors when filtered by code=~\"5..\". Primary role is traffic; error detection is secondary.",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

---

## Begin

Start with **Batch 1: Kubernetes Core**. Spawn subagents as needed to parallelize research across kube-state-metrics, kubelet, apiserver, etc.

Present the completed Batch 1 JSON for my review before proceeding to Batch 2# Well-Known Metrics Research for Spectre Observatory

You are helping me build a comprehensive database of well-known Prometheus metrics and their signal role classifications. This will be used to improve automatic classification in Spectre's observability core.

## Objective

Research and compile an exhaustive list of metrics from various exporters, runtimes, applications, and frameworks. For each metric, classify it into a signal role and assess its importance for incident response.

## Signal Role Taxonomy

Classify each metric into exactly one of these roles:

| Role | Description | Examples |
|------|-------------|----------|
| **availability** | Is the thing up/reachable? | `up`, `kube_pod_status_phase`, `pg_up` |
| **latency** | How long do operations take? | `http_request_duration_seconds`, `etcd_request_duration_seconds` |
| **errors** | What's failing? | `http_requests_total{status=~"5.."}`, `grpc_server_handled_total{code!="OK"}` |
| **traffic** | How much load/throughput? | `http_requests_total`, `kafka_messages_in_total` |
| **saturation** | How full are resources? | `container_memory_usage_bytes`, `node_filesystem_avail_bytes` |
| **churn** | How much instability? | `kube_pod_container_status_restarts_total`, `process_start_time_seconds` |
| **novelty** | What's new or unusual? | `kube_pod_created`, `process_num_fds` (when used for leak detection) |

When a metric could serve multiple roles depending on context, pick the **primary/most common use case** and note alternatives in the `notes` field.

## Output Format

Produce one JSON file per research batch with this structure:

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T12:00:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/tree/main/docs"
  ],
  "metrics": [
    {
      "name": "kube_pod_status_phase",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.95,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "pod", "phase"],
      "common_promql_patterns": [
        "sum by (namespace) (kube_pod_status_phase{phase=\"Failed\"})",
        "kube_pod_status_phase{phase=~\"Pending|Unknown\"} > 0"
      ],
      "notes": "Primary metric for pod lifecycle state. phase label values: Pending, Running, Succeeded, Failed, Unknown",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Exact metric name |
| `name_pattern` | string\|null | no | Regex pattern if metric has variants (e.g., `http_request_duration.*`) |
| `signal_role` | string | yes | One of: availability, latency, errors, traffic, saturation, churn, novelty |
| `confidence` | float | yes | 0.0–1.0. How confident is this classification? 0.95+ for clearly documented, 0.7+ for inferred from name/usage |
| `importance` | float | yes | 0.0–1.0. How critical is this metric for incident response? Would you look at this in the first 5 minutes of an outage? |
| `source` | string | yes | Format: `org/exporter` or `ecosystem/component` (e.g., `prometheus/node_exporter`, `kubernetes/kubelet`) |
| `metric_type` | string | yes | One of: counter, gauge, histogram, summary, info, stateset, unknown |
| `labels_of_interest` | array | no | Labels useful for filtering, grouping, or K8s resource correlation |
| `common_promql_patterns` | array | no | Idiomatic PromQL queries using this metric |
| `notes` | string | no | Additional context, caveats, or alternative interpretations |
| `deprecated` | bool | yes | Is this metric deprecated or superseded? |
| `disabled_by_default` | bool | yes | Does this require explicit opt-in to collect? |

## Research Sources

For each batch, consult (in order of priority):

1. **Official documentation** — metric references, instrumentation guides
2. **Source code** — metric definitions in exporters, `prometheus.NewCounterVec()` calls, etc.
3. **GitHub repos** — README, docs folders, CHANGELOG for deprecations
4. **Prometheus mixins** — https://monitoring.mixins.dev/ — curated alert rules and dashboards
5. **Grafana dashboard catalog** — https://grafana.com/grafana/dashboards/ — real-world usage patterns
6. **OpenMetrics/OpenTelemetry specs** — for standard conventions

Record all sources consulted in `sources_consulted`.

## Research Batches

Execute these batches sequentially. After completing each batch, present the JSON output for my review before proceeding to the next.

### Batch 1: Kubernetes Core
**Subagent focus**: Control plane and core cluster metrics

Research:
- kubelet metrics (`kubelet_*`)
- kube-apiserver metrics (`apiserver_*`)
- kube-scheduler metrics (`scheduler_*`)
- kube-controller-manager metrics (`workqueue_*`, etc.)
- etcd metrics (`etcd_*`)
- kube-state-metrics (`kube_*`)
- cadvisor container metrics (`container_*`)

Key sources:
- https://kubernetes.io/docs/reference/instrumentation/metrics/
- https://github.com/kubernetes/kube-state-metrics/tree/main/docs
- https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md

---

### Batch 2: Node & Infrastructure
**Subagent focus**: Host-level and infrastructure metrics

Research:
- node_exporter (`node_*`)
- process-exporter (`namedprocess_*`)
- systemd exporter (`systemd_*`)
- cAdvisor standalone (if different from kubelet-embedded)

Key sources:
- https://github.com/prometheus/node_exporter
- https://github.com/ncabatoff/process-exporter
- https://prometheus.io/docs/guides/node-exporter/

---

### Batch 3: Language Runtimes
**Subagent focus**: Application runtime metrics across languages

Research:
- **Go**: default prometheus client metrics (`go_*`, `process_*`, `promhttp_*`)
- **Java/JVM**: Micrometer (`jvm_*`, `process_*`), JMX exporter patterns
- **Node.js**: prom-client default metrics (`nodejs_*`, `process_*`)
- **Python**: prometheus_client defaults (`python_*`, `process_*`)
- **.NET**: prometheus-net (`dotnet_*`, `process_*`)

Key sources:
- https://github.com/prometheus/client_golang
- https://micrometer.io/docs/concepts
- https://github.com/siimon/prom-client
- https://github.com/prometheus/client_python

---

### Batch 4: CNCF Ecosystem
**Subagent focus**: Cloud-native tooling metrics

Research:
- Prometheus (`prometheus_*`)
- Grafana (`grafana_*`)
- Flux (`gotk_*`, `flux_*`)
- ArgoCD (`argocd_*`)
- cert-manager (`certmanager_*`)
- external-secrets (`externalsecret_*`)
- Cilium (`cilium_*`, `hubble_*`)
- CoreDNS (`coredns_*`)
- Envoy (`envoy_*`)
- Istio (`istio_*`, `pilot_*`, `galley_*`)
- Linkerd (`linkerd_*`, `request_total`)
- KEDA (`keda_*`)
- Crossplane (`crossplane_*`)

Key sources:
- Official docs for each project
- https://monitoring.mixins.dev/

---

### Batch 5: Databases
**Subagent focus**: Database exporter metrics

Research:
- PostgreSQL (`pg_*`) — postgres_exporter
- MySQL (`mysql_*`) — mysqld_exporter
- Redis (`redis_*`) — redis_exporter
- MongoDB (`mongodb_*`) — mongodb_exporter
- Elasticsearch (`elasticsearch_*`) — elasticsearch_exporter

Key sources:
- https://github.com/prometheus-community/postgres_exporter
- https://github.com/prometheus/mysqld_exporter
- https://github.com/oliver006/redis_exporter
- https://github.com/percona/mongodb_exporter
- https://github.com/prometheus-community/elasticsearch_exporter

---

### Batch 6: Message Queues & Storage
**Subagent focus**: Async infrastructure metrics

Research:
- Kafka (`kafka_*`) — kafka_exporter, JMX exporter patterns
- RabbitMQ (`rabbitmq_*`)
- NATS (`nats_*`, `gnatsd_*`)
- MinIO (`minio_*`)
- Rook/Ceph (`ceph_*`, `rook_*`)

Key sources:
- https://github.com/danielqsj/kafka_exporter
- https://www.rabbitmq.com/prometheus.html
- https://github.com/nats-io/prometheus-nats-exporter
- https://min.io/docs/minio/linux/operations/monitoring/metrics-and-alerts.html

---

### Batch 7: HTTP & Networking
**Subagent focus**: Ingress, proxy, and HTTP metrics

Research:
- nginx (`nginx_*`, `nginxexporter_*`)
- HAProxy (`haproxy_*`)
- Traefik (`traefik_*`)
- ingress-nginx (`nginx_ingress_controller_*`)
- Generic HTTP patterns (OpenTelemetry HTTP semantic conventions)
- Generic gRPC patterns

Key sources:
- https://github.com/nginxinc/nginx-prometheus-exporter
- https://www.haproxy.com/documentation/haproxy-configuration-tutorials/alerts-and-monitoring/prometheus/
- https://doc.traefik.io/traefik/observability/metrics/prometheus/
- https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/
- https://opentelemetry.io/docs/specs/semconv/http/http-metrics/

---

### Batch 8: Conventions & Patterns
**Subagent focus**: Cross-cutting naming conventions and methodologies

Research:
- OpenMetrics conventions (`_total`, `_bucket`, `_sum`, `_count`, `_info`, `_created`)
- OpenTelemetry semantic conventions (metrics)
- RED method standard patterns
- USE method standard patterns
- Google SRE golden signals patterns
- Common anti-patterns to flag (e.g., high-cardinality labels)

Key sources:
- https://openmetrics.io/
- https://opentelemetry.io/docs/specs/semconv/
- https://www.weave.works/blog/the-red-method-key-metrics-for-microservices-architecture/
- https://www.brendangregg.com/usemethod.html
- https://sre.google/sre-book/monitoring-distributed-systems/

This batch should produce a special output format — patterns rather than specific metrics:

```json
{
  "batch": "conventions-patterns",
  "patterns": [
    {
      "pattern": "_total$",
      "pattern_type": "suffix",
      "inferred_metric_type": "counter",
      "inferred_signal_role": null,
      "role_disambiguation": "Depends on metric name: *_errors_total → errors, *_requests_total → traffic",
      "confidence": 0.9,
      "source": "openmetrics",
      "notes": "OpenMetrics convention for counters"
    },
    {
      "pattern": "histogram_quantile\\(.*_bucket\\)",
      "pattern_type": "promql",
      "inferred_signal_role": "latency",
      "confidence": 0.85,
      "notes": "histogram_quantile on _bucket metrics almost always indicates latency"
    }
  ]
}
```

---

## Execution Instructions

1. **Spawn subagents** for parallel research. Maximum 4-6 concurrent subagents.

2. **Each subagent** should:
   - Focus on one batch or a subset of a batch
   - Consult the specified sources
   - Produce JSON in the exact format specified
   - Include `sources_consulted` for traceability
   - Flag any metrics where classification is ambiguous

3. **After each batch completes**, present the output for my review. I may:
   - Approve and continue to next batch
   - Request corrections or additions
   - Ask for deeper research on specific exporters

4. **Handle conflicts** as follows:
   - Same metric name, different exporters, different meanings → separate entries with `source` disambiguation
   - Same metric, genuinely ambiguous role → pick primary use case, note alternatives in `notes`
   - Deprecated metric superseded by new metric → include both, link via `notes`

5. **Quality checks** before presenting each batch:
   - All required fields populated
   - `confidence` and `importance` are calibrated (not everything is 0.9+)
   - `signal_role` is one of the seven valid values
   - `metric_type` is one of: counter, gauge, histogram, summary, info, stateset, unknown
   - JSON is valid and parseable

## Example Output (Partial)

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T14:30:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md",
    "https://monitoring.mixins.dev/kubernetes/"
  ],
  "metrics": [
    {
      "name": "up",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 1.0,
      "importance": 1.0,
      "source": "prometheus/scrape",
      "metric_type": "gauge",
      "labels_of_interest": ["job", "instance"],
      "common_promql_patterns": [
        "up == 0",
        "avg by (job) (up)"
      ],
      "notes": "Universal Prometheus scrape health metric. 1 = target up, 0 = target down. First thing to check in any outage.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "kube_deployment_status_replicas_unavailable",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.9,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "deployment"],
      "common_promql_patterns": [
        "kube_deployment_status_replicas_unavailable > 0",
        "sum by (namespace) (kube_deployment_status_replicas_unavailable)"
      ],
      "notes": "Number of unavailable replicas. Non-zero indicates deployment health issue.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "container_cpu_usage_seconds_total",
      "name_pattern": null,
      "signal_role": "saturation",
      "confidence": 0.9,
      "importance": 0.85,
      "source": "google/cadvisor",
      "metric_type": "counter",
      "labels_of_interest": ["namespace", "pod", "container", "cpu"],
      "common_promql_patterns": [
        "rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])",
        "sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])) / sum by (namespace, pod) (kube_pod_container_resource_limits{resource=\"cpu\"})"
      ],
      "notes": "CPU time consumed. Use rate() and compare against limits for saturation. Exclude empty container label to avoid cgroup aggregates.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_duration_seconds",
      "name_pattern": null,
      "signal_role": "latency",
      "confidence": 0.95,
      "importance": 0.85,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "histogram",
      "labels_of_interest": ["verb", "resource", "subresource", "scope"],
      "common_promql_patterns": [
        "histogram_quantile(0.99, sum by (verb, le) (rate(apiserver_request_duration_seconds_bucket[5m])))",
        "histogram_quantile(0.99, rate(apiserver_request_duration_seconds_bucket{verb!~\"WATCH|LIST\"}[5m]))"
      ],
      "notes": "API server request latency. Exclude WATCH/LIST for meaningful percentiles. High latency here affects entire cluster.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_total",
      "name_pattern": null,
      "signal_role": "traffic",
      "confidence": 0.9,
      "importance": 0.7,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "counter",
      "labels_of_interest": ["verb", "resource", "code", "component"],
      "common_promql_patterns": [
        "sum(rate(apiserver_request_total[5m])) by (verb)",
        "sum(rate(apiserver_request_total{code=~\"5..\"}[5m]))"
      ],
      "notes": "API server request count. Can also be used for errors when filtered by code=~\"5..\". Primary role is traffic; error detection is secondary.",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

---

## Begin

Start with **Batch 1: Kubernetes Core**. Spawn subagents as needed to parallelize research across kube-state-metrics, kubelet, apiserver, etc.

Present the completed Batch 1 JSON for my review before proceeding to Batch 2.# Well-Known Metrics Research for Spectre Observatory

You are helping me build a comprehensive database of well-known Prometheus metrics and their signal role classifications. This will be used to improve automatic classification in Spectre's observability core.

## Objective

Research and compile an exhaustive list of metrics from various exporters, runtimes, applications, and frameworks. For each metric, classify it into a signal role and assess its importance for incident response.

## Signal Role Taxonomy

Classify each metric into exactly one of these roles:

| Role | Description | Examples |
|------|-------------|----------|
| **availability** | Is the thing up/reachable? | `up`, `kube_pod_status_phase`, `pg_up` |
| **latency** | How long do operations take? | `http_request_duration_seconds`, `etcd_request_duration_seconds` |
| **errors** | What's failing? | `http_requests_total{status=~"5.."}`, `grpc_server_handled_total{code!="OK"}` |
| **traffic** | How much load/throughput? | `http_requests_total`, `kafka_messages_in_total` |
| **saturation** | How full are resources? | `container_memory_usage_bytes`, `node_filesystem_avail_bytes` |
| **churn** | How much instability? | `kube_pod_container_status_restarts_total`, `process_start_time_seconds` |
| **novelty** | What's new or unusual? | `kube_pod_created`, `process_num_fds` (when used for leak detection) |

When a metric could serve multiple roles depending on context, pick the **primary/most common use case** and note alternatives in the `notes` field.

## Output Format

Produce one JSON file per research batch with this structure:

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T12:00:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/tree/main/docs"
  ],
  "metrics": [
    {
      "name": "kube_pod_status_phase",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.95,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "pod", "phase"],
      "common_promql_patterns": [
        "sum by (namespace) (kube_pod_status_phase{phase=\"Failed\"})",
        "kube_pod_status_phase{phase=~\"Pending|Unknown\"} > 0"
      ],
      "notes": "Primary metric for pod lifecycle state. phase label values: Pending, Running, Succeeded, Failed, Unknown",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Exact metric name |
| `name_pattern` | string\|null | no | Regex pattern if metric has variants (e.g., `http_request_duration.*`) |
| `signal_role` | string | yes | One of: availability, latency, errors, traffic, saturation, churn, novelty |
| `confidence` | float | yes | 0.0–1.0. How confident is this classification? 0.95+ for clearly documented, 0.7+ for inferred from name/usage |
| `importance` | float | yes | 0.0–1.0. How critical is this metric for incident response? Would you look at this in the first 5 minutes of an outage? |
| `source` | string | yes | Format: `org/exporter` or `ecosystem/component` (e.g., `prometheus/node_exporter`, `kubernetes/kubelet`) |
| `metric_type` | string | yes | One of: counter, gauge, histogram, summary, info, stateset, unknown |
| `labels_of_interest` | array | no | Labels useful for filtering, grouping, or K8s resource correlation |
| `common_promql_patterns` | array | no | Idiomatic PromQL queries using this metric |
| `notes` | string | no | Additional context, caveats, or alternative interpretations |
| `deprecated` | bool | yes | Is this metric deprecated or superseded? |
| `disabled_by_default` | bool | yes | Does this require explicit opt-in to collect? |

## Research Sources

For each batch, consult (in order of priority):

1. **Official documentation** — metric references, instrumentation guides
2. **Source code** — metric definitions in exporters, `prometheus.NewCounterVec()` calls, etc.
3. **GitHub repos** — README, docs folders, CHANGELOG for deprecations
4. **Prometheus mixins** — https://monitoring.mixins.dev/ — curated alert rules and dashboards
5. **Grafana dashboard catalog** — https://grafana.com/grafana/dashboards/ — real-world usage patterns
6. **OpenMetrics/OpenTelemetry specs** — for standard conventions

Record all sources consulted in `sources_consulted`.

## Research Batches

Execute these batches sequentially. After completing each batch, present the JSON output for my review before proceeding to the next.

### Batch 1: Kubernetes Core
**Subagent focus**: Control plane and core cluster metrics

Research:
- kubelet metrics (`kubelet_*`)
- kube-apiserver metrics (`apiserver_*`)
- kube-scheduler metrics (`scheduler_*`)
- kube-controller-manager metrics (`workqueue_*`, etc.)
- etcd metrics (`etcd_*`)
- kube-state-metrics (`kube_*`)
- cadvisor container metrics (`container_*`)

Key sources:
- https://kubernetes.io/docs/reference/instrumentation/metrics/
- https://github.com/kubernetes/kube-state-metrics/tree/main/docs
- https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md

---

### Batch 2: Node & Infrastructure
**Subagent focus**: Host-level and infrastructure metrics

Research:
- node_exporter (`node_*`)
- process-exporter (`namedprocess_*`)
- systemd exporter (`systemd_*`)
- cAdvisor standalone (if different from kubelet-embedded)

Key sources:
- https://github.com/prometheus/node_exporter
- https://github.com/ncabatoff/process-exporter
- https://prometheus.io/docs/guides/node-exporter/

---

### Batch 3: Language Runtimes
**Subagent focus**: Application runtime metrics across languages

Research:
- **Go**: default prometheus client metrics (`go_*`, `process_*`, `promhttp_*`)
- **Java/JVM**: Micrometer (`jvm_*`, `process_*`), JMX exporter patterns
- **Node.js**: prom-client default metrics (`nodejs_*`, `process_*`)
- **Python**: prometheus_client defaults (`python_*`, `process_*`)
- **.NET**: prometheus-net (`dotnet_*`, `process_*`)

Key sources:
- https://github.com/prometheus/client_golang
- https://micrometer.io/docs/concepts
- https://github.com/siimon/prom-client
- https://github.com/prometheus/client_python

---

### Batch 4: CNCF Ecosystem
**Subagent focus**: Cloud-native tooling metrics

Research:
- Prometheus (`prometheus_*`)
- Grafana (`grafana_*`)
- Flux (`gotk_*`, `flux_*`)
- ArgoCD (`argocd_*`)
- cert-manager (`certmanager_*`)
- external-secrets (`externalsecret_*`)
- Cilium (`cilium_*`, `hubble_*`)
- CoreDNS (`coredns_*`)
- Envoy (`envoy_*`)
- Istio (`istio_*`, `pilot_*`, `galley_*`)
- Linkerd (`linkerd_*`, `request_total`)
- KEDA (`keda_*`)
- Crossplane (`crossplane_*`)

Key sources:
- Official docs for each project
- https://monitoring.mixins.dev/

---

### Batch 5: Databases
**Subagent focus**: Database exporter metrics

Research:
- PostgreSQL (`pg_*`) — postgres_exporter
- MySQL (`mysql_*`) — mysqld_exporter
- Redis (`redis_*`) — redis_exporter
- MongoDB (`mongodb_*`) — mongodb_exporter
- Elasticsearch (`elasticsearch_*`) — elasticsearch_exporter

Key sources:
- https://github.com/prometheus-community/postgres_exporter
- https://github.com/prometheus/mysqld_exporter
- https://github.com/oliver006/redis_exporter
- https://github.com/percona/mongodb_exporter
- https://github.com/prometheus-community/elasticsearch_exporter

---

### Batch 6: Message Queues & Storage
**Subagent focus**: Async infrastructure metrics

Research:
- Kafka (`kafka_*`) — kafka_exporter, JMX exporter patterns
- RabbitMQ (`rabbitmq_*`)
- NATS (`nats_*`, `gnatsd_*`)
- MinIO (`minio_*`)
- Rook/Ceph (`ceph_*`, `rook_*`)

Key sources:
- https://github.com/danielqsj/kafka_exporter
- https://www.rabbitmq.com/prometheus.html
- https://github.com/nats-io/prometheus-nats-exporter
- https://min.io/docs/minio/linux/operations/monitoring/metrics-and-alerts.html

---

### Batch 7: HTTP & Networking
**Subagent focus**: Ingress, proxy, and HTTP metrics

Research:
- nginx (`nginx_*`, `nginxexporter_*`)
- HAProxy (`haproxy_*`)
- Traefik (`traefik_*`)
- ingress-nginx (`nginx_ingress_controller_*`)
- Generic HTTP patterns (OpenTelemetry HTTP semantic conventions)
- Generic gRPC patterns

Key sources:
- https://github.com/nginxinc/nginx-prometheus-exporter
- https://www.haproxy.com/documentation/haproxy-configuration-tutorials/alerts-and-monitoring/prometheus/
- https://doc.traefik.io/traefik/observability/metrics/prometheus/
- https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/
- https://opentelemetry.io/docs/specs/semconv/http/http-metrics/

---

### Batch 8: Conventions & Patterns
**Subagent focus**: Cross-cutting naming conventions and methodologies

Research:
- OpenMetrics conventions (`_total`, `_bucket`, `_sum`, `_count`, `_info`, `_created`)
- OpenTelemetry semantic conventions (metrics)
- RED method standard patterns
- USE method standard patterns
- Google SRE golden signals patterns
- Common anti-patterns to flag (e.g., high-cardinality labels)

Key sources:
- https://openmetrics.io/
- https://opentelemetry.io/docs/specs/semconv/
- https://www.weave.works/blog/the-red-method-key-metrics-for-microservices-architecture/
- https://www.brendangregg.com/usemethod.html
- https://sre.google/sre-book/monitoring-distributed-systems/

This batch should produce a special output format — patterns rather than specific metrics:

```json
{
  "batch": "conventions-patterns",
  "patterns": [
    {
      "pattern": "_total$",
      "pattern_type": "suffix",
      "inferred_metric_type": "counter",
      "inferred_signal_role": null,
      "role_disambiguation": "Depends on metric name: *_errors_total → errors, *_requests_total → traffic",
      "confidence": 0.9,
      "source": "openmetrics",
      "notes": "OpenMetrics convention for counters"
    },
    {
      "pattern": "histogram_quantile\\(.*_bucket\\)",
      "pattern_type": "promql",
      "inferred_signal_role": "latency",
      "confidence": 0.85,
      "notes": "histogram_quantile on _bucket metrics almost always indicates latency"
    }
  ]
}
```

---

## Execution Instructions

1. **Spawn subagents** for parallel research. Maximum 4-6 concurrent subagents.

2. **Each subagent** should:
   - Focus on one batch or a subset of a batch
   - Consult the specified sources
   - Produce JSON in the exact format specified
   - Include `sources_consulted` for traceability
   - Flag any metrics where classification is ambiguous

3. **After each batch completes**, present the output for my review. I may:
   - Approve and continue to next batch
   - Request corrections or additions
   - Ask for deeper research on specific exporters

4. **Handle conflicts** as follows:
   - Same metric name, different exporters, different meanings → separate entries with `source` disambiguation
   - Same metric, genuinely ambiguous role → pick primary use case, note alternatives in `notes`
   - Deprecated metric superseded by new metric → include both, link via `notes`

5. **Quality checks** before presenting each batch:
   - All required fields populated
   - `confidence` and `importance` are calibrated (not everything is 0.9+)
   - `signal_role` is one of the seven valid values
   - `metric_type` is one of: counter, gauge, histogram, summary, info, stateset, unknown
   - JSON is valid and parseable

## Example Output (Partial)

```json
{
  "batch": "kubernetes-core",
  "researched_at": "2025-01-30T14:30:00Z",
  "sources_consulted": [
    "https://kubernetes.io/docs/reference/instrumentation/metrics/",
    "https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md",
    "https://monitoring.mixins.dev/kubernetes/"
  ],
  "metrics": [
    {
      "name": "up",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 1.0,
      "importance": 1.0,
      "source": "prometheus/scrape",
      "metric_type": "gauge",
      "labels_of_interest": ["job", "instance"],
      "common_promql_patterns": [
        "up == 0",
        "avg by (job) (up)"
      ],
      "notes": "Universal Prometheus scrape health metric. 1 = target up, 0 = target down. First thing to check in any outage.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "kube_deployment_status_replicas_unavailable",
      "name_pattern": null,
      "signal_role": "availability",
      "confidence": 0.95,
      "importance": 0.9,
      "source": "kubernetes/kube-state-metrics",
      "metric_type": "gauge",
      "labels_of_interest": ["namespace", "deployment"],
      "common_promql_patterns": [
        "kube_deployment_status_replicas_unavailable > 0",
        "sum by (namespace) (kube_deployment_status_replicas_unavailable)"
      ],
      "notes": "Number of unavailable replicas. Non-zero indicates deployment health issue.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "container_cpu_usage_seconds_total",
      "name_pattern": null,
      "signal_role": "saturation",
      "confidence": 0.9,
      "importance": 0.85,
      "source": "google/cadvisor",
      "metric_type": "counter",
      "labels_of_interest": ["namespace", "pod", "container", "cpu"],
      "common_promql_patterns": [
        "rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])",
        "sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!=\"\"}[5m])) / sum by (namespace, pod) (kube_pod_container_resource_limits{resource=\"cpu\"})"
      ],
      "notes": "CPU time consumed. Use rate() and compare against limits for saturation. Exclude empty container label to avoid cgroup aggregates.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_duration_seconds",
      "name_pattern": null,
      "signal_role": "latency",
      "confidence": 0.95,
      "importance": 0.85,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "histogram",
      "labels_of_interest": ["verb", "resource", "subresource", "scope"],
      "common_promql_patterns": [
        "histogram_quantile(0.99, sum by (verb, le) (rate(apiserver_request_duration_seconds_bucket[5m])))",
        "histogram_quantile(0.99, rate(apiserver_request_duration_seconds_bucket{verb!~\"WATCH|LIST\"}[5m]))"
      ],
      "notes": "API server request latency. Exclude WATCH/LIST for meaningful percentiles. High latency here affects entire cluster.",
      "deprecated": false,
      "disabled_by_default": false
    },
    {
      "name": "apiserver_request_total",
      "name_pattern": null,
      "signal_role": "traffic",
      "confidence": 0.9,
      "importance": 0.7,
      "source": "kubernetes/kube-apiserver",
      "metric_type": "counter",
      "labels_of_interest": ["verb", "resource", "code", "component"],
      "common_promql_patterns": [
        "sum(rate(apiserver_request_total[5m])) by (verb)",
        "sum(rate(apiserver_request_total{code=~\"5..\"}[5m]))"
      ],
      "notes": "API server request count. Can also be used for errors when filtered by code=~\"5..\". Primary role is traffic; error detection is secondary.",
      "deprecated": false,
      "disabled_by_default": false
    }
  ]
}
```

---

## Begin

Start with **Batch 1: Kubernetes Core**. Spawn subagents as needed to parallelize research across kube-state-metrics, kubelet, apiserver, etc.

Present the completed Batch 1 JSON for my review before proceeding to Batch 2.
