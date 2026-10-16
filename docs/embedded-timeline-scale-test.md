# Embedded timeline storage: scale, sizing and limits

Scope: the embedded engine (`internal/embeddedstore`) behind `GET /v1/timeline`, covering
ingest, persistence and timeline queries, plus the watcher's List/Watch loop, which
decides how much data the engine receives. FalkorDB and the graph endpoints are out of
scope.

## Summary

The engine now keeps only compact per-version metadata in memory, about 100 bytes per
change. Object payloads stay on disk and are read for the page being returned. Watch
re-lists no longer duplicate history, and checkpoints and compactions no longer block
ingest. As a result, retention is limited by memory roughly linearly in the number of
changes kept, and 7–30 days are practical for large clusters:

| Cluster (assumed churn after duplicate removal) | 7 days | 14 days | 30 days |
|---|---|---|---|
| Medium: 100 nodes, ~3k pods (3 changes/s, 1 Event/s) | 0.5 GB heap, 0.75 GiB limit, 3 GB disk | 0.7 GB, 1 GiB, 4 GB | 1.2 GB, 1.75 GiB, 9 GB |
| Large: 500 nodes, ~15k pods (10 changes/s, 3 Events/s) | 1.2 GB, 2 GiB, 7 GB | 2.0 GB, 3 GiB, 13 GB | 3.7 GB, 6 GiB, 28 GB |
| XL: 1000 nodes, ~30k pods (25 changes/s, 8 Events/s) | 2.7 GB, 4 GiB, 17 GB | 4.6 GB, 7 GiB, 33 GB | 8.7 GB, 13 GiB, 69 GB |

These figures come from the [sizing formula](#sizing). Measure your own rates and plug
them in; the churn assumptions above are rough.

What we measured, on 4 vCPUs (details below):

- **30 days, 5.07M events** over 970k distinct objects (17.4 GB of object JSON):
  - ⟨LIVE30⟩ live heap, ⟨DISK30⟩ on disk
  - ingest ⟨LOAD30⟩ events/s in batches and ⟨LIVEEV30⟩ events/s one at a time
  - timeline pages in ⟨Q30⟩ for any window from 15 minutes to 30 days
  - restart in ⟨REOPEN30⟩
- **Reference workload, 7 days** (500 nodes, 108k objects, 335k events), against the
  code before this work:

  | | Before | After |
  |---|---|---|
  | Live heap | 2.2 GB | ⟨REFHEAP⟩ |
  | Bulk ingest | 430 ev/s | ⟨REFLOAD⟩ |
  | Longest ingest stall | 36 s | ⟨REFSTALL⟩ |
  | Full-window timeline page | 1.1 s | ⟨REFFULL⟩ |

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
   - That is **~100 bytes per version** (`TestScaleVersionMemory`).
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
5. **Tiered compaction in the background.**
   - It merges neighbouring segments up to 512 MiB and 24 hours, copying raw records.
   - It used to decode every event of every segment into one slice under the lock.
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
9. **Smaller items.**
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
| Bulk ingest (batches of 256) | 430 ev/s | ⟨R:load⟩ |
| Longest batch stall | 32.7 s | ⟨R:batchmax⟩ |
| Live ingest (one `ProcessEvent` per event) | 87 ev/s | ⟨R:live⟩ |
| Live ingest p99 / max | 14.5 ms / 36 s | ⟨R:livep99⟩ / ⟨R:livemax⟩ |
| Checkpoints (count × mean) | 145 × 4.6 s | ⟨R:ckpt⟩ |
| Compactions (count × mean) | 26 × 17.2 s | ⟨R:compact⟩ |
| Live heap / peak heap | 2,235 / 5,495 MB | ⟨R:heap⟩ |
| Disk | 277 MB | ⟨R:disk⟩ |
| Close / reopen | 5.2 s / 4.2 s | ⟨R:closeopen⟩ |
| Resources, last 1h (page of 100) | 6.4 ms | ⟨R:q1h⟩ |
| Resources, last 24h | – | ⟨R:q24h⟩ |
| Resources, full 7 days | 1,147 ms | ⟨R:qfull⟩ |
| Resources, full 7 days, one namespace | 61 ms | ⟨R:qfullns⟩ |
| Walk every page of the last hour (82 × 500) | 607 ms | ⟨R:walk⟩ |
| 8 concurrent timeline requests, 1h | 431 ms | ⟨R:conc⟩ |
| Event query, full 7 days* | 15.0 s | ⟨R:evfull⟩ |

\* Before this work, every timeline request also ran this query. It is now only used
when someone asks for the `Event` kind explicitly.

### 30 days: `xlarge` profile, 5M churn events, 10% core/v1 Events, `retentionDays: 30`

1000 nodes, 400 namespaces. 5.07M events touching 970k distinct objects; most are pods
replaced over the month, plus 500k Event objects. 17.4 GB of object JSON.

⟨THIRTY⟩

## Sizing

Live heap is roughly:

```
heap ≈ 150 MB
     + 0.9 KB × objects that existed during the retention window
     + 0.1 KB × resource changes in the retention window
     + 0.45 KB × core/v1 Events in the last 48h
```

- **Objects** includes deleted ones still inside the window, such as replaced pods.
  Labels make up a third of the per-object cost.
- **Changes** count after duplicate removal; `spectre_embedded_ingest_events_total`
  counts exactly these.
- **The Event term** is bounded by `K8sEventSummaryHorizon` and by 64 summaries per
  involved object.

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

- **Response size of long windows.**
  - A timeline page carries every version of each resource in the window, with its full
    object, because the UI diffs neighbouring versions.
  - Objects that change often add up: a 30-day page of 100 Nodes held 61,821 versions,
    about 300 MB of JSON, and took ~1 s.
  - The planned fix is to load `resourceData` per version on demand: send segments
    without payloads and fetch a version when the detail panel opens. That needs an API
    and UI change.
  - Until then, the UI's long presets work best with namespace or kind filters.
- **Node heartbeats dominate history.** Kubelets rewrite Node status (heartbeat
  timestamps) every few minutes even when nothing changes: 1000 nodes produce
  ~290k versions a day. Not storing heartbeat-only status changes would cut memory and
  response size for Nodes substantially. It is lossy for the detail view, so it is
  left as a follow-up decision.
- **Explicit Event-kind queries** (`kind=Event` in the timeline) scan Event metadata for
  the whole window: 0.4–0.5 s for 7 days and ~7 s for 30 days with 500k Events. Default
  timeline requests no longer run this query.
- **Older Event summaries are read from disk.**
  - The first query reaching back more than 48h loads the associated-UID index of each
    segment in its window.
  - Those indexes stay cached, and their memory grows with the number of Events in them.
- **Checkpoint CPU grows with history.** A checkpoint encodes the whole projection
  without blocking ingest, but still costs CPU: ⟨CKPT30⟩ each at the 30-day scale, every
  50k events.
- **Bulk-load stalls.** While loading at thousands of events per second, the longest
  batch stall at 30 days was ⟨STALL30⟩: retention and compaction commits hold the
  engine lock briefly. Real ingest rates are far below the measured ceiling.
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
