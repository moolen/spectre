# Embedded graph storage: scale, sizing and limits

Scope: the analysis store of the embedded engine (`internal/embeddedstore`, `Store`) and
what is built on it: `GET /v1/namespace-graph` with its cache, anomaly detection,
causal paths and root-cause analysis. The timeline is covered in
[embedded-timeline-scale-test.md](embedded-timeline-scale-test.md); FalkorDB is out of
scope.

## Summary

Before this work the graph store did not scale past small clusters. Every namespace
graph request built edges for every resource in the cluster. To do that it read,
decompressed and parsed about three payloads per resource from disk, all under the
projection read lock. Cost grew linearly with cluster size, and ingest waited behind
each request:

| Cluster | Namespace graph, page of 50 | Root cause, one pod | Cache pre-warm cycle | Ingest while graph queries run |
|---|---:|---:|---:|---|
| small: 10 nodes, 2.4k live objects | 0.3 s | 0.6 s | 4 s | 3 ev/s, 365 ms per event |
| medium: 100 nodes, 17.6k live objects | 2.4 s | 4.9 s | 120 s | 0.4 ev/s, up to 4.8 s per event |
| large: 500 nodes, 75k live objects | 24 s | 50 s | 81 min | stalled, 49 s per event |
| xlarge: 1000 nodes, 150k live objects | 94 s | 190 s | – | stalled, 96 s per event |

Two things made this a hard limit rather than slow pages:

- The namespace graph cache, enabled by default, pre-warmed every namespace every
  2 minutes. On a medium cluster one cycle took the whole 2 minutes, so the cache kept
  the store busy and ingest stalled continuously. On a large cluster one cycle would
  have taken 81 minutes.
- The UI loads namespace graphs with anomaly detection. That runs a root-cause analysis
  for every recently changed resource on the page, and each analysis made two more
  full-cluster graph queries.

Now edges come from in-memory facts and reverse indexes. Query cost depends on the
namespace and page, not on the cluster:

| Cluster | Namespace graph, page of 50 / 500 | Root cause, one pod | Cache pre-warm, all namespaces | Ingest while graph queries run back to back |
|---|---:|---:|---:|---|
| medium | 0.5 / 3 ms | 11 ms | 0.2 s | 327 ev/s, p99 11 ms |
| large | 0.2 / 7 ms | 10 ms | 1.6 s | 230 ev/s, p99 13 ms |
| xlarge | 0.2 / 7 ms | 12 ms | 3.5 s | 164 ev/s, p99 22 ms |

Real ingest rates are well below these figures. A large cluster produces about 10
changes per second; the measured ceiling under a saturating query loop is 20 times
that.

The price is memory. Facts and indexes add about **0.4 KB per object** retained in the
window, and bulk ingest is 10–15% slower.

**What we support.** Medium, large and xlarge clusters (1000 nodes, 30k pods, 150k live
objects) answer every graph query in milliseconds. One case is still slow: loading
every page of one very large namespace. The UI does that with auto-load, and it takes
1.5 s for a namespace with 13.4k objects. See [Limits](#limits-and-caveats).

## What changed

1. **Graph facts at ingest.**
   - The fields edges are derived from are extracted when an event is applied, from
     the payload that is parsed there anyway. They are owner UIDs, name references
     (volumes, env sources, service account, node, role bindings, Ingress backends,
     Flux sources), the pod selector and `spec.replicas`.
   - They live on `versionMeta`, which consecutive versions of an object already share.
     A meta is split only when the facts change.
   - The pods of one ReplicaSet share one copy of everything but their node.
2. **Reverse indexes.** The projection indexes records by namespace, by owner UID and by
   the `(namespace, kind, name)` their references and GitOps manager labels resolve to.
   Index entries are candidates. A query keeps only those whose version at the query
   time really has the edge, so history and stale entries never produce one. Retention
   removes entries together with the records and metas it drops.
3. **A walk instead of a full-cluster scan.**
   - A namespace graph needs the cluster-scoped resources within `maxDepth` hops of its
     page. The previous code built every edge of the cluster to find them.
   - Now it walks from the page, expanding only what it reaches.
   - On the last hop it only checks cluster-scoped neighbours. Selector edges never
     leave a namespace, so with the default depth of 1 no selector is evaluated
     outside the page.
4. **Payloads off the lock.** Spec diffs are the only payloads a namespace graph still
   reads, two per returned node. They are read after the projection lock is released.
5. **Paging without sorting the namespace.** Each page keeps the next `limit+1`
   resources while scanning the namespace once, instead of sorting all of it.
6. **Checkpoints store the facts**, in a new identity-block marker of checkpoint v3.
   Older checkpoints still load: their facts are derived once at startup from one
   payload per meta. If an object's edge fields changed while its labels stayed the
   same (a pod being scheduled), that pre-upgrade history shows the later fields. It
   ages out with retention.
7. **Namespace graph cache keyed by request.**
   - The cache returned its one entry per namespace for any request on that namespace,
     ignoring timestamp, cursor, page size and the anomaly and causal path flags. The UI
     always asks for anomalies and causal paths. It got the pre-warmed view instead:
     500 nodes, no anomalies, the present instead of the selected time, and a repeated
     first page that ended auto-loading.
   - Entries are now keyed by the normalized request. Requests for "now" fall into
     30-second buckets, so reloads and several viewers share entries.
   - The pre-warm and refresh loop is gone: it computed a view no client requests.

Parity tests (`analysis_store_graph_parity_test.go`) keep the previous payload-parsing
implementation and compare namespace graphs (every page, depths 1 to 4), related
resources and ownership chains on randomized clusters. The clusters cover every edge
type, deletes with and without payloads, name reuse, rescheduling, selector and label
changes, out-of-order events and retention. The comparison also runs after a
checkpoint round trip, with and without stored facts, and while ingest, flushes and
checkpoints run concurrently under the race detector.

## Results

All runs used 4 vCPUs and local disk, with 24 hours of churn and 5% core/v1 Events. The
workload is described under [How to reproduce](#how-to-reproduce). Latencies are p50.

| Profile | Nodes / namespaces | Events | Objects (distinct / live) | `team-big` objects |
|---|---|---:|---:|---:|
| medium | 100 / 51 | 260k | 57k / 17.6k | 2.9k |
| large | 500 / 201 | 809k | 194k / 75k | 6.7k |
| xlarge | 1000 / 401 | 1.62M | 389k / 150k | 13.4k |

`team-001` is a typical namespace (290–340 objects). `team-big` has 10–40 times more,
the "everything in one namespace" shape.

### Store queries

| Query | medium before → after | large before → after | xlarge before → after |
|---|---:|---:|---:|
| `GetNamespaceGraph` team-001, 50, depth 1 | 2,409 → 0.5 ms | 24,186 → 0.2 ms | 93,534 → 0.2 ms |
| `GetNamespaceGraph` team-001, 500, depth 1 | 2,419 → 3.0 ms | – → 6.7 ms | – → 6.9 ms |
| `GetNamespaceGraph` team-001, 50, depth 3 | 2,432 → 8.0 ms | – → 8.6 ms | – → 11 ms |
| `GetNamespaceGraph` team-001, 50, 12h ago | 1,965 → 0.9 ms | – → 0.2 ms | – → 0.3 ms |
| `GetNamespaceGraph` team-big, 500, depth 1 | 2,426 → 5.3 ms | – → 4.9 ms | – → 6.8 ms |
| `GetRelatedResources` pod | 64 → 0.1 ms | 331 → 0.1 ms | – → 0.1 ms |
| `GetRelatedResources` pod in team-big | 120 → 1.0 ms | – → 1.9 ms | – → 4.6 ms |
| Payloads read per team-001 page of 50 | 48,391 → 2 | 206,243 → 0 | 412,799 → 0 |
| Memory allocated per team-001 page of 50 | 554 MB → 0.1 MB | 2.3 GB → 0.1 MB | 4.6 GB → 0.1 MB |

### Analysis built on the store

| Request | medium before → after | large before → after | xlarge before → after |
|---|---:|---:|---:|
| Root-cause analysis of one pod | 4,921 → 11 ms | 50,084 → 10 ms | 190,431 → 12 ms |
| Anomaly detection for one pod | 4,839 → 12 ms | – → 9.5 ms | – → 10 ms |
| UI page: team-001, 50 nodes, anomalies and causal paths | 2,445 → 0.8 ms | – → 0.5 ms | – → 0.3 ms |
| UI, every page of team-001 | 14.3 s → 6 ms | – → 9 ms | – → 8 ms |
| UI, every page of team-big | 142 s → 105 ms | – → 421 ms | – → 1.5 s |
| Cache pre-warm, every namespace at 500 nodes | 120 s → 0.2 s | 81 min → 1.6 s | – → 3.5 s |

Anomaly detection runs a full root-cause analysis only for resources that changed
within 5 minutes of the requested time. Others return immediately. So a UI page costs
about 10 ms per recently changed Pod, workload, Service, Ingress or Node on it.

### Ingest and timeline while graph queries run

The harness pushes 2,000 single events through `ProcessEvent`, 1 ms apart, and runs a
timeline query every 100 events. It does this alone, then while a goroutine runs 500-node
namespace graph queries back to back. That is a saturating load: hundreds of graph
queries per second.

| Profile | Alone: ingest rate, p99 | With graph queries: before | With graph queries: after |
|---|---|---|---|
| medium | 573 ev/s, 1.2 ms | 0.4 ev/s, p99 4.8 s | 327 ev/s, p99 11 ms, max 36 ms |
| large | 520 ev/s, 2.3 ms | 0.03 ev/s, 49 s per event | 230 ev/s, p99 13 ms, max 37 ms |
| xlarge | 539 ev/s, 2.0 ms | stalled, 96 s per event | 164 ev/s, p99 22 ms, max 31 ms |

Timeline queries were unaffected in all runs (p50 2–4 ms).

### Memory and ingest cost

Live heap after loading, measured once background compaction and checkpoints had
finished:

| Profile | Before | After | Per distinct object |
|---|---:|---:|---:|
| medium (57k objects) | 134 MB | 156 MB | +0.39 KB |
| large (194k objects) | 373 MB | 451 MB | +0.40 KB |
| xlarge (389k objects) | 707 MB | 849 MB | +0.37 KB |

Bulk ingest got 10–15% slower: the large profile loaded in 112–117 s instead of 100–101 s.

## Sizing

Add the graph's share to the timeline formula in
[embedded-timeline-scale-test.md](embedded-timeline-scale-test.md#sizing):

```
heap ≈ timeline formula + 0.4 KB × objects that existed during the retention window
```

For the timeline guide's 30-day xlarge example (970k objects) that is about 0.4 GB
more. The UI no longer needs headroom for background graph queries. The cache holds
at most `--namespace-graph-cache-memory-mb` (256 MB by default) of responses, and it
fills only with what clients request.

## Limits and caveats

- **Very large namespaces.**
  - A page costs time proportional to the namespace's objects, including those deleted
    within the retention window. That is about 0.5 ms per 1,000 objects.
  - The UI loads every page of a namespace, 50 at a time. For 13.4k objects that is 269
    pages and 1.5 s in total.
  - A cursor-ordered index per namespace would make pages independent of the
    namespace's size.
- **Depth above 1.**
  - A walk through a Node visits every pod scheduled on it within the retention window,
    deleted ones included, to check which are visible at the query time.
  - Depth 3 cost 8–10 ms in these runs. With 30 days of retention and heavy pod churn
    expect more.
  - The default depth is 1.
- **Anomaly detection** costs about 10 ms per recently changed resource on a page. A page
  of 50 resources that all changed in the last 5 minutes takes about half a second.
- **Graph queries still take the projection read lock.** A 500-node page holds it for up
  to about 10 ms, and ingest of a single event waits at most that long.
- **Facts from older checkpoints are approximate.** They are derived from one payload
  per meta at the first start after upgrading. Edges in history recorded before the
  upgrade can show an object's later references if they changed without a label change.
- **Format.**
  - Checkpoints written by this version use a new identity-block marker. Older Spectre
    versions cannot read them.
  - As with the timeline changes, downgrading means wiping the data directory.

## How to reproduce

The harness sits behind the `scale` build tag.

```bash
# medium, about 3 minutes; large about 8, xlarge about 15 (and ~2 GB of heap):
SPECTRE_GRAPH_PROFILE=medium SPECTRE_GRAPH_OUT=/tmp/graph-medium.json \
  go test -tags scale -run 'TestGraphScale$' -timeout 0 -v ./internal/embeddedstore/

# Only some phases, e.g. the slow queries of the old code at scale:
SPECTRE_GRAPH_PROFILE=large SPECTRE_GRAPH_BUDGET=30s \
SPECTRE_GRAPH_QUERIES='^(store NamespaceGraph team-001 limit=50 depth=1|analysis RootCause pod|prewarm|interference)$' \
  go test -tags scale -run 'TestGraphScale$' -timeout 0 -v ./internal/embeddedstore/
```

`graph_scale_workload_test.go` generates the cluster deterministically.

- **Per namespace:** each app has a Deployment, ReplicaSet, 3 Pods, Service,
  ServiceAccount, Role, RoleBinding, ConfigMap and Secret. Every fourth app also has
  an Ingress. Each namespace has a Flux HelmRelease that labels its Deployments.
- **Cluster-scoped:** Nodes and a GitRepository.
- **Selectors and references are real:** selectors match pod labels, and pods
  reference their ConfigMap, Secret, ServiceAccount and Node.
- **Object sizes** match the timeline workload: Pod ~4 KB, Node ~5 KB.
- **Churn** uses the timeline mix: pod status updates (5% crash-looping), pod
  replacements, rollouts that replace every pod, Node heartbeats and ConfigMap edits.

`graph_scale_test.go` measures:

- each store query and the analyzers as the HTTP handler calls them
- every page of a namespace, as the UI loads it
- the cache's former pre-warm cycle
- ingest and timeline latency with and without concurrent graph queries

Knobs are listed at the top of the file.
