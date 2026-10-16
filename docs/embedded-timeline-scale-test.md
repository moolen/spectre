# Embedded timeline storage: scale, sizing and limits

Scope: the embedded engine (`internal/embeddedstore`) behind `GET /v1/timeline`, covering
ingest, persistence and timeline queries, plus the watcher's List/Watch loop, which
decides how much data the engine receives. FalkorDB and the graph endpoints are out of
scope.

## Summary

The engine now keeps only compact per-version metadata in memory, 100–150 bytes per
change. Object payloads stay on disk and are read for the page being returned. Watch
re-lists no longer duplicate history, and checkpoints and compactions no longer block
ingest. As a result, retention is limited by memory roughly linearly in the number of
changes kept, and 7–30 days are practical for large clusters:

| Cluster (assumed churn after duplicate removal) | 7 days | 14 days | 30 days |
|---|---|---|---|
| Medium: 100 nodes, ~3k pods (3 changes/s, 1 Event/s) | 0.6 GB heap, 1 GiB limit, 3 GB disk | 0.9 GB, 1.25 GiB, 4 GB | 1.6 GB, 2.25 GiB, 9 GB |
| Large: 500 nodes, ~15k pods (10 changes/s, 3 Events/s) | 1.6 GB, 2.5 GiB, 7 GB | 2.8 GB, 4 GiB, 13 GB | 5.3 GB, 7.5 GiB, 28 GB |
| XL: 1000 nodes, ~30k pods (25 changes/s, 8 Events/s) | 3.7 GB, 5.25 GiB, 17 GB | 6.4 GB, 9 GiB, 33 GB | 12.6 GB, 18 GiB, 69 GB |

These figures come from the [sizing formula](#sizing). Measure your own rates and plug
them in; the churn assumptions above are rough.

What we measured on 4 vCPUs. Details are under [Results](#results).

| Run | Events / objects | Live heap | Disk | Ingest (batch / single) | Timeline page, any window |
|---|---|---:|---:|---|---|
| 7 days, `large` (500 nodes) | 335k / 108k | 231 MB | 0.4 GB | 7,258 / 2,169 ev/s | 3–9 ms |
| 14 days, `xlarge` (1000 nodes) | 2.4M / 490k | 722 MB | 2.3 GB | 5,381 / 1,518 ev/s | 4–36 ms |
| 30 days, `xlarge` (1000 nodes) | 5.07M / 970k | 1.34 GB | 4.0 GB | 5,029 / 1,113 ev/s | 11–60 ms |

The same 7-day workload against the code before this work:

| | Before | After |
|---|---|---|
| Live heap | 2.2 GB | 231 MB |
| Bulk ingest | 430 ev/s | 7,258 ev/s |
| Longest ingest stall | 36 s | 0.28 s |
| Full-window timeline page | 1.1 s | 9 ms |

## What changed

Each item below names the limit it removed.

1. **Payloads moved out of memory.**
   - Before: every version of every object kept its full JSON in the projection, about
     6–8 KB per change.
   - Now a version holds:
     - a 16-byte inline event ID
     - timestamp, status and change flags as one byte each
     - a shared identity block
     - a segment reference (`versionDataRef`) to its payload
   - That is **~100 bytes per version** in `TestScaleVersionMemory`, and ~150 bytes in the
     30-day run, where label changes add identity blocks.
   - Payloads are read with one `pread` and a zstd decode, and only for the resources on
     the page being returned.
2. **Binary segments and tail.**
   - Event metadata sits outside the compressed payload, so scans filter on kind,
     namespace, UID and time before decompressing anything.
   - Change detection uses xxhash fingerprints of the spec, status and kind-specific
     fields. It used to parse the object about five times per event; now it parses once.
3. **Watch re-lists no longer duplicate history.**
   - The API server closes every watch after 30–60 minutes. The watcher then re-listed
     the whole resource type and stored every object again. For a 150k-object cluster
     that was ~5M copies a day, more than the real churn.
   - The watcher now resumes from the last resourceVersion it received, using bookmarks,
     and only re-lists when the watch fails or expires.
   - The engine drops Create/Update events whose payload is byte-identical to the
     object's latest stored state. These are counted in
     `spectre_embedded_ingest_unchanged_events_total`. A re-list now costs ~1 µs per
     object and stores nothing (`TestScaleRelist`).
4. **Checkpoints off the lock.**
   - Format v3 is a binary delta encoding with payloads stored as segment references.
   - A checkpoint snapshots under the lock, encodes without it, and is triggered in the
     background every 50k tail events or 256 MiB.
   - It used to serialise the whole projection with gob while holding the engine mutex,
     every 2048 events.
5. **Tiered compaction in the background, and retention without rewrites.**
   - Compaction merges neighbouring segments up to 512 MiB and 24 hours, copying raw
     records. It used to decode every event of every segment into one slice under the
     lock.
   - Retention retires a segment once all its records have expired. A segment that
     straddles the cutoff is kept whole, and queries clamp to the cutoff. Rewriting it on
     every checkpoint stalled ingest for seconds and wrote more than was ingested.
6. **Resource pages independent of the window.**
   - Pages come from an ordered B-tree walked from the cursor. Each resource's window is
     checked against in-memory metadata, and payloads are read for the page only.
   - A 30-day page costs about the same as a 15-minute one.
7. **No separate Event query per timeline request.**
   - The handler used to run a core/v1 Event query over the whole window on every
     request. The embedded executor already attaches each page resource's Events, so
     that query was pure overhead: 6.8 s for 30 days.
   - Executors that attach Events now say so (`AttachesK8sEventsToResources`), and the
     query is skipped.
8. **Event summaries kept in memory for 48 hours.**
   - Per-object core/v1 Event summaries (reason, message, count) are trimmed after
     `K8sEventSummaryHorizon`, 48h by default.
   - Older windows read them from each segment's associated-UID index, for both the
     timeline and the analysis store.
   - Kept for 30 days, they would be the largest part of the heap: ~450 bytes each.
9. **Long-window scans and responses bounded.**
   - Segment scans read each record's timestamp, kind and namespace without allocating,
     skip non-matching records before decoding them, and run across segments in
     parallel.
   - When a resource has more than 256 versions in the window, the timeline page leaves
     out versions that repeat the previous status without a configuration change, such
     as Node heartbeats. Creates, deletes, configuration changes, status transitions and
     the last version are always kept, so the UI's version diffs still cover every
     change. A 30-day page of 100 Nodes went from 61,821 versions (~300 MB) to 200.
10. **Smaller items.**
   - The recent-Event cache is gone. It held 4 hours of Event payloads and gave no
     latency benefit.
   - Kinds, namespaces and labels are interned.
   - The hot store no longer drops unflushed events.
   - Retention pins the payloads of versions that start a window.
   - The chart sets `GOMEMLIMIT` from the memory limit.

## Results

All runs use the harness described under
[How to reproduce](#how-to-reproduce), on 4 vCPU with local disk.

### Reference workload: `large` profile, 7 days, 20% core/v1 Events

500 nodes, 200 namespaces, 108k distinct objects, 335k events, 1.08 GB of object JSON.

| Metric | Before | After |
|---|---:|---:|
| Bulk ingest (batches of 256) | 430 ev/s | 7,258 ev/s |
| Longest batch stall | 32.7 s | 0.28 s |
| Live ingest (one `ProcessEvent` per event) | 87 ev/s | 2,169 ev/s |
| Live ingest p99 / max | 14.5 ms / 36 s | 1.2 ms / 153 ms |
| Checkpoints (count × mean) | 145 × 4.6 s | 6 × 1.0 s (off the lock) |
| Compactions (count × mean) | 26 × 17.2 s | 14 × 0.13 s (background) |
| Live heap / peak heap | 2,235 / 5,495 MB | 231 / 452 MB |
| Disk | 277 MB | 376 MB |
| Close / reopen | 5.2 s / 4.2 s | 1.0 s / 0.83 s |
| Resources, last 1h (page of 100) | 6.4 ms | 3.7 ms |
| Resources, last 24h | – | 2.7 ms |
| Resources, full 7 days | 1,147 ms | 8.7 ms |
| Resources, full 7 days, one namespace | 61 ms | 9.4 ms |
| Resources, full 7 days, `kind=Node` | 156 ms | 84 ms |
| Walk every page of the last hour (82 × 500) | 607 ms | 630 ms |
| 8 concurrent timeline requests, 1h | 431 ms | 4.7 ms |
| Event query, full 7 days / one namespace* | 15.0 s / 15.4 s | 213 ms / 28 ms |

\* Before this work, every timeline request also ran this query. It is now only used
when someone asks for the `Event` kind explicitly.

### 14 and 30 days: `xlarge` profile, 10% core/v1 Events

1000 nodes, 400 namespaces, with retention set to the window:

- **14 days:** 2.33M churn events, 2.4M events in total over 490k distinct objects,
  8.3 GB of object JSON.
- **30 days:** 5M churn events, 5.07M events over 970k distinct objects, 17.4 GB of
  object JSON.

Most of the objects are pods replaced over the window; the rest include 250k and 500k
Event objects respectively.

| Metric | 14 days | 30 days |
|---|---:|---:|
| Bulk ingest (batches of 256) | 5,381 ev/s | 5,029 ev/s |
| Batch latency p50 / p99 / max | 27 / 251 / 560 ms | 29 / 256 / 1,177 ms |
| Live ingest (one `ProcessEvent` per event) | 1,518 ev/s | 1,113 ev/s |
| Live ingest p99 / max | 3.5 / 319 ms | 5.8 / 563 ms |
| Checkpoints (count × mean, off the lock) | 47 × 6.3 s | 90 × 9.4 s |
| Compactions (count × mean, background) | 168 × 0.54 s | 348 × 0.57 s |
| Live heap / peak heap (no `GOMEMLIMIT`) | 722 / 1,653 MB | 1,342 / 3,316 MB |
| Disk: segments / checkpoints / tail | 1.9 / 0.22 / 0.15 GB | 3.5 / 0.44 / 0.15 GB |
| Close / reopen / first query after reopen | 3.4 s / 4.2 s / 24 ms | 6.8 s / 8.3 s / 69 ms |

| Timeline query, page of 100 resources (p50) | 14 days | 30 days |
|---|---:|---:|
| Resources, last 15m / 1h / 6h | 7.5 / 4.3 / 4.6 ms | 12 / 11 / 11 ms |
| Resources, last 24h / 7d | 6.4 / 11.5 ms | 21 / 27 ms |
| Resources, full window | 17 ms | 41 ms |
| Resources, full window, one namespace | 36 ms | 60 ms |
| Resources, full window, `kind=Node` (versions returned) | 31 ms (451) | 51 ms (200) |
| 8 concurrent timeline requests, 1h | 29 ms | 190 ms |
| Explicit `kind=Event` query, 24h / 7d | 84 / 439 ms | 68 / 386 ms |
| Explicit `kind=Event` query, full window / one namespace | 1.3 s / 0.20 s | 2.9 s / 0.34 s |

Heap profile after the 30-day load (pprof `inuse_space`, ~1.2 GB sampled):

| Holder | Size |
|---|---:|
| Version slices | ~400 MB |
| Label maps | ~200 MB |
| Resource records and indexes | ~160 MB |
| Shared identity blocks | ~85 MB |
| Ordered B-tree | ~55 MB |
| In-flight checkpoint snapshot | ~58 MB |

An earlier 30-day run had:

- a 2.1 GB live heap and a 2.7 s worst batch stall
- a 6.8 s Event query on every timeline request
- 61,821 versions on a 100-Node page

That run came before:

- inline event IDs and string interning
- the Event summary horizon
- dropping the per-request Event query
- retention without rewrites
- bounded per-resource histories

## Sizing

Live heap is roughly:

```
heap ≈ 150 MB
     + 1.3 KB × objects that existed during the retention window
     + 0.15 KB × resource changes in the retention window
     + 0.45 KB × core/v1 Events in the last 48h
```

- **Objects** includes deleted ones still inside the window, such as replaced pods.
  Labels make up a third of the per-object cost. Against the measured runs the formula
  is slightly conservative:

  | Run | Formula | Measured live heap |
  |---|---:|---:|
  | 7 days | 253 MB | 231 MB |
  | 14 days | 825 MB | 722 MB |
  | 30 days | 1.46 GB | 1.34 GB |
- **Changes** count after duplicate removal; `spectre_embedded_ingest_events_total`
  counts exactly these.
- **The Event term** is bounded by `K8sEventSummaryHorizon` and by 64 summaries per
  involved object.
- **The graph store** adds about 0.4 KB per object for its edge facts and indexes; see
  [embedded-graph-scale-test.md](embedded-graph-scale-test.md#sizing).

Set `resources.limits.memory` to about 1.5× that. The chart sets `GOMEMLIMIT` from the
limit (`goMemLimitFromResources: true`), so the GC works to stay below it. The headroom
covers:

- checkpoint encoding, which holds a snapshot of slice headers and Event summaries
- compaction and queries in flight

Without `GOMEMLIMIT`, peak heap runs at about 2× live heap.

Disk:

- about **0.8 KB per stored event** (segments, zstd-compressed)
- plus up to 2 × 256 MiB of tail journal
- plus the last 3 checkpoints, each about 10% of segment size

The chart's default `persistence.enabled: false` keeps data on an `emptyDir`, so it does
not survive the pod. For 7 days or more, enable persistence and size the volume with the
numbers above.

Measuring your rates:

- Changes: `rate(spectre_embedded_ingest_events_total[1h])`. This includes core/v1
  Events.
- Events alone: count `kind=Event` objects in the timeline, or use the cluster's own
  Event rate.
- Duplicates dropped: `spectre_embedded_ingest_unchanged_events_total`.

## Limits and caveats

- **Per-resource history on long windows is thinned.**
  - Past 256 versions in the window, a timeline page leaves out status-only updates
    that keep the same status. The same applies to the MCP timeline tools, which share
    the query path.
  - Changes inside those updates, such as a restart count or a heartbeat time, appear
    only in the diff between the kept neighbours.
  - Objects whose status really flips hundreds of times in a window still return every
    transition.
- **Node heartbeats still cost memory.** Kubelets rewrite Node status (heartbeat
  timestamps) every few minutes even when nothing changes: 1000 nodes produce ~290k
  versions a day, all kept in the version index for the whole retention window. Not
  storing heartbeat-only status changes at ingest would save that memory, but it would
  also lose them from history, so it is left as a decision for later.
- **Explicit Event-kind queries** (`kind=Event` in the timeline) read Event metadata
  for the whole window. That is 0.2 s for 7 days on the reference workload and 2.9 s
  for 30 days with 500k Events, or 0.34 s with a namespace filter. Default timeline
  requests no longer run this query.
- **Older Event summaries are read from disk.**
  - The first query reaching back more than 48h loads the associated-UID index of each
    segment in its window.
  - Those indexes stay cached, and their memory grows with the number of Events in them.
- **Checkpoint CPU grows with history.** A checkpoint encodes the whole projection
  without blocking ingest, but still costs CPU: ~9 s each at the 30-day scale (6 s at
  14 days), every 50k events.
- **Bulk-load stalls.** While loading at thousands of events per second, the longest
  batch stall at 30 days was 1.2 s, while a single live event waited at most 563 ms.
  Flushes, and compaction and retention commits, hold the engine lock briefly. Real ingest rates are far below the measured ceiling.
- **Storage format.**
  - Segments, tail and checkpoints use new binary formats (segment record magic `0xB1`,
    tail `0xB2`, checkpoint v3), and the manifest has new fields.
  - Older formats (JSON records, v2 checkpoints) are still read, so upgrading in place
    works.
  - Downgrading to an older Spectre does not; wipe the data directory to do that.

## How to reproduce

The harness sits behind the `scale` build tag, so it never runs in `make test`.

```bash
# Reference workload (7 days, 335k events), ~2 minutes:
SPECTRE_SCALE_PROFILE=large SPECTRE_SCALE_CHURN=300000 SPECTRE_SCALE_K8S_EVENTS=20 \
SPECTRE_SCALE_WINDOW=168h SPECTRE_SCALE_OUT=/tmp/ref.json \
  go test -tags scale -run 'TestTimelineScale$' -timeout 0 -v ./internal/embeddedstore/

# 30 days, 5M events, ~20 minutes and ~6 GB of disk:
SPECTRE_SCALE_PROFILE=xlarge SPECTRE_SCALE_CHURN=5000000 SPECTRE_SCALE_K8S_EVENTS=10 \
SPECTRE_SCALE_WINDOW=720h SPECTRE_SCALE_RETENTION_DAYS=30 SPECTRE_SCALE_DIR=/data/scale \
SPECTRE_SCALE_OUT=/tmp/30d.json SPECTRE_SCALE_HEAPPROFILE=/tmp/30d.heap \
  go test -tags scale -run 'TestTimelineScale$' -timeout 0 -v ./internal/embeddedstore/

# Heap per stored version, and the cost of a watch re-list:
go test -tags scale -run 'TestScaleVersionMemory$|TestScaleRelist$' -v ./internal/embeddedstore/
```

The workload in `scale_workload_test.go` is deterministic and shaped like watcher
output. Object sizes match real objects with managedFields stripped: Pod ~4 KB, Node
~5 KB, Deployment and ReplicaSet ~3 KB, ConfigMap ~2 KB. The churn mix is:

| Change | Share |
|---|---:|
| Pod status updates | 55% |
| Pod replacements (delete, create, 2 updates) | 15% |
| Rollouts | 3% |
| Node status updates | 20% |
| ConfigMap edits | 7% |
| core/v1 Events | optional share of all churn |

Timestamps are spread evenly over the window, ending now.

`scale_test.go` opens the engine through `Open(Config{...})` with the production
defaults. It then measures:

- bulk load (`ProcessBatch`, 256 events per batch)
- live ingest (5k single `ProcessEvent` calls, as the watcher makes them)
- checkpoint, flush and compaction cost, from the engine's metrics
- live and peak heap, and disk
- timeline queries through `ExecutePaginated`, as the HTTP handler calls it
- a page walk and concurrent requests
- close and reopen

Environment knobs are listed at the top of `scale_test.go`.
