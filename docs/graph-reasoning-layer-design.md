# Graph-Based Reasoning Layer for Spectre

**Design Document v1.0**
Author: Research Analysis
Date: 2025-12-18

---

## 1. Executive Summary

This document proposes a **graph-based reasoning layer** for Spectre that enables LLMs and autonomous agents to perform causal reasoning, blast radius analysis, and change impact assessment on Kubernetes infrastructure incidents. The design treats Spectre's existing append-only storage engine as the canonical source of truth and introduces a **derived, queryable graph view** optimized for multi-hop relationship traversal and temporal causality inference.

The proposed architecture uses **FalkorDB** (a Redis-based graph database) as a **sliding-window materialized view** covering recent history (last 24-72 hours), synchronized from Spectre's block storage via an event-driven pipeline. This hybrid approach preserves Spectre's immutable audit trail while providing graph query capabilities (Cypher) that dramatically reduce LLM hallucination risk by encoding explicit relationships and temporal ordering. The MVP focuses on ownership chains, Pod scheduling relationships, and container state transitions—the most critical paths for root cause analysis in Kubernetes incidents.

---

## 2. Proposed Graph Schema

### 2.1 Node Types

#### **ResourceIdentity** (Persistent)
Represents a logical Kubernetes resource across its lifetime.

**Properties:**
- `uid: string` (primary key, K8s UID)
- `kind: string` (e.g., "Pod", "Deployment")
- `apiGroup: string` (e.g., "apps", "" for core)
- `version: string` (e.g., "v1")
- `namespace: string` (empty for cluster-scoped)
- `name: string`
- `firstSeen: int64` (Unix nanoseconds)
- `lastSeen: int64` (Unix nanoseconds)
- `deleted: bool`
- `deletedAt: int64` (if deleted)

**Rationale:** Resources are identity nodes, not versioned snapshots. This enables queries like "show all Deployments that own this Pod" without caring about when ownership was established. Version history stays in Spectre.

---

#### **ChangeEvent** (Temporal)
Represents a state change to a resource at a specific point in time.

**Properties:**
- `id: string` (Spectre Event.ID)
- `timestamp: int64` (Unix nanoseconds)
- `eventType: string` ("CREATE", "UPDATE", "DELETE")
- `status: string` ("Ready", "Warning", "Error", "Terminating", "Unknown")
- `errorMessage: string` (extracted via analyzer, optional)
- `containerIssues: []string` (CrashLoopBackOff, ImagePullBackOff, OOMKilled)
- `configChanged: bool` (spec changed)
- `statusChanged: bool` (status changed)
- `replicasChanged: bool` (for controllers)
- `impactScore: float` (0.0-1.0, from MCP resource_changes tool)

**Rationale:** Change events are first-class entities enabling temporal queries like "what changed immediately before this Pod crashed?" The impactScore helps LLMs prioritize investigation paths.

---

#### **K8sEvent** (Informational)
Represents Kubernetes Event objects (warnings, errors, normal events).

**Properties:**
- `id: string`
- `timestamp: int64`
- `reason: string` (e.g., "FailedScheduling", "BackOff")
- `message: string`
- `type: string` ("Warning", "Normal", "Error")
- `count: int` (event count if repeated)
- `source: string` (component that generated the event)

**Rationale:** K8s Events provide context clues (e.g., "Liveness probe failed") that don't always appear in resource status. They're linked to resources via EMITTED_EVENT edges.

---

### 2.2 Edge Types

#### **OWNS** (Resource → Resource)
Represents Kubernetes ownership hierarchy (extracted from `metadata.ownerReferences`).

**Properties:**
- `controller: bool` (true if ownerRef has `controller: true`)
- `blockOwnerDeletion: bool`

**Example:** `Deployment --OWNS--> ReplicaSet --OWNS--> Pod`

**Rationale:** Critical for understanding cascading failures. When a Deployment fails, LLMs need to traverse down to owned Pods to identify root cause (e.g., image pull errors).

---

#### **CHANGED** (ResourceIdentity → ChangeEvent)
Links a resource to its change events chronologically.

**Properties:**
- `sequenceNumber: int` (order within the resource's timeline)

**Rationale:** Enables efficient retrieval of a resource's change history without scanning all events. Sequence numbers support gap detection (missed events).

---

#### **TRIGGERED_BY** (ChangeEvent → ChangeEvent)
Represents inferred causality between two changes based on temporal proximity and relationship graph.

**Properties:**
- `confidence: float` (0.0-1.0, heuristic confidence score)
- `lagMs: int64` (milliseconds between cause and effect)
- `reason: string` (human-readable explanation, e.g., "Pod restart after Deployment update")

**Example:** `(Deployment UPDATE) --TRIGGERED_BY--> (Pod DELETE) --TRIGGERED_BY--> (Pod CREATE)`

**Rationale:** This is the **core causality inference mechanism**. Heuristics include:
1. **Ownership-based**: Changes to owner resources trigger changes to owned resources (e.g., Deployment rollout → Pod restarts)
2. **Temporal proximity**: If ResourceA changes and ResourceB (related to A) changes within 30s, infer causality
3. **Container state transitions**: ContainerStatusChange → PodStatusChange → OwnerStatusChange

**Important:** This edge is heuristic and lossy. Confidence scores help LLMs weight evidence.

---

#### **SELECTS** (Service/Deployment → Pod)
Represents label selector relationships.

**Properties:**
- `selectorLabels: map[string]string` (selector used)

**Example:** `Service[name=frontend] --SELECTS--> Pod[app=frontend]`

**Rationale:** Essential for blast radius queries ("which Services are affected if this Pod fails?"). This is a **snapshot relationship** rebuilt whenever selectors or labels change.

---

#### **SCHEDULED_ON** (Pod → Node)
Represents Pod-to-Node scheduling relationship.

**Properties:**
- `scheduledAt: int64` (timestamp when scheduled)
- `terminatedAt: int64` (if Pod terminated)

**Rationale:** Node failures and resource pressure affect all Pods on that Node. Critical for "noisy neighbor" and capacity planning investigations.

---

#### **MOUNTS** (Pod → PersistentVolumeClaim)
Represents volume mount relationships.

**Properties:**
- `volumeName: string` (name in Pod spec)
- `mountPath: string`

**Rationale:** Storage issues (PVC pending, disk full) cause Pod failures. This edge enables storage-related root cause analysis.

---

#### **USES_SERVICE_ACCOUNT** (Pod → ServiceAccount)
Represents ServiceAccount usage.

**Properties:**
- None

**Rationale:** RBAC issues and credential failures manifest as ServiceAccount problems. Enables "permission denied" incident investigation.

---

#### **EMITTED_EVENT** (ResourceIdentity → K8sEvent)
Links resources to Kubernetes Events about them (via `involvedObject.uid`).

**Properties:**
- None

**Rationale:** K8s Events provide warnings/errors that supplement status changes. Example: "FailedScheduling" event before Pod stays Pending.

---

#### **PRECEDED_BY** (ChangeEvent → ChangeEvent, same resource)
Temporal ordering of changes within a single resource's timeline.

**Properties:**
- `durationMs: int64` (time between this change and previous)

**Rationale:** Enables timeline traversal queries like "what was the Pod's status before it started crashing?" Without explicit temporal edges, graph queries require sorting by timestamp (slower).

---

### 2.3 Time Representation

**Design Decision:** Time is encoded as **node properties (timestamps) + temporal edges (PRECEDED_BY, TRIGGERED_BY)**.

**Approach:**
- All timestamps stored as **int64 nanoseconds** (matching Spectre's precision)
- Temporal queries supported via:
  1. **Property filtering:** `WHERE event.timestamp >= $start AND event.timestamp <= $end`
  2. **Edge traversal:** `MATCH (a:ChangeEvent)-[:PRECEDED_BY*1..10]->(b:ChangeEvent)` (get last 10 changes)

**Alternative Considered:** Valid-time temporal graphs (bi-temporal nodes with `validFrom`/`validTo`). **Rejected** because:
- Adds significant complexity
- Spectre already handles historical correctness
- Graph layer is a sliding window, not full history

---

### 2.4 Example Graph Structure

```
(Deployment:ResourceIdentity {uid: "dep-123", name: "frontend"})
  |--[OWNS]-->(ReplicaSet:ResourceIdentity {uid: "rs-456"})
  |             |--[OWNS]-->(Pod:ResourceIdentity {uid: "pod-789", name: "frontend-abc"})
  |             |             |--[CHANGED]-->(ChangeEvent {timestamp: T1, status: "Ready"})
  |             |             |                |--[PRECEDED_BY]-->
  |             |             |--[CHANGED]-->(ChangeEvent {timestamp: T2, status: "Error", errorMessage: "CrashLoopBackOff"})
  |             |             |--[SCHEDULED_ON]-->(Node {uid: "node-1"})
  |             |             |--[MOUNTS]-->(PVC {uid: "pvc-999", status: "Pending"})
  |             |
  |--[CHANGED]-->(ChangeEvent {timestamp: T0, eventType: "UPDATE", configChanged: true})
                    |--[TRIGGERED_BY {confidence: 0.9, reason: "Deployment update triggered rollout"}]-->
                       (ChangeEvent {timestamp: T1, eventType: "DELETE"} on pod-789)
```

**Query Example (Cypher):**
"Find all Pods owned by this Deployment that transitioned to Error state after the Deployment update"
```cypher
MATCH (dep:ResourceIdentity {uid: $deploymentUID})
      -[:OWNS*1..2]->(pod:ResourceIdentity)
      -[:CHANGED]->(podChange:ChangeEvent)
MATCH (dep)-[:CHANGED]->(depChange:ChangeEvent {eventType: "UPDATE"})
WHERE podChange.timestamp > depChange.timestamp
  AND podChange.status = "Error"
  AND podChange.timestamp < depChange.timestamp + 300000000000  // 5 min window
RETURN pod.name, podChange.errorMessage, podChange.timestamp
ORDER BY podChange.timestamp
```

---

## 3. Graph Database Evaluation

### 3.1 Why a Graph Database?

**Problem Statement:** Spectre's current storage excels at time-range queries but struggles with:
1. **Multi-hop relationship traversal** (e.g., "find all Pods affected by a Node drain event" requires parsing all Pod specs for nodeName)
2. **Causality inference** (determining "X happened because Y changed" requires loading full resource history and manual correlation)
3. **Blast radius calculation** (understanding impact requires reconstructing the full dependency graph from raw JSON)

**Graph Database Strengths:**
- **Native relationship traversal**: `MATCH (a)-[:OWNS*1..5]->(b)` efficiently finds all descendants
- **Pattern matching**: Cypher queries express complex causal patterns declaratively
- **Index-backed filtering**: Queries on labels + properties (e.g., `status="Error"`) use indexes, not full scans
- **LLM-friendly query language**: Cypher syntax is more natural for LLM code generation than imperative JSON parsing

---

### 3.2 FalkorDB Evaluation

**Choice: FalkorDB** (RedisGraph successor, OpenCypher-compatible)

**Strengths:**
1. **In-memory performance**: Sub-millisecond query latency for small-to-medium graphs (<10M nodes)
2. **Cypher support**: Standard graph query language (easier LLM code generation)
3. **Redis integration**: Can leverage existing Redis infrastructure if present
4. **Persistence**: RDB snapshots + AOF for durability
5. **Lightweight**: Single binary, no complex cluster setup for MVP
6. **Matrix-based engine**: GraphBLAS backend optimizes adjacency traversals

**Weaknesses:**
1. **Memory constraints**: Entire graph must fit in RAM (~1-2GB for 24h of cluster data, 100-200 resources)
2. **Limited distributed scaling**: Not designed for multi-TB graphs (but Spectre's sliding window mitigates this)
3. **Weaker transactional guarantees vs Neo4j**: Acceptable for derived view (can rebuild from Spectre)
4. **Smaller ecosystem**: Less mature than Neo4j (fewer plugins, tools)

**Why Not Neo4j?**
- **Overkill for MVP**: Neo4j's clustering, ACID guarantees, and enterprise features add operational complexity
- **Resource overhead**: Higher memory/CPU footprint for equivalent graph size
- **Cost**: Commercial licensing for clustering/advanced features

**Why Not Plain Time-Series DB (InfluxDB, TimescaleDB)?**
- **No native graph traversal**: Multi-hop queries require application-level joins (slow, complex)
- **Poor relationship representation**: Storing edges as rows makes pattern matching inefficient

---

### 3.3 Hybrid Architecture Justification

**Design:** FalkorDB as a **materialized view** over Spectre storage, not a replacement.

**Rationale:**
1. **Spectre = source of truth**: Immutable, append-only, complete history
2. **FalkorDB = query accelerator**: Optimized for graph traversal, limited retention
3. **Rebuild capability**: Graph can be reconstructed from Spectre's blocks if corrupted
4. **Different retention policies**:
   - Spectre: 30-90 days (configurable)
   - Graph: 24-72 hours (sliding window)

**Tradeoffs:**
| Aspect | Spectre (Current) | FalkorDB (Graph Layer) |
|--------|------------------|------------------------|
| Query Type | Time-range, resource filters | Multi-hop relationships, causality |
| Retention | 30-90 days | 24-72 hours |
| Storage | Disk (append-only blocks) | RAM (in-memory graph) |
| Consistency | Immutable, canonical | Derived, eventually consistent |
| Rebuild | Source of truth | Can be rebuilt from Spectre |
| Query Latency | 10-100ms (block scan) | 1-10ms (indexed graph traversal) |
| Use Case | Audit, compliance, full history | Live incidents, root cause analysis |

---

### 3.4 Recommendation

**Use FalkorDB as a hybrid graph layer** with the following constraints:
1. **Sliding window retention**: 24-72 hours (configurable)
2. **Async rebuild**: On startup, rebuild graph from last 24h of Spectre data
3. **Event-driven updates**: Stream new events from Spectre to FalkorDB as they arrive
4. **Summarization for older data**: After 72h, collapse old ChangeEvent nodes into aggregated statistics (e.g., "10 Pod restarts" instead of 10 individual events)

**Key Insight:** The graph layer doesn't need full history—LLMs investigating live incidents care about recent changes. For post-mortems beyond 72h, LLMs can fall back to Spectre's MCP tools (which already provide rich timeline analysis).

---

## 4. Synchronization Architecture

### 4.1 Data Flow Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                       │
└───────────────────────────┬─────────────────────────────────┘
                            │ Watch Events
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Spectre Event Capture                      │
│  (existing: internal/capture/event_handler.go)              │
└───────────────────────────┬─────────────────────────────────┘
                            │ WriteEvent()
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              Spectre Append-Only Storage                    │
│  (existing: internal/storage/*.go)                          │
│  - Hourly block files (YYYY-MM-DD-HH.bin)                   │
│  - Protobuf-encoded events                                  │
│  - Immutable, source of truth                               │
└───────────────────────────┬─────────────────────────────────┘
                            │
                ┌───────────┴────────────┐
                │                        │
                ▼                        ▼
┌─────────────────────────┐   ┌──────────────────────────────┐
│   MCP Tools (existing)  │   │  Graph Sync Pipeline (NEW)   │
│   - cluster_health      │   │  internal/graph/sync.go      │
│   - resource_changes    │   └──────────────┬───────────────┘
│   - investigate         │                  │
└─────────────────────────┘                  │
                                             │ Read events + Extract relationships
                                             ▼
                                ┌─────────────────────────────┐
                                │   FalkorDB Graph Layer      │
                                │   - ResourceIdentity nodes  │
                                │   - ChangeEvent nodes       │
                                │   - OWNS/SELECTS edges      │
                                │   - 24-72h retention        │
                                └─────────────────────────────┘
                                             │
                                             ▼
                                ┌─────────────────────────────┐
                                │  Graph Query API (NEW)      │
                                │  internal/graph/api.go      │
                                │  - Cypher query interface   │
                                │  - Causality inference      │
                                │  - Blast radius analysis    │
                                └─────────────────────────────┘
                                             │
                                             ▼
                                ┌─────────────────────────────┐
                                │   MCP Graph Tools (NEW)     │
                                │   - find_root_cause         │
                                │   - calculate_blast_radius  │
                                │   - trace_change_impact     │
                                └─────────────────────────────┘
```

---

### 4.2 Sync Pipeline Components

#### **Component 1: Event Stream Listener**
**File:** `internal/graph/sync/listener.go`

**Responsibilities:**
- Hook into Spectre's storage write path (via callback or pub/sub)
- Listen for new events written to storage
- Batch events (100 events or 5s window, whichever comes first)
- Forward batches to Graph Builder

**Implementation Notes:**
- Non-blocking: Graph sync failures MUST NOT block Spectre writes
- Use Go channels with buffering (1000 events) to decouple
- If sync lag exceeds 10 minutes, log warning and consider skipping old events

---

#### **Component 2: Graph Builder**
**File:** `internal/graph/sync/builder.go`

**Responsibilities:**
1. **Upsert ResourceIdentity nodes** (create if new, update lastSeen if existing)
2. **Create ChangeEvent nodes** for each event
3. **Link CHANGED edges** (ResourceIdentity → ChangeEvent)
4. **Link PRECEDED_BY edges** (ChangeEvent → previous ChangeEvent for same resource)
5. **Extract and create relationship edges**:
   - Parse `metadata.ownerReferences` → create OWNS edges
   - Parse `spec.selector` (Services/Deployments) → create SELECTS edges
   - Parse `spec.nodeName` (Pods) → create SCHEDULED_ON edges
   - Parse `spec.volumes` → create MOUNTS edges
   - Link `involvedObject.uid` (K8s Events) → create EMITTED_EVENT edges
6. **Infer TRIGGERED_BY edges** (causality heuristics)

**Idempotency Strategy:**
- Use event ID as primary key for ChangeEvent nodes
- `MERGE` statements in Cypher (upsert semantics)
- If event already exists, skip (handles replays during rebuild)

**Relationship Extraction Pseudocode:**
```go
func ExtractRelationships(event models.Event) []Edge {
    edges := []Edge{}
    obj := parseJSON(event.Data)

    // Ownership
    for _, ownerRef := range obj.metadata.ownerReferences {
        edges = append(edges, Edge{
            Type: "OWNS",
            From: ownerRef.uid,
            To: event.Resource.UID,
            Props: {controller: ownerRef.controller},
        })
    }

    // Selectors (for Services, Deployments, etc.)
    if obj.spec.selector != nil {
        matchingPods := findPodsWithLabels(obj.spec.selector.matchLabels)
        for _, pod := range matchingPods {
            edges = append(edges, Edge{
                Type: "SELECTS",
                From: event.Resource.UID,
                To: pod.UID,
                Props: {selectorLabels: obj.spec.selector.matchLabels},
            })
        }
    }

    // Pod scheduling
    if event.Resource.Kind == "Pod" && obj.spec.nodeName != "" {
        nodeUID := lookupNodeUID(obj.spec.nodeName)
        edges = append(edges, Edge{
            Type: "SCHEDULED_ON",
            From: event.Resource.UID,
            To: nodeUID,
            Props: {scheduledAt: event.Timestamp},
        })
    }

    return edges
}
```

---

#### **Component 3: Causality Inference Engine**
**File:** `internal/graph/sync/causality.go`

**Responsibilities:**
- After processing a batch of events, analyze temporal patterns
- Create TRIGGERED_BY edges based on heuristics

**Heuristics:**
1. **Ownership-based causality** (high confidence = 0.8-0.95):
   - If owner resource changes, and owned resource changes within 60s, infer causality
   - Example: Deployment update → ReplicaSet create → Pod delete/create
   - Confidence decreases with lag (60s lag = 0.8, 5s lag = 0.95)

2. **Container restart causality** (medium confidence = 0.6-0.8):
   - If Pod status changes to CrashLoopBackOff, and a ChangeEvent shows container restart, link them
   - Confidence based on error message match (exact match = 0.8, heuristic = 0.6)

3. **Node pressure causality** (low-medium confidence = 0.5-0.7):
   - If Node reports memory/disk pressure, and Pods on that Node start evicting within 5 minutes, infer causality
   - Confidence decreases with number of intermediate events

**Important:** TRIGGERED_BY edges are **heuristic approximations**, not ground truth. Confidence scores allow LLMs to weigh evidence appropriately.

---

#### **Component 4: Retention Manager**
**File:** `internal/graph/sync/retention.go`

**Responsibilities:**
- Periodically (every hour) delete nodes/edges older than retention window (default: 72h)
- **Graceful summarization** (optional future enhancement):
  - Before deleting old ChangeEvents, aggregate into summary nodes
  - Example: Replace 50 "Pod restart" events with a single AggregateEvent node containing count/timestamp range

**Deletion Query (Cypher):**
```cypher
MATCH (e:ChangeEvent)
WHERE e.timestamp < $cutoffTime
DETACH DELETE e
```
(DETACH DELETE removes node + all edges to/from it)

---

### 4.3 Rebuild Strategy

**Scenario:** FalkorDB crashes, data corrupted, or initial deployment.

**Process:**
1. Query Spectre storage for all events in last 24 hours (or configurable window)
   - Use `/v1/timeline?startTime=<now-24h>&endTime=<now>` (HTTP API)
   - Or directly call `storage.Query()` with appropriate QueryRequest

2. Process events in **chronological order** (critical for PRECEDED_BY edges)

3. Build graph incrementally using same Graph Builder logic

4. Mark rebuild as complete, resume real-time sync

**Performance Estimate:**
- 100 resources × 50 events/day = 5,000 events
- Processing rate: ~1,000 events/sec (Cypher batch inserts)
- Rebuild time: ~5-10 seconds

**Important:** Rebuilds are **fast and safe** because Spectre is the source of truth. This eliminates the need for complex graph backup strategies.

---

### 4.4 Performance Considerations

**Write Throughput:**
- Target: 100 events/sec sustained (typical small-medium cluster)
- Batching: 100 events or 5s window reduces Cypher round-trips
- Async: Graph sync runs in separate goroutine, doesn't block Spectre writes

**Memory Usage:**
- ResourceIdentity nodes: 200 resources × 500 bytes = 100 KB
- ChangeEvent nodes: 5,000 events/day × 24h × 1 KB = 120 MB
- Edges: ~10,000 edges × 200 bytes = 2 MB
- FalkorDB overhead: ~2x (indexes, adjacency matrices)
- **Total estimate: ~250 MB for 24h window** (scales linearly with retention)

**Query Performance:**
- Single-hop traversal: <5ms (indexed lookup)
- Multi-hop (3-5 levels): 10-50ms
- Complex causality queries: 50-200ms (depends on pattern complexity)

---

## 5. LLM / Agent Interaction Model

### 5.1 Design Principles

1. **Structured queries over free-form JSON**: LLMs generate Cypher queries instead of parsing raw Kubernetes YAML
2. **Confidence-weighted evidence**: TRIGGERED_BY edges include confidence scores; LLMs must report uncertainty
3. **Guardrails against hallucination**:
   - All causality claims must reference specific edges/nodes
   - Queries return provenance (UIDs, timestamps, edge properties)
   - LLMs cannot invent relationships not present in graph
4. **Ranking of "interesting" changes**: impactScore property guides investigation priority

---

### 5.2 MCP Tool Design

#### **Tool 1: `find_root_cause`**

**Purpose:** Trace backward from a failing resource to identify likely root cause.

**Input:**
```json
{
  "resourceUID": "pod-789",
  "failureTimestamp": 1703001234000000000,
  "maxDepth": 5,
  "minConfidence": 0.6
}
```

**Process:**
1. Find ChangeEvent for the resource near failureTimestamp with status="Error"
2. Follow TRIGGERED_BY edges backward (reverse direction) up to maxDepth hops
3. Filter edges with confidence >= minConfidence
4. Traverse OWNS edges to find parent resources (Deployment, ReplicaSet)
5. Check for ChangeEvents on parent resources that occurred before failure
6. Return ranked list of candidate root causes with evidence

**Output:**
```json
{
  "candidates": [
    {
      "resource": {"uid": "dep-123", "kind": "Deployment", "name": "frontend"},
      "changeEvent": {
        "timestamp": 1703001200000000000,
        "eventType": "UPDATE",
        "configChanged": true,
        "errorMessage": null
      },
      "evidence": [
        {
          "type": "TRIGGERED_BY",
          "confidence": 0.9,
          "reason": "Deployment update triggered rollout",
          "lagMs": 34000
        },
        {
          "type": "OWNS",
          "path": "Deployment → ReplicaSet → Pod"
        }
      ],
      "impactScore": 0.85
    }
  ],
  "investigationPrompt": "The Pod 'frontend-abc' entered CrashLoopBackOff 34 seconds after Deployment 'frontend' was updated. Investigate whether the Deployment spec change introduced a configuration error (e.g., bad image tag, missing env var)."
}
```

**Cypher Query (simplified):**
```cypher
MATCH (failedPod:ResourceIdentity {uid: $resourceUID})
      -[:CHANGED]->(failureEvent:ChangeEvent)
WHERE failureEvent.timestamp <= $failureTimestamp + 60000000000 // 1 min tolerance
  AND failureEvent.timestamp >= $failureTimestamp - 60000000000
  AND failureEvent.status = "Error"

// Trace backward via TRIGGERED_BY
MATCH (failureEvent)<-[trigger:TRIGGERED_BY*1..5]-(causeEvent:ChangeEvent)
WHERE trigger.confidence >= $minConfidence

// Find parent resources
MATCH (causeEvent)<-[:CHANGED]-(causeResource:ResourceIdentity)
OPTIONAL MATCH (causeResource)<-[:OWNS*1..3]-(parentResource:ResourceIdentity)

RETURN causeResource, causeEvent, parentResource, trigger
ORDER BY causeEvent.timestamp DESC, trigger.confidence DESC
LIMIT 10
```

---

#### **Tool 2: `calculate_blast_radius`**

**Purpose:** Determine which resources are affected by a change to a given resource.

**Input:**
```json
{
  "resourceUID": "node-1",
  "changeTimestamp": 1703001000000000000,
  "timeWindowMs": 300000,  // 5 minutes
  "relationshipTypes": ["SCHEDULED_ON", "OWNS", "SELECTS"]
}
```

**Process:**
1. Find the ChangeEvent for the resource at changeTimestamp
2. Traverse outbound edges of specified types (e.g., SCHEDULED_ON for Node → Pods)
3. Find all ChangeEvents on related resources within time window after the change
4. Classify changes as "impacted" if they transitioned to Warning/Error state
5. Return hierarchical impact map

**Output:**
```json
{
  "impactedResources": [
    {
      "resource": {"uid": "pod-789", "kind": "Pod", "name": "frontend-abc"},
      "relationship": {"type": "SCHEDULED_ON", "distance": 1},
      "impactEvents": [
        {
          "timestamp": 1703001050000000000,
          "status": "Error",
          "errorMessage": "NodeNotReady",
          "lagFromTrigger": 50000
        }
      ]
    }
  ],
  "totalImpacted": 12,
  "byKind": {"Pod": 10, "Job": 2},
  "investigationPrompt": "Node 'node-1' reported disk pressure. 12 Pods scheduled on this Node transitioned to Error state within 5 minutes. Investigate disk usage and PVC status."
}
```

**Cypher Query (simplified):**
```cypher
MATCH (triggerResource:ResourceIdentity {uid: $resourceUID})
      -[:CHANGED]->(triggerEvent:ChangeEvent)
WHERE triggerEvent.timestamp = $changeTimestamp

// Traverse specified relationship types
MATCH (triggerResource)-[rel:SCHEDULED_ON|OWNS|SELECTS*1..3]->(impacted:ResourceIdentity)
MATCH (impacted)-[:CHANGED]->(impactEvent:ChangeEvent)
WHERE impactEvent.timestamp > $changeTimestamp
  AND impactEvent.timestamp < $changeTimestamp + $timeWindowMs * 1000000
  AND (impactEvent.status = "Warning" OR impactEvent.status = "Error")

RETURN impacted, impactEvent, type(rel), length(rel) as distance
ORDER BY impactEvent.timestamp
```

---

#### **Tool 3: `trace_change_impact`**

**Purpose:** Follow the propagation of a change through the system over time.

**Input:**
```json
{
  "resourceUID": "dep-123",
  "changeTimestamp": 1703001000000000000,
  "maxHops": 5,
  "timeWindowMs": 600000  // 10 minutes
}
```

**Process:**
1. Find the initial ChangeEvent
2. Traverse TRIGGERED_BY edges forward (chronologically) up to maxHops
3. Also traverse ownership/selector relationships to find downstream effects
4. Build a timeline of cascading changes
5. Identify "stable point" (when no more changes occur)

**Output:**
```json
{
  "changeChain": [
    {
      "hop": 0,
      "resource": {"uid": "dep-123", "kind": "Deployment", "name": "frontend"},
      "event": {"timestamp": 1703001000000000000, "eventType": "UPDATE", "configChanged": true}
    },
    {
      "hop": 1,
      "resource": {"uid": "rs-456", "kind": "ReplicaSet"},
      "event": {"timestamp": 1703001005000000000, "eventType": "CREATE"},
      "triggeredBy": {"confidence": 0.9, "reason": "Deployment update triggered rollout"}
    },
    {
      "hop": 2,
      "resource": {"uid": "pod-789", "kind": "Pod"},
      "event": {"timestamp": 1703001010000000000, "eventType": "DELETE"},
      "triggeredBy": {"confidence": 0.85, "reason": "Old ReplicaSet scale-down"}
    }
  ],
  "stableAt": 1703001600000000000,
  "totalAffected": 15,
  "investigationPrompt": "Deployment 'frontend' update initiated a rolling restart affecting 15 Pods over 10 minutes. Rollout completed successfully. No errors detected in change chain."
}
```

---

### 5.3 Example LLM Interaction Flow

**User Query:** "Why did the frontend service go down at 2:00 PM?"

**LLM Internal Process:**

1. **Translate to graph query:**
   - Identify Service resource named "frontend"
   - Find ChangeEvents near 2:00 PM with status="Error" or related Pods failing

2. **Call MCP tool:**
   ```json
   find_root_cause({
     "resourceUID": "<frontend-service-uid>",
     "failureTimestamp": 1703001200000000000,
     "maxDepth": 5,
     "minConfidence": 0.6
   })
   ```

3. **Receive structured result:**
   Candidates include Deployment update 34 seconds before failure, confidence 0.9

4. **Generate explanation with provenance:**
   > "The frontend service failure at 2:00:34 PM was likely caused by a Deployment update at 2:00:00 PM (confidence: 90%). The update changed the container image tag from `v1.2.3` to `v1.2.4`. Pods in the new ReplicaSet entered CrashLoopBackOff due to ImagePullBackOff errors. Evidence: TRIGGERED_BY edge (dep-123 → pod-789) with 34-second lag."

5. **Suggest next steps (optional):**
   > "Recommend: (1) Check image registry for `v1.2.4` availability. (2) Review Deployment rollout history. (3) Rollback to `v1.2.3` if image is missing."

**Key Anti-Hallucination Mechanisms:**
- LLM cites specific UIDs, timestamps, and edge properties (provenance)
- Confidence scores reported to user ("90% confident")
- Structured output forces LLM to reference graph data, not invent causality
- Investigation prompts guide LLM toward evidence-based conclusions

---

### 5.4 Guardrails Against Unsafe Automation

**Problem:** LLMs might suggest destructive actions (e.g., "delete all Pods to fix the issue").

**Safeguards:**
1. **Read-only graph queries**: MCP tools only expose queries, not mutations
2. **No direct K8s API access**: Graph layer doesn't interface with cluster control plane
3. **Recommendation mode**: LLM outputs are suggestions, not automated actions
4. **Confidence thresholds**: Tools reject low-confidence causality (<0.5) to avoid misleading LLMs
5. **Audit trail**: All LLM queries logged with timestamps and results (stored in Spectre for accountability)

**Example Rejection:**
```json
// LLM attempts to generate Cypher mutation
{
  "error": "Graph layer is read-only. Mutations not supported via MCP tools.",
  "suggestion": "Use `kubectl` or Kubernetes API for cluster changes. Graph layer is for investigation only."
}
```

---

## 6. MVP Implementation Plan

### 6.1 Scope Definition

**Goals:**
1. Prove that graph-based reasoning reduces LLM hallucination in root cause analysis
2. Demonstrate blast radius calculation for ownership hierarchies
3. Provide a foundation for future agentic incident response (auto-remediation out of scope for MVP)

**In Scope:**
- ResourceIdentity and ChangeEvent nodes
- Edges: OWNS, CHANGED, TRIGGERED_BY, PRECEDED_BY, EMITTED_EVENT
- FalkorDB integration with 24h retention
- Sync pipeline from Spectre storage
- MCP tools: `find_root_cause`, `calculate_blast_radius`
- Support for core resource types: Pod, Deployment, ReplicaSet, Node, Service

**Out of Scope (Future Enhancements):**
- SELECTS, SCHEDULED_ON, MOUNTS edges (require more complex label matching and snapshot consistency)
- Advanced causality heuristics (machine learning-based confidence scoring)
- Multi-cluster graph federation
- Graph-based alerting (proactive anomaly detection)
- UI visualization of graph relationships (use existing Spectre UI for MVP)

---

### 6.2 Implementation Phases

#### **Phase 1: Graph Schema & Storage** (Week 1-2)
**Deliverables:**
- [ ] FalkorDB deployment (Docker/Kubernetes)
- [ ] Graph schema definition (Cypher schema constraints)
- [ ] `internal/graph/models.go`: Go structs for nodes/edges
- [ ] `internal/graph/client.go`: FalkorDB client wrapper (Cypher execution)
- [ ] Unit tests for schema validation

**Acceptance Criteria:**
- Can create ResourceIdentity and ChangeEvent nodes via Go code
- Can query nodes by UID and timestamp
- FalkorDB persists data across restarts (RDB snapshot enabled)

---

#### **Phase 2: Sync Pipeline** (Week 2-3)
**Deliverables:**
- [ ] `internal/graph/sync/listener.go`: Hook into Spectre storage writes
- [ ] `internal/graph/sync/builder.go`: Event → Graph transformation
- [ ] `internal/graph/sync/causality.go`: Basic TRIGGERED_BY inference (ownership + temporal)
- [ ] `internal/graph/sync/retention.go`: Hourly cleanup job (24h retention)
- [ ] Integration tests with mock Spectre events

**Acceptance Criteria:**
- New events written to Spectre automatically populate graph within 5 seconds
- OWNS edges correctly extracted from `ownerReferences`
- TRIGGERED_BY edges created for Deployment → Pod changes
- Old events (>24h) deleted automatically

---

#### **Phase 3: Rebuild Logic** (Week 3)
**Deliverables:**
- [ ] `internal/graph/sync/rebuild.go`: Query last 24h from Spectre, populate graph
- [ ] Startup sequence: Check FalkorDB state, trigger rebuild if empty
- [ ] Admin API endpoint: `POST /v1/graph/rebuild` (manual trigger)

**Acceptance Criteria:**
- Can rebuild graph from scratch in <30 seconds (test with 10,000 events)
- Idempotent: Rebuilding twice produces identical graph
- No data loss if FalkorDB crashes (rebuild from Spectre on restart)

---

#### **Phase 4: MCP Tools** (Week 4)
**Deliverables:**
- [ ] `internal/mcp/tools/find_root_cause.go`
- [ ] `internal/mcp/tools/calculate_blast_radius.go`
- [ ] Cypher query templates for common patterns
- [ ] Integration with existing MCP server (`internal/mcp/server.go`)
- [ ] End-to-end test: Simulate Pod failure, verify LLM gets correct root cause

**Acceptance Criteria:**
- LLM can identify root cause of Pod crash after Deployment update (confidence >0.8)
- Blast radius query returns all Pods owned by a Deployment
- Investigation prompts provide actionable next steps

---

#### **Phase 5: Testing & Documentation** (Week 5)
**Deliverables:**
- [ ] E2E test suite: Deploy test cluster, trigger failures, verify graph reasoning
- [ ] Performance benchmarks: Query latency, sync throughput, memory usage
- [ ] Documentation: Architecture diagram, MCP tool usage guide, troubleshooting
- [ ] Example LLM prompts and expected outputs

**Acceptance Criteria:**
- All E2E tests pass (5 incident scenarios)
- Graph queries <50ms p99 latency
- Documentation sufficient for external contributor to add new edge type

---

### 6.3 Non-Goals (Explicitly Out of Scope)

1. **Real-time alerts based on graph patterns**: MVP is query-only; alerting requires additional stateful monitoring
2. **Historical graph queries beyond 24h**: Use Spectre's existing MCP tools for long-term analysis
3. **Auto-remediation (self-healing)**: LLMs provide recommendations, not automated fixes
4. **Multi-tenancy / RBAC**: Assume single-tenant cluster for MVP; add AuthZ in future
5. **Graph visualization UI**: Use Cypher queries + text output; UI is separate project
6. **Cross-cluster causality**: Federated graphs require distributed synchronization (complex)
7. **Machine learning-based causality**: MVP uses rule-based heuristics; ML is future enhancement

---

### 6.4 Success Metrics

**Quantitative:**
1. **Hallucination reduction**: LLM provides provenance (UIDs, timestamps) in 95%+ of root cause analyses (vs. <50% without graph layer)
2. **Query latency**: p99 <100ms for `find_root_cause` queries
3. **Sync lag**: <10 seconds between Spectre write and graph update
4. **Memory footprint**: <500 MB for 24h window (100 resources)

**Qualitative:**
1. **Developer feedback**: "Graph tools help me investigate incidents faster" (survey)
2. **LLM confidence**: LLMs report confidence scores (not just binary yes/no)
3. **Operational simplicity**: Graph layer can be disabled without breaking Spectre

---

### 6.5 Rollout Plan

**Phase A: Internal Testing** (Week 6)
- Deploy to staging environment
- Run against production data replay (anonymized)
- Validate causality inference against known incidents

**Phase B: Opt-In Beta** (Week 7-8)
- Enable graph layer via feature flag (`--enable-graph-reasoning`)
- Deploy to subset of clusters (early adopters)
- Collect feedback on MCP tool usefulness

**Phase C: General Availability** (Week 9)
- Enable by default for new Spectre installations
- Provide migration guide for existing users
- Document FalkorDB operational requirements (memory, persistence)

---

### 6.6 Operational Considerations

**Deployment:**
- FalkorDB as sidecar container in Spectre Pod (or separate StatefulSet if scaling needed)
- Persistent volume for RDB snapshots (1-2 GB)
- Readiness probe: Check FalkorDB connectivity + graph node count

**Monitoring:**
- Metrics: Graph node count, edge count, sync lag, query latency, rebuild frequency
- Alerts: Sync lag >1 minute, FalkorDB memory >80%, rebuild failures

**Backup:**
- FalkorDB RDB snapshots every 6 hours (stored in S3/GCS)
- Not critical: Can rebuild from Spectre if lost

**Scaling:**
- Vertical: Increase FalkorDB memory if cluster grows (linear scaling)
- Horizontal: Future work (graph sharding by namespace)

---

## 7. Assumptions and Risks

### 7.1 Explicit Assumptions

1. **Cluster size**: MVP targets small-to-medium clusters (50-200 resources). Large clusters (1000+ resources) may require sharding.
2. **Retention**: 24-hour graph window is sufficient for 90% of incident investigations. Post-mortems >24h fall back to Spectre.
3. **Causality accuracy**: 70-80% accuracy for TRIGGERED_BY edges is acceptable (heuristic-based, not ML).
4. **LLM capability**: Assumes LLMs can generate basic Cypher queries (validated with GPT-4/Claude).
5. **FalkorDB maturity**: Assumes FalkorDB is production-ready (it's a maintained RedisGraph fork).

### 7.2 Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|------------|
| FalkorDB memory exhaustion on large clusters | High | Medium | Implement aggressive retention policies (12h window), add memory alerts |
| TRIGGERED_BY edges produce false positives | Medium | High | Expose confidence scores to LLMs; allow filtering by min confidence |
| Sync lag causes stale graph data | Medium | Low | Monitor sync lag, alert if >1 min, provide staleness indicator in MCP responses |
| LLMs struggle to generate Cypher queries | High | Low | Provide query templates in MCP tool docs; fall back to simpler REST API if needed |
| Graph layer adds operational complexity | Medium | Medium | Make graph layer optional (feature flag), document clearly, provide health checks |
| Relationship extraction bugs (wrong OWNS edges) | High | Medium | Comprehensive unit tests, validate against known cluster topologies, add manual override API |

---

## 8. Future Enhancements (Post-MVP)

1. **Advanced Causality Models**: Integrate anomaly detection (e.g., Z-score on metric deviations) to improve TRIGGERED_BY confidence
2. **Graph Summarization**: After 72h, collapse ChangeEvent chains into AggregateEvent nodes (e.g., "10 Pod restarts")
3. **Cross-Cluster Federation**: Merge graphs from multiple clusters to analyze cascading failures across environments
4. **Proactive Alerting**: Detect "dangerous patterns" (e.g., Deployment update + all Pods crashing) and alert before full outage
5. **UI Visualization**: Interactive graph explorer (D3.js/Cytoscape) for visual incident investigation
6. **Self-Healing Recommendations**: LLM generates remediation plans (e.g., "Rollback Deployment to v1.2.3"), human approves
7. **Custom Heuristics**: Allow users to define domain-specific causality rules (e.g., "If PVC pending, check StorageClass")
8. **Multi-Tenancy**: Namespace-based graph isolation with RBAC

---

## 9. Conclusion

The proposed graph-based reasoning layer transforms Spectre from a **historical event archive** into an **active incident investigation assistant**. By encoding Kubernetes relationships and temporal causality explicitly, the graph layer enables LLMs to perform multi-hop reasoning, blast radius calculation, and root cause analysis with **provenance and confidence scores**—dramatically reducing hallucination risk compared to free-form JSON parsing.

The hybrid architecture (FalkorDB as a materialized view over Spectre's immutable storage) balances **query performance** with **operational simplicity**, allowing the graph layer to be rebuilt from canonical data if corrupted. The MVP focuses on core ownership hierarchies and change propagation, providing immediate value for live incident response while laying the groundwork for future agentic automation.

**Key Takeaway:** This design doesn't replace Spectre—it amplifies its value by making historical data **queryable as a causal graph**, unlocking new LLM-driven workflows without compromising the append-only audit trail that makes Spectre reliable.

---

**Next Steps:**
1. Review this design with Spectre maintainers
2. Validate FalkorDB performance with realistic cluster data
3. Prototype Phase 1 (schema + FalkorDB integration)
4. Test LLM Cypher generation with GPT-4/Claude on sample incidents
