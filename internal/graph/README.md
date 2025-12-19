# Spectre Graph Reasoning Layer

This package provides a graph-based reasoning layer for Spectre, enabling LLMs and autonomous agents to perform causal reasoning, blast radius analysis, and change impact assessment.

## Architecture

The graph layer uses **FalkorDB** (a Redis-based graph database) as a sliding-window materialized view (24-72h) over Spectre's canonical append-only storage. This hybrid approach provides:

- **Fast multi-hop relationship traversal** via Cypher queries
- **Causal inference** with confidence-scored TRIGGERED_BY edges
- **Rebuild capability** from Spectre's immutable storage
- **Low memory footprint** (~250 MB for 24h window)

## Components

### Models (`models.go`)
Defines graph schema:
- **Nodes**: ResourceIdentity, ChangeEvent, K8sEvent
- **Edges**: OWNS, TRIGGERED_BY, CHANGED, PRECEDED_BY, etc.

### Client (`client.go`)
FalkorDB client wrapper providing:
- Connection management
- Query execution
- Node/edge creation
- Graph statistics

### Schema (`schema.go`)
Query builders for common operations:
- `UpsertResourceIdentityQuery` - Idempotent resource upsert
- `CreateChangeEventQuery` - Event insertion
- `FindRootCauseQuery` - Trace causality backward
- `CalculateBlastRadiusQuery` - Find affected resources

## Quick Start

### 1. Start FalkorDB

```bash
docker-compose -f docker-compose.graph.yml up -d
```

This starts:
- FalkorDB on port 6379
- RedisInsight on port 5540 (with `--profile dev`)

### 2. Initialize Client

```go
import "github.com/moolen/spectre/internal/graph"

// Create client
config := graph.DefaultClientConfig()
client := graph.NewClient(config)

// Connect
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()

// Initialize schema
schema := graph.NewSchema(client)
if err := schema.Initialize(ctx); err != nil {
    log.Fatal(err)
}
```

### 3. Create Nodes and Edges

```go
// Create a ResourceIdentity node
resource := graph.ResourceIdentity{
    UID:       "pod-123",
    Kind:      "Pod",
    APIGroup:  "",
    Version:   "v1",
    Namespace: "default",
    Name:      "frontend-abc",
    FirstSeen: time.Now().UnixNano(),
    LastSeen:  time.Now().UnixNano(),
}

query := graph.UpsertResourceIdentityQuery(resource)
_, err := client.ExecuteQuery(ctx, query)

// Create a ChangeEvent
event := graph.ChangeEvent{
    ID:            "event-456",
    Timestamp:     time.Now().UnixNano(),
    EventType:     "UPDATE",
    Status:        "Error",
    ErrorMessage:  "CrashLoopBackOff",
    ImpactScore:   0.85,
}

query = graph.CreateChangeEventQuery(event)
_, err = client.ExecuteQuery(ctx, query)

// Link resource to event
query = graph.CreateChangedEdgeQuery("pod-123", "event-456", 1)
_, err = client.ExecuteQuery(ctx, query)
```

### 4. Query the Graph

```go
// Find root cause of a failure
query := graph.FindRootCauseQuery(
    "pod-123",                  // failing resource UID
    time.Now().UnixNano(),      // failure timestamp
    5,                          // max depth
    0.6,                        // min confidence
)

result, err := client.ExecuteQuery(ctx, query)
// Process result...
```

## Graph Schema

### Node Types

**ResourceIdentity** - Persistent K8s resource
```
{
  uid: string (K8s UID)
  kind: string
  apiGroup: string
  version: string
  namespace: string
  name: string
  firstSeen: int64 (nanoseconds)
  lastSeen: int64
  deleted: bool
  deletedAt: int64
}
```

**ChangeEvent** - State change at a point in time
```
{
  id: string (Spectre Event.ID)
  timestamp: int64 (nanoseconds)
  eventType: string (CREATE|UPDATE|DELETE)
  status: string (Ready|Warning|Error|Terminating|Unknown)
  errorMessage: string
  containerIssues: []string
  configChanged: bool
  statusChanged: bool
  replicasChanged: bool
  impactScore: float (0.0-1.0)
}
```

**K8sEvent** - Kubernetes Event object
```
{
  id: string
  timestamp: int64
  reason: string
  message: string
  type: string (Warning|Normal|Error)
  count: int
  source: string
}
```

### Edge Types

- **OWNS**: Ownership hierarchy (Deployment → ReplicaSet → Pod)
- **CHANGED**: Resource → ChangeEvent linkage
- **TRIGGERED_BY**: Causal inference with confidence scores
- **PRECEDED_BY**: Temporal ordering within resource timeline
- **SELECTS**: Label selector relationships
- **SCHEDULED_ON**: Pod → Node scheduling
- **MOUNTS**: Pod → PVC volume relationships
- **USES_SERVICE_ACCOUNT**: Pod → ServiceAccount
- **EMITTED_EVENT**: Resource → K8sEvent
- **REFERENCES_SPEC**: Explicit spec references (e.g., HelmRelease → Secret)
- **MANAGES**: Lifecycle management (inferred with confidence)
- **ANNOTATES**: Label/annotation-based linkage
- **CREATES_OBSERVED**: Observed creation correlation

#### Custom Resource Edges (with Confidence Scoring)

Custom Resource edges (REFERENCES_SPEC, MANAGES, ANNOTATES, CREATES_OBSERVED) include confidence scores and evidence tracking:

```go
type ManagesEdge struct {
    Confidence      float64         // 0.0-1.0 confidence score
    Evidence        []EvidenceItem  // Evidence supporting this relationship
    FirstObserved   int64          // When first detected
    LastValidated   int64          // Last validation timestamp
    ValidationState ValidationState // valid, stale, invalid, pending
}
```

Evidence types:
- **label**: Label match (e.g., `helm.toolkit.fluxcd.io/name=frontend`)
- **annotation**: Annotation match
- **temporal**: Temporal proximity (created shortly after reconcile)
- **namespace**: Same namespace
- **ownership**: OwnerReference present
- **reconcile**: Reconcile event correlation

Example query:
```cypher
// Find all resources managed by a HelmRelease with >70% confidence
MATCH (hr:ResourceIdentity {name: "frontend"})-[m:MANAGES]->(managed)
WHERE m.confidence >= 0.7
RETURN managed.kind, managed.name, m.confidence, m.evidence
```

## Development

### Running Tests

```bash
# Unit tests (no FalkorDB required)
go test ./internal/graph -v

# Integration tests (requires FalkorDB)
docker-compose -f docker-compose.graph.yml up -d
go test ./internal/graph -v -tags=integration
```

### Adding Dependencies

The graph layer requires the Redis client:

```bash
go get github.com/redis/go-redis/v9
go mod tidy
```

### Viewing Graph Data

Use RedisInsight for visualization:

```bash
docker-compose -f docker-compose.graph.yml --profile dev up -d
# Open http://localhost:5540
```

Then connect to `localhost:6379` and query:

```cypher
GRAPH.QUERY spectre "MATCH (n) RETURN n LIMIT 50"
```

## Configuration

Environment variables:
- `GRAPH_ENABLED`: Enable graph layer (default: false)
- `GRAPH_HOST`: FalkorDB host (default: localhost)
- `GRAPH_PORT`: FalkorDB port (default: 6379)
- `GRAPH_PASSWORD`: FalkorDB password (optional)
- `GRAPH_NAME`: Graph database name (default: spectre)
- `GRAPH_RETENTION_HOURS`: Retention window in hours (default: 24)

## Custom Resource Extractors

The graph layer includes a pluggable extractor framework for modeling Custom Resource relationships.

### Built-in Extractors

**Flux HelmRelease Extractor**: Models Flux HelmRelease relationships
- Extracts `REFERENCES_SPEC` edges from `valuesFrom`, `sourceRef`, `kubeConfig.secretRef`
- Infers `MANAGES` edges to deployed resources with confidence scoring
- Uses 4 evidence types: label match (40%), namespace (10%), temporal proximity (30%), reconcile events (20%)
- Minimum confidence threshold: 0.5 (50%)

Example:
```
HelmRelease (flux-system)
  │
  ├─REFERENCES_SPEC──→ Secret: frontend-values
  │
  └─MANAGES (0.94 confidence)──→ Deployment: frontend (production)
      │
      └─OWNS──→ ReplicaSet
          │
          └─OWNS──→ Pod
```

### Implementing Custom Extractors

See `docs/flux-crd-extractor-implementation-plan.md` for detailed guide.

Quick example:
```go
type MyExtractor struct {
    logger *logging.Logger
}

func (e *MyExtractor) Name() string {
    return "my-extractor"
}

func (e *MyExtractor) Matches(event models.Event) bool {
    return event.Resource.Group == "my.api.group" &&
           event.Resource.Kind == "MyResource"
}

func (e *MyExtractor) ExtractRelationships(
    ctx context.Context,
    event models.Event,
    lookup ResourceLookup,
) ([]graph.Edge, error) {
    // Extract relationships from resource spec
    // Return REFERENCES_SPEC or MANAGES edges
}

func (e *MyExtractor) Priority() int {
    return 100 // Run after native K8s extractors
}
```

Register in `internal/graph/sync/builder.go`:
```go
registry.Register(extractors.NewMyExtractor())
```

## Design Documents

See `docs/graph-reasoning-layer-design.md` for:
- Full architecture overview
- Causality inference heuristics
- MCP tool integration
- MVP implementation plan

## Status

**Phase 1**: Graph Schema & Storage ✅ **COMPLETE** (2025-12-18)
- [x] Models and schema definition
- [x] FalkorDB client wrapper
- [x] Query builders
- [x] Docker Compose setup
- [x] Redis client integration (`github.com/redis/go-redis/v9`)
- [x] Unit tests (32 tests, all passing)
- [x] Integration tests (7 test cases with FalkorDB)
- [x] Makefile targets for testing and development

**Implementation Summary**: See `/docs/graph-layer-phase1-summary.md`

**Phase 2**: Sync Pipeline ✅ **CORE COMPLETE** (2025-12-18)
- [x] Graph Builder with relationship extraction
- [x] Causality Inference Engine (8 heuristics)
- [x] Retention Manager with periodic cleanup
- [x] Pipeline Coordinator
- [x] Unit tests (15 tests, all passing)
- [ ] Event Listener (pending Phase 3 integration)

**Implementation Summary**: See `/docs/graph-layer-phase2-summary.md`

**Phase 3**: Custom Resource Extractors ✅ **COMPLETE** (2025-12-19)
- [x] Pluggable extractor framework
- [x] ResourceLookup interface for graph queries
- [x] ExtractorRegistry for managing multiple extractors
- [x] Flux HelmRelease extractor
  - [x] Spec reference extraction (valuesFrom, sourceRef, secretRef)
  - [x] Managed resource discovery with confidence scoring
  - [x] Evidence-based relationship inference
- [x] Unit tests (11 tests, all passing)
- [x] Integration tests (3 test scenarios)

**Implementation Summary**: See `/docs/crd-extractor-implementation-summary.md`

**Next Phases**:
- Phase 4: MCP Tools (find_root_cause with CRD relationships, blast_radius)
- Phase 5: Additional Extractors (ArgoCD, Crossplane, Cert-Manager)
- Phase 6: Revalidation Logic (background jobs, confidence decay)
- Phase 7: Production Deployment & Monitoring
