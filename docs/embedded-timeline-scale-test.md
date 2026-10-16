# Embedded timeline storage: scale test results

Date: 2026-10-16. Scope: the embedded engine (`internal/embeddedstore`) as used by the
timeline (`GET /v1/timeline`): ingest, persistence, and timeline queries. FalkorDB is out of
scope, as are the analysis and graph endpoints.

## How to reproduce

The harness is opt-in. It sits behind the `scale` build tag, so it never runs in `make test`.

```bash
SPECTRE_SCALE_PROFILE=medium SPECTRE_SCALE_OUT=/tmp/medium.json \
  go test -tags scale -run TestTimelineScale -timeout 0 -v ./internal/embeddedstore/
```

- `scale_workload_test.go` generates a deterministic, watcher-shaped workload. Object sizes
  match real objects with managedFields stripped: Pod ~4 KB, Node ~5 KB, Deployment and
  ReplicaSet ~3 KB, ConfigMap ~2 KB. The churn mix is:
  - pod status updates: 55%
  - pod replacements (delete, create, 2 updates): 15%
  - rollouts: 3%
  - node status updates: 20%
  - ConfigMap edits: 7%
  - optional core/v1 Events
- `scale_test.go` opens the engine through `Open(Config{...})` with the chart defaults:
  - 15m checkpoint interval
  - `retentionDays: 0`
  - checkpoint every 2048 tail events
  - 16 MiB segments
  - auto-compaction at 4 segments
- It then measures:
  - bulk load, using `ProcessBatch` with batches of 256
  - live ingest: the last 5k events go one at a time through `ProcessEvent`, which is
    exactly what the watcher does
  - checkpoint, flush and compaction cost, read from the engine's Prometheus metrics
  - heap and disk usage
  - timeline queries, made through the same `ExecutePaginated` calls the HTTP handler uses
  - close and reopen

Test machine: 4 vCPU, 15 GB RAM, local disk with fsync p50 0.27 ms / p99 3 ms. Cloud
block storage typically has 1–3 ms fsync, so per-event ingest will be somewhat worse there.

## Results

| Run | Resources | Events | Object JSON | Live heap | Peak heap | Disk |
|---|---:|---:|---:|---:|---:|---:|
| small | 6.6k | 51k | 184 MB | 425 MB | 840 MB | 43 MB |
| medium | 37k | 257k | 931 MB | 2.08 GB | **4.82 GB** | 220 MB |
| xlarge objects (150k initial objects, light churn) | 83k | 171k | 584 MB | 1.38 GB | 2.89 GB | 147 MB |
| medium + 40% core/v1 Events | 54k | 127k | 357 MB | 740 MB | 1.77 GB | 96 MB |
| medium, tuned (checkpoint per 100k events, no auto-compaction) | 37k | 257k | 931 MB | 1.97 GB | 4.08 GB | 396 MB |

| Run | Bulk ingest | Live ingest (per event) | Longest ingest stall | Checkpoint mean / count | Compaction mean / count | Close | Reopen |
|---|---:|---:|---:|---:|---:|---:|---:|
| small | 1,506 ev/s | 233 ev/s, p50 3.9 ms | 4.4 s | 0.55 s / 22 | 3.0 s / 2 | 5.5 s | 0.4 s |
| medium | 526 ev/s | 107 ev/s, p50 3.0 ms | **27.9 s** | 3.6 s / 111 | 14.0 s / 19 | 2.7 s | 2.4 s |
| xlarge objects | 697 ev/s | 227 ev/s, p50 3.3 ms | 17.5 s | 2.1 s / 74 | 8.8 s / 11 | 17.8 s | 1.7 s |
| medium + Events | 1,166 ev/s | 268 ev/s, p50 2.9 ms | 10.2 s | 1.2 s / 55 | 5.9 s / 7 | 11.2 s | 0.9 s |
| medium, tuned | 2,353 ev/s | 236 ev/s, p50 3.6 ms | 2.3 s | 1.7 s / 2 | – | 2.8 s | 2.4 s |

Timeline query latency, p50 of 5 runs. Resource queries return page 1 at 100 resources per
page; Events use the handler's unpaginated Event query.

| Query | small | medium | xlarge objects | medium + Events |
|---|---:|---:|---:|---:|
| resources, last 15m | 1.7 ms | 4.7 ms | 4.4 ms | 3.4 ms |
| resources, last 1h | 5.2 ms | 35 ms | 33 ms | 14 ms |
| resources, last 6h | 27 ms | 183 ms | 194 ms | 83 ms |
| resources, last 24h | 133 ms | 798 ms | 694 ms | 289 ms |
| resources, 24h, one namespace | 13 ms | 35 ms | 66 ms | 16 ms |
| core/v1 Events, last 1h | – | – | – | 139 ms |
| core/v1 Events, last 6h | – | – | – | **1.23 s** |
| core/v1 Events, last 24h | – | – | – | **5.14 s** |
| page through all resources in 1h (pages of 500) | 23 ms | 1.1 s | 5.9 s (148 pages) | 0.35 s |
| 8 concurrent timeline requests, 1h | 16 ms | 111 ms | 198 ms | 487 ms |

## What limits scale

### 1. Memory grows with every event ever retained (the hard ceiling)

`Projection.appendResourceVersion` keeps a full copy of the object JSON for **every version
of every resource**. The heap profile of the xlarge run shows 72% of live heap in
`cloneBytes`, called from `appendResourceVersion`. Live heap comes to 6–8.5 KB per retained
event, roughly the payload plus 2–4 KB for identity, labels and change info. The process
peaks at about 2× live heap:

- The chart sets no `GOMEMLIMIT`, so GC runs at the default `GOGC=100`.
- Compaction decodes every on-disk event back into memory (see 3).

With the chart defaults, memory never stops growing: `retentionDays: 0` means nothing is
pruned. The limit is `limits.memory: 1400Mi`, so the pod is OOM-killed after roughly
**70–90k retained events (~250–300 MB of object JSON)**. The small run already peaked at
840 MB with 51k events. The medium run crossed 1400Mi before 100k events.

Time until OOM is therefore about 80k events divided by your event rate. Your real rate is
`rate(spectre_embedded_ingest_events_total[1h])`.

| Event rate | Time until OOM (default chart) | Heap needed for 1 day of history (live → peak) |
|---|---|---|
| 1 ev/s | ~22 h | 0.7 → 1.5 GB |
| 5 ev/s | ~4.5 h | 3.5 → 7.5 GB |
| 20 ev/s | ~1 h | 14 → 30 GB |
| 100 ev/s | ~13 min | 70 → 150 GB |

The smallest retention you can set is `retentionDays: 1`. Even then, memory must hold a full
day of every version.

### 2. Ingest throughput and lock-held stalls

- **Per-event cost.** The watcher calls `ProcessEvent` once per event. Each call:
  - does one fsync
  - runs about five full `json.Unmarshal`s of the object (`parseObject`,
    `ParseResourceData`, `detectChangeKinds` on the old and new object, `ComputeJSONDiff`)
  - holds the global engine mutex throughout

  Measured: **~100–270 events/s sustained**, p50 3–4 ms, which is CPU, not fsync.
- **Checkpoints scale with history.** Every 2048 tail events (`checkpointMaxTailEvents`),
  `checkpointLocked` serializes the entire projection: gob, then zstd. It does this while
  holding the engine mutex. Cost grows linearly with history: 0.55 s mean at 51k events,
  3.6 s at 257k. The total write work therefore grows quadratically, and ingest is blocked
  for the full duration.
- **Compaction rewrites everything.** With 4 or more segments, `compactLocked` merges *all*
  segments into one. It decodes every event into a single slice, rewrites it, and does all
  of this under the engine mutex. It ran 19 times in the medium run, averaging 14 s with a
  28 s maximum. It is also the source of the second memory spike.
- **The tuned config removes the stalls.** Checkpointing every 100k events with
  auto-compaction off took the longest stall from 28 s to 2.3 s and made bulk ingest 4.5×
  faster. Memory stayed the same.
- **Shutdown is not bounded.** It does a flush and a checkpoint, which took up to 18 s
  here. The pod's `terminationGracePeriodSeconds` is 60 s, and the checkpoint is
  proportional to history.

### 3. Query cost

- **Resource timeline.** Cost is proportional to the number of resource *changes* in the
  window, not to the number of objects. `recentWindowChangedUIDsLocked` builds a set of
  every UID changed since `start`, and then sorts. That works out to about 3 µs per change:
  35 ms for 1h and ~0.8 s for 24h at 250k changes/day. Linear extrapolation gives ~3.5 s for
  a 24h view at 1M changes/day. A namespace filter helps a lot (24h: 35 ms instead of
  800 ms).
- **Per-resource history is unbounded.** A page holds at most 100–500 resources, but each
  resource returns *every* version in the window, with full `ResourceData`. The 24h page of
  100 Nodes returned 33,165 versions, about 200 MB of JSON in a single response. Real nodes
  that report status every minute would make this worse.
- **core/v1 Events: the slowest path.** `GET /v1/timeline` fires this query on *every*
  request, in parallel with the resource query.
  - Inside the 4h `recentEventCache`, each query still clones the whole cache.
  - Outside it (the UI's 6h, 24h and 48h presets), `exportTimeRange` decompresses and
    `json.Unmarshal`s every Event record in the window. It filters afterwards and returns
    only 100.
  - Cost is about 0.1 ms per Event in the window: 5.1 s for 24h with 48k Events. A cluster
    emitting 20 Events/s would need about 3 minutes per 24h request.
- **Object count by itself is fine.** 83k resources gave the same latency as 37k.

### 4. Disk is not a constraint

Segments and checkpoints compress about 4–6× (931 MB of JSON fits in 220 MB). The default
10 Gi PVC is far larger than memory allows the store to grow. With compaction disabled,
segments accumulate (57 after 257k events), and the tail journal grows to the configured
budget.

## What we can support today

| Cluster | Typical churn* | With default chart (1400Mi, no retention) | Best case with config only |
|---|---|---|---|
| Small (≤10 nodes, ≤500 pods) | ≤1 ev/s | Runs for roughly a day, then OOM | `retentionDays: 1`, 2 Gi limit, `GOMEMLIMIT`: OK |
| Medium (~100 nodes, ~3k pods) | 3–10 ev/s | OOM within hours | `retentionDays: 1` + 8–16 Gi + fewer checkpoints / no auto-compaction: workable, but 24h views ~1 s and Event views slow |
| Large (500–1000+ nodes, 15–30k pods) | 20–100+ ev/s | Not supported | Not supported: needs 30–150 GB heap for 1 day, and ingest sits at or above the ~100–250 ev/s single-writer ceiling |

\* Rough rates. Measure yours with `spectre_embedded_ingest_events_total`.

**Ceilings, in the order they bite:**

1. **Memory.** About 2× (retained events × average object size). Unbounded under the
   default config.
2. **Ingest.** About 100–250 events/s on one writer, minus checkpoint and compaction
   stalls that grow with history.
3. **Event query latency.** Linear in the number of core/v1 Events in the window, and
   charged to every timeline request.

## Recommended changes, by impact

1. **Stop keeping full object JSON for every historical version in the projection.**
   Keep the latest object, plus the small per-version metadata the timeline needs (status,
   change flags, timestamps). Hydrate historical `ResourceData` from segments using the
   existing `resource.idx`. Memory then scales with object count rather than event count,
   which also shrinks checkpoints. This is the change that moves the ceiling.
2. **Bound checkpoint and compaction work.**
   - Make checkpoints incremental, or at least take them off the engine mutex.
   - Replace merge-all compaction with size-tiered merging that streams records rather
     than materialising every event.
   - Short term: raise the `checkpointMaxTailEvents` default (2048 is far too small), and
     consider disabling auto-compaction (measured above).
3. **Make the Event query proportional to the page, not the window.**
   - Walk segments newest-first and stop once the page is full.
   - Filter on the index (namespace, involved UID) before decoding.
   - Store a compact `K8sEventInfo` instead of re-parsing full JSON.
   - Stop cloning the whole recent cache per request.
4. **Cap per-resource versions in a timeline response.** Downsample, or send `ResourceData`
   only for status transitions.
5. **Chart defaults.**
   - Set a default `retentionDays` (1–3).
   - Set `GOMEMLIMIT` from `limits.memory`.
   - Document a sizing formula: limit ≈ 2.5 × daily event volume × average object size ×
     retention days. Until item 1 lands, this is the only real way to size the pod.
   - Make sure `terminationGracePeriodSeconds` covers the shutdown checkpoint.
