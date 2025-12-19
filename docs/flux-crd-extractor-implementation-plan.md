# Implementation Plan: Custom Resource Relationship Modeling in Spectre

**Author**: GitHub Copilot CLI  
**Date**: 2025-12-19  
**Status**: Draft  
**Target**: Spectre Graph Reasoning Layer Extension

---

## Executive Summary

This plan extends Spectre's graph reasoning layer to model relationships involving Custom Resources (CRs), with **Flux HelmRelease** as the initial implementation target. The design prioritizes **evidence-based relationship extraction** with explicit confidence scoring, avoiding blind ownership inference while remaining extensible to ArgoCD, Crossplane, and other CRD ecosystems.

### Core Principles
1. **All inferred relationships carry evidence and confidence scores** (0.0 - 1.0)
2. **Graph remains rebuildable** from Spectre's append-only storage
3. **No blind inference** from labels/annotations alone
4. **Temporal validation** prevents causality violations
5. **Incremental updates** avoid full graph rebuilds

---

## A. Graph Schema Changes

### 1. New Edge Types

Add four new relationship types to `internal/graph/models.go`:

```go
const (
    // Existing edges...
    EdgeTypeOwns              EdgeType = "OWNS"
    EdgeTypeChanged           EdgeType = "CHANGED"
    EdgeTypeTriggeredBy       EdgeType = "TRIGGERED_BY"
    // ... existing edges ...
    
    // NEW: Custom Resource relationships
    EdgeTypeReferencesSpec    EdgeType = "REFERENCES_SPEC"    // Explicit spec references
    EdgeTypeManages           EdgeType = "MANAGES"            // Lifecycle management (inferred)
    EdgeTypeAnnotates         EdgeType = "ANNOTATES"          // Label/annotation linkage
    EdgeTypeCreatesObserved   EdgeType = "CREATES_OBSERVED"   // Observed creation correlation
)
```

### 2. Edge Property Structures

Add new edge property types:

```go
// ReferencesSpecEdge represents explicit references in resource spec
// Example: HelmRelease → Secret (valuesFrom.secretKeyRef)
type ReferencesSpecEdge struct {
    FieldPath   string `json:"fieldPath"`   // JSONPath to the reference (e.g., "spec.valuesFrom[0].name")
    RefKind     string `json:"refKind"`     // Referenced resource kind
    RefName     string `json:"refName"`     // Referenced resource name
    RefNamespace string `json:"refNamespace,omitempty"` // Namespace (if different)
}

// ManagesEdge represents lifecycle management relationship (INFERRED)
// Example: HelmRelease → Deployment (HelmRelease manages Deployment lifecycle)
type ManagesEdge struct {
    Confidence      float64           `json:"confidence"`      // 0.0-1.0 confidence score
    Evidence        []EvidenceItem    `json:"evidence"`        // Evidence supporting this relationship
    FirstObserved   int64             `json:"firstObserved"`   // When first detected (Unix nanoseconds)
    LastValidated   int64             `json:"lastValidated"`   // Last validation timestamp
    ValidationState ValidationState   `json:"validationState"` // Current validation state
}

// AnnotatesEdge represents label/annotation-based linkage
// Example: Deployment has label "helm.toolkit.fluxcd.io/name: myrelease"
type AnnotatesEdge struct {
    AnnotationKey   string  `json:"annotationKey"`   // Full annotation key
    AnnotationValue string  `json:"annotationValue"` // Annotation value
    Confidence      float64 `json:"confidence"`      // Confidence based on annotation reliability
}

// CreatesObservedEdge represents observed creation following reconcile
// Example: HelmRelease reconciled → new Pod appeared within 30s
type CreatesObservedEdge struct {
    Confidence       float64 `json:"confidence"`       // Temporal correlation confidence
    ObservedLagMs    int64   `json:"observedLagMs"`    // Time between reconcile and creation
    ReconcileEventID string  `json:"reconcileEventId"` // Event ID of triggering reconcile
    Evidence         string  `json:"evidence"`         // Why we believe this (e.g., "pod created 5s after HelmRelease reconcile")
}

// EvidenceItem represents a piece of evidence for an inferred relationship
type EvidenceItem struct {
    Type      EvidenceType `json:"type"`      // Label, Temporal, Annotation, etc.
    Value     string       `json:"value"`     // Evidence value
    Weight    float64      `json:"weight"`    // How much this evidence contributes to confidence
    Timestamp int64        `json:"timestamp"` // When evidence was observed
}

// EvidenceType categorizes evidence
type EvidenceType string

const (
    EvidenceTypeLabel       EvidenceType = "label"       // Label match
    EvidenceTypeAnnotation  EvidenceType = "annotation"  // Annotation match
    EvidenceTypeTemporal    EvidenceType = "temporal"    // Temporal proximity
    EvidenceTypeNamespace   EvidenceType = "namespace"   // Same namespace
    EvidenceTypeOwnership   EvidenceType = "ownership"   // OwnerReference present
    EvidenceTypeReconcile   EvidenceType = "reconcile"   // Reconcile event correlation
)

// ValidationState tracks the validation state of inferred edges
type ValidationState string

const (
    ValidationStateValid      ValidationState = "valid"      // Passes validation checks
    ValidationStateStale      ValidationState = "stale"      // Needs revalidation
    ValidationStateInvalid    ValidationState = "invalid"    // Failed validation
    ValidationStatePending    ValidationState = "pending"    // Not yet validated
)
```

### 3. ResourceIdentity Node Enhancements

No changes to node structure required, but add metadata tracking:

```go
// Add to ResourceIdentity (optional labels field for extractor use)
type ResourceIdentity struct {
    // ... existing fields ...
    
    // NEW: Optional labels for relationship matching (not indexed in graph)
    // Populated only when needed by extractors, not persisted to graph
    Labels      map[string]string `json:"-"` // Not serialized to graph
}
```

**Rationale**: Labels are high-cardinality and change frequently. We extract them on-demand from `Event.Data` rather than duplicating them in graph nodes.

### 4. Query Builders

Add query builders to `internal/graph/schema.go`:

```go
// CreateReferencesSpecEdgeQuery creates a REFERENCES_SPEC relationship
func CreateReferencesSpecEdgeQuery(sourceUID, targetUID string, props ReferencesSpecEdge) GraphQuery {
    return GraphQuery{
        Query: `
            MATCH (source:ResourceIdentity {uid: $sourceUID})
            MATCH (target:ResourceIdentity {uid: $targetUID})
            MERGE (source)-[r:REFERENCES_SPEC]->(target)
            ON CREATE SET
                r.fieldPath = $fieldPath,
                r.refKind = $refKind,
                r.refName = $refName,
                r.refNamespace = $refNamespace
        `,
        Parameters: map[string]interface{}{
            "sourceUID":    sourceUID,
            "targetUID":    targetUID,
            "fieldPath":    props.FieldPath,
            "refKind":      props.RefKind,
            "refName":      props.RefName,
            "refNamespace": props.RefNamespace,
        },
    }
}

// CreateManagesEdgeQuery creates a MANAGES relationship with confidence
func CreateManagesEdgeQuery(managerUID, managedUID string, props ManagesEdge) GraphQuery {
    evidenceJSON, _ := json.Marshal(props.Evidence)
    
    return GraphQuery{
        Query: `
            MATCH (manager:ResourceIdentity {uid: $managerUID})
            MATCH (managed:ResourceIdentity {uid: $managedUID})
            MERGE (manager)-[r:MANAGES]->(managed)
            ON CREATE SET
                r.confidence = $confidence,
                r.evidence = $evidence,
                r.firstObserved = $firstObserved,
                r.lastValidated = $lastValidated,
                r.validationState = $validationState
            ON MATCH SET
                r.confidence = $confidence,
                r.evidence = $evidence,
                r.lastValidated = $lastValidated,
                r.validationState = $validationState
        `,
        Parameters: map[string]interface{}{
            "managerUID":      managerUID,
            "managedUID":      managedUID,
            "confidence":      props.Confidence,
            "evidence":        string(evidenceJSON),
            "firstObserved":   props.FirstObserved,
            "lastValidated":   props.LastValidated,
            "validationState": string(props.ValidationState),
        },
    }
}

// FindManagedResourcesQuery finds all resources managed by a CR
func FindManagedResourcesQuery(crUID string, minConfidence float64) GraphQuery {
    return GraphQuery{
        Query: `
            MATCH (cr:ResourceIdentity {uid: $crUID})
                  -[manages:MANAGES]->(managed:ResourceIdentity)
            WHERE manages.confidence >= $minConfidence
              AND managed.deleted = false
            RETURN managed, manages
            ORDER BY manages.confidence DESC
        `,
        Parameters: map[string]interface{}{
            "crUID":         crUID,
            "minConfidence": minConfidence,
        },
    }
}

// FindStaleInferredEdgesQuery finds edges needing revalidation
func FindStaleInferredEdgesQuery(cutoffTimestamp int64) GraphQuery {
    return GraphQuery{
        Query: `
            MATCH (source)-[edge:MANAGES]->(target)
            WHERE edge.lastValidated < $cutoffTimestamp
               OR edge.validationState = 'stale'
            RETURN source.uid as sourceUID, 
                   target.uid as targetUID,
                   edge
            LIMIT 1000
        `,
        Parameters: map[string]interface{}{
            "cutoffTimestamp": cutoffTimestamp,
        },
    }
}
```

### 5. Example Graph Structure

```
┌─────────────────────────────────────────┐
│  HelmRelease: frontend                  │
│  (helm.toolkit.fluxcd.io/v2beta1)       │
│  namespace: production                  │
└─────────┬───────────────────────────────┘
          │
          │ REFERENCES_SPEC
          │ {fieldPath: "spec.valuesFrom[0]"}
          │ {confidence: 1.0}  ← explicit reference
          ▼
┌─────────────────────────────────────────┐
│  Secret: frontend-values                │
│  namespace: production                  │
└─────────────────────────────────────────┘

          │
          │ MANAGES
          │ {confidence: 0.85}
          │ {evidence: [
          │    {type: "label", value: "helm.toolkit.fluxcd.io/name=frontend"},
          │    {type: "temporal", value: "created 8s after reconcile"},
          │    {type: "namespace", value: "production"}
          │  ]}
          ▼
┌─────────────────────────────────────────┐
│  Deployment: frontend                   │
│  namespace: production                  │
└─────────┬───────────────────────────────┘
          │
          │ OWNS (controller=true)
          │ ← Native K8s ownership
          ▼
┌─────────────────────────────────────────┐
│  ReplicaSet: frontend-abc123            │
└─────────────────────────────────────────┘
```

**Key Observations**:
- `REFERENCES_SPEC`: High confidence (1.0), explicit from spec
- `MANAGES`: Lower confidence (0.85), inferred from multiple evidence sources
- `OWNS`: Native Kubernetes, not inferred

---

## B. Relationship Extraction Pipeline

### 1. Pluggable Extractor Interface

Create `internal/graph/sync/extractors/extractor.go`:

```go
package extractors

import (
    "context"
    "encoding/json"

    "github.com/moolen/spectre/internal/graph"
    "github.com/moolen/spectre/internal/models"
)

// RelationshipExtractor extracts relationships from Kubernetes resources
type RelationshipExtractor interface {
    // Name returns the extractor identifier (e.g., "flux-helmrelease")
    Name() string
    
    // Matches checks if this extractor applies to the given resource
    Matches(event models.Event) bool
    
    // ExtractRelationships extracts relationships from the resource
    // Returns edges to create/update in the graph
    ExtractRelationships(ctx context.Context, event models.Event, lookup ResourceLookup) ([]graph.Edge, error)
    
    // Priority returns extraction priority (lower = earlier execution)
    // Used when multiple extractors match the same resource
    Priority() int
}

// ResourceLookup provides access to existing graph data for relationship validation
type ResourceLookup interface {
    // FindResourceByUID retrieves a resource node by UID
    FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error)
    
    // FindResourceByNamespace finds resources by namespace and name
    FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error)
    
    // FindRecentEvents finds recent ChangeEvents for a resource
    FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error)
    
    // QueryGraph executes arbitrary Cypher queries (for complex lookups)
    QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
}

// ExtractorRegistry manages relationship extractors
type ExtractorRegistry struct {
    extractors []RelationshipExtractor
    lookup     ResourceLookup
}

// NewExtractorRegistry creates a new registry
func NewExtractorRegistry(lookup ResourceLookup) *ExtractorRegistry {
    return &ExtractorRegistry{
        extractors: []RelationshipExtractor{},
        lookup:     lookup,
    }
}

// Register adds an extractor to the registry
func (r *ExtractorRegistry) Register(extractor RelationshipExtractor) {
    r.extractors = append(r.extractors, extractor)
    
    // Sort by priority
    sort.Slice(r.extractors, func(i, j int) bool {
        return r.extractors[i].Priority() < r.extractors[j].Priority()
    })
}

// Extract applies all matching extractors to an event
func (r *ExtractorRegistry) Extract(ctx context.Context, event models.Event) ([]graph.Edge, error) {
    var allEdges []graph.Edge
    
    for _, extractor := range r.extractors {
        if !extractor.Matches(event) {
            continue
        }
        
        edges, err := extractor.ExtractRelationships(ctx, event, r.lookup)
        if err != nil {
            // Log but continue - partial extraction is acceptable
            log.Warnf("Extractor %s failed for event %s: %v", extractor.Name(), event.ID, err)
            continue
        }
        
        allEdges = append(allEdges, edges...)
    }
    
    return allEdges, nil
}
```

### 2. Integration into GraphBuilder

Modify `internal/graph/sync/builder.go`:

```go
type graphBuilder struct {
    logger            *logging.Logger
    client            graph.Client
    extractorRegistry *extractors.ExtractorRegistry  // NEW
}

func NewGraphBuilderWithClient(client graph.Client) GraphBuilder {
    // Create resource lookup adapter
    lookup := &graphClientLookup{client: client}
    
    // Create extractor registry
    registry := extractors.NewExtractorRegistry(lookup)
    
    // Register built-in extractors
    registry.Register(extractors.NewFluxHelmReleaseExtractor())
    // Future: registry.Register(extractors.NewArgoCDApplicationExtractor())
    
    return &graphBuilder{
        logger:            logging.GetLogger("graph.sync.builder"),
        client:            client,
        extractorRegistry: registry,  // NEW
    }
}

// ExtractRelationships now delegates to the registry
func (b *graphBuilder) ExtractRelationships(ctx context.Context, event models.Event) ([]graph.Edge, error) {
    edges := []graph.Edge{}
    
    // ... existing code for native K8s relationships ...
    
    // NEW: Apply custom resource extractors
    crEdges, err := b.extractorRegistry.Extract(ctx, event)
    if err != nil {
        b.logger.Warn("Custom resource extraction failed for event %s: %v", event.ID, err)
    } else {
        edges = append(edges, crEdges...)
    }
    
    return edges, nil
}
```

### 3. Resource Lookup Implementation

Create `internal/graph/sync/extractors/lookup.go`:

```go
package extractors

import (
    "context"
    "fmt"

    "github.com/moolen/spectre/internal/graph"
)

// graphClientLookup implements ResourceLookup using graph.Client
type graphClientLookup struct {
    client graph.Client
}

func (l *graphClientLookup) FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
    query := graph.FindResourceByUIDQuery(uid)
    result, err := l.client.ExecuteQuery(ctx, query)
    if err != nil {
        return nil, err
    }
    
    if len(result.Rows) == 0 {
        return nil, fmt.Errorf("resource not found: %s", uid)
    }
    
    // Parse node from result
    // ... implementation ...
}

func (l *graphClientLookup) FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
    query := graph.GraphQuery{
        Query: `
            MATCH (r:ResourceIdentity)
            WHERE r.namespace = $namespace
              AND r.kind = $kind
              AND r.name = $name
              AND r.deleted = false
            RETURN r
            LIMIT 1
        `,
        Parameters: map[string]interface{}{
            "namespace": namespace,
            "kind":      kind,
            "name":      name,
        },
    }
    
    result, err := l.client.ExecuteQuery(ctx, query)
    // ... parse and return ...
}

func (l *graphClientLookup) FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error) {
    now := time.Now().UnixNano()
    query := graph.FindChangeEventsByResourceQuery(uid, now-windowNs, now)
    
    result, err := l.client.ExecuteQuery(ctx, query)
    // ... parse and return ...
}

func (l *graphClientLookup) QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
    return l.client.ExecuteQuery(ctx, query)
}
```

### 4. Lifecycle Hooks

Extractors are invoked:
- **On CREATE**: Extract all relationships (spec references + inferred)
- **On UPDATE**: Re-extract relationships, update confidence scores
- **On DELETE**: Mark edges as stale for cleanup

**Error Handling Strategy**:
1. **Per-extractor failures**: Log warning, continue with other extractors
2. **Missing referenced resources**: Create edge with lower confidence, mark for revalidation
3. **Query timeouts**: Skip temporal validation, use label-only confidence
4. **Graph unavailable**: Queue edges for retry (with backoff)

---

## C. Flux HelmRelease Extractor

### 1. Field Paths Used

| **Field Path** | **Purpose** | **Example** |
|---|---|---|
| `spec.valuesFrom[i].name` | Reference to Secret/ConfigMap with Helm values | `{"kind": "Secret", "name": "app-values"}` |
| `spec.chart.spec.sourceRef` | Reference to HelmRepository/GitRepository | `{"kind": "HelmRepository", "name": "bitnami"}` |
| `spec.kubeConfig.secretRef` | Reference to Secret with kubeconfig | `{"name": "remote-cluster-kubeconfig"}` |
| `spec.targetNamespace` | Namespace where Helm chart is installed | `"production"` |

### 2. Label Keys Relied Upon

| **Label Key** | **Usage** | **Reliability** |
|---|---|---|
| `helm.toolkit.fluxcd.io/name` | Identifies resources managed by HelmRelease | **High** - Flux always sets this |
| `helm.toolkit.fluxcd.io/namespace` | Identifies source namespace of HelmRelease | **Medium** - May be omitted |
| `app.kubernetes.io/managed-by` | Generic "managed by" label | **Low** - Not Flux-specific |

### 3. Confidence Scoring Logic

```
Confidence = (Σ earned_weight) / (Σ total_weight)

Where:
  earned_weight = weight * match_score

Evidence weights:
  - Label match:          0.4  (40%)
  - Namespace match:      0.1  (10%)
  - Temporal proximity:   0.3  (30%)
  - Reconcile event:      0.2  (20%)

Example scoring:
  ✓ Label match (helm.toolkit.fluxcd.io/name=frontend)  → +0.4
  ✓ Same namespace (production)                         → +0.1
  ✓ Created 8s after reconcile                          → +0.24 (0.3 * 0.8 proximity)
  ✓ Reconcile event present                             → +0.2
  ───────────────────────────────────────────────────────
  Total confidence: 0.94 / 1.0 = 94%
```

### 4. Failure Modes & Mitigations

| **Failure Mode** | **Impact** | **Mitigation** |
|---|---|---|
| **Referenced Secret doesn't exist yet** | `REFERENCES_SPEC` edge points to empty UID | Create edge with validation state `pending`, revalidate later |
| **Labels are missing** | Lower confidence for `MANAGES` edge | Rely more on temporal + reconcile evidence |
| **Temporal data unavailable** | Can't validate creation timing | Use label-only confidence (lower threshold: 0.4) |
| **HelmRelease creates resources in different namespace** | Namespace filter misses them | Check `spec.targetNamespace`, search across namespaces |
| **Resource deleted before extraction** | Race condition | Check `deleted` field in queries, mark edges as stale |
| **Multiple HelmReleases with overlapping labels** | Ambiguous ownership | Prefer most recent reconcile event, highest confidence wins |

---

## D. Temporal Validation & Confidence Scoring

### 1. Temporal Ordering Validation

**Rule**: A resource cannot be managed by a HelmRelease that was created *after* the resource.

```go
// ValidateTemporalOrdering checks if cause precedes effect
func ValidateTemporalOrdering(
    managerFirstSeen int64,  // HelmRelease creation time
    managedFirstSeen int64,  // Managed resource creation time
) bool {
    // Allow 5-second tolerance for clock skew
    toleranceNs := int64(5 * time.Second.Nanoseconds())
    
    return managerFirstSeen <= (managedFirstSeen + toleranceNs)
}
```

**Application**: Run this check during extraction and revalidation jobs.

### 2. Confidence Computation Formula

```go
// ComputeManagementConfidence calculates confidence for MANAGES edge
func ComputeManagementConfidence(evidence []graph.EvidenceItem) float64 {
    if len(evidence) == 0 {
        return 0.0
    }
    
    totalWeight := 0.0
    earnedWeight := 0.0
    
    for _, item := range evidence {
        totalWeight += item.Weight
        earnedWeight += item.Weight // item.Weight already includes match score
    }
    
    if totalWeight == 0 {
        return 0.0
    }
    
    return earnedWeight / totalWeight
}
```

### 3. Confidence Decay Over Time

Confidence degrades if not revalidated:

```go
// ApplyConfidenceDecay reduces confidence based on time since last validation
func ApplyConfidenceDecay(
    originalConfidence float64,
    lastValidated int64,
    now int64,
    halfLifeHours int,
) float64 {
    hoursSinceValidation := float64(now-lastValidated) / float64(time.Hour.Nanoseconds())
    decayFactor := math.Pow(0.5, hoursSinceValidation/float64(halfLifeHours))
    
    return originalConfidence * decayFactor
}

// Example: After 24 hours without revalidation (halfLife=24h):
// 0.9 confidence → 0.45 confidence
```

### 4. Edge Downgrade & Removal

```go
// ValidationJob revalidates stale inferred edges
func (v *ValidationJob) Run(ctx context.Context) error {
    cutoff := time.Now().Add(-24 * time.Hour).UnixNano()
    
    query := graph.FindStaleInferredEdgesQuery(cutoff)
    result, err := v.client.ExecuteQuery(ctx, query)
    if err != nil {
        return err
    }
    
    for _, row := range result.Rows {
        sourceUID, targetUID, edge := parseEdgeRow(row)
        
        // Re-score the relationship
        newConfidence, newEvidence := v.rescore(ctx, sourceUID, targetUID)
        
        if newConfidence < 0.3 {
            // Confidence too low - remove edge
            v.deleteEdge(ctx, sourceUID, targetUID, edge.Type)
            v.logger.Info("Removed stale edge: %s -[MANAGES]-> %s (confidence dropped to %.2f)", 
                sourceUID, targetUID, newConfidence)
        } else if newConfidence < edge.Confidence {
            // Downgrade confidence
            updatedEdge := edge
            updatedEdge.Confidence = newConfidence
            updatedEdge.Evidence = newEvidence
            updatedEdge.LastValidated = time.Now().UnixNano()
            updatedEdge.ValidationState = graph.ValidationStateValid
            
            v.updateEdge(ctx, sourceUID, targetUID, updatedEdge)
            v.logger.Info("Downgraded edge confidence: %s -[MANAGES]-> %s (%.2f -> %.2f)",
                sourceUID, targetUID, edge.Confidence, newConfidence)
        } else {
            // Confidence maintained or improved - update timestamp
            edge.LastValidated = time.Now().UnixNano()
            edge.ValidationState = graph.ValidationStateValid
            v.updateEdge(ctx, sourceUID, targetUID, edge)
        }
    }
    
    return nil
}
```

---

## E. Incremental Graph Updates

### 1. Edge Addition Without Full Rebuild

The extractor pipeline already supports incremental updates:

```go
// ProcessEvent in sync pipeline
func (p *pipeline) ProcessEvent(ctx context.Context, event models.Event) error {
    // Build graph update (includes extractor execution)
    update, err := p.builder.BuildFromEvent(ctx, event)
    if err != nil {
        return err
    }
    
    // Apply update to graph (idempotent upserts)
    for _, edge := range update.Edges {
        query := buildEdgeUpsertQuery(edge) // Uses MERGE for idempotency
        if _, err := p.client.ExecuteQuery(ctx, query); err != nil {
            p.logger.Warn("Failed to create edge: %v", err)
        }
    }
    
    return nil
}
```

**Key Properties**:
- Uses `MERGE` for idempotent edge creation
- Existing edges are updated (`ON MATCH SET`)
- New edges are created (`ON CREATE SET`)

### 2. Revalidation Jobs

Run periodic background job to revalidate inferred edges:

```go
// RevalidationScheduler runs validation jobs
type RevalidationScheduler struct {
    client   graph.Client
    interval time.Duration
    logger   *logging.Logger
}

func (s *RevalidationScheduler) Start(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := s.runValidation(ctx); err != nil {
                s.logger.Error("Validation job failed: %v", err)
            }
        }
    }
}

func (s *RevalidationScheduler) runValidation(ctx context.Context) error {
    job := &ValidationJob{
        client: s.client,
        logger: s.logger,
    }
    
    return job.Run(ctx)
}
```

**Configuration**:
- **Default interval**: 6 hours
- **Batch size**: 1000 edges per run
- **Validation cutoff**: 24 hours since last validation

### 3. Stale Edge Handling

Edges become stale when:
1. **Time-based**: Not revalidated in 24 hours
2. **Event-based**: Referenced resource deleted
3. **Confidence-based**: Confidence drops below 0.3

**Cleanup Strategy**:
```go
// CleanupStaleEdges removes edges that failed revalidation
func CleanupStaleEdges(ctx context.Context, client graph.Client) error {
    query := graph.GraphQuery{
        Query: `
            MATCH (source)-[edge:MANAGES]->(target)
            WHERE edge.validationState = 'invalid'
               OR (edge.confidence < 0.3 AND edge.lastValidated < $cutoff)
            DELETE edge
            RETURN count(edge) as deletedCount
        `,
        Parameters: map[string]interface{}{
            "cutoff": time.Now().Add(-48 * time.Hour).UnixNano(),
        },
    }
    
    result, err := client.ExecuteQuery(ctx, query)
    if err != nil {
        return err
    }
    
    // Log cleanup stats
    deletedCount := extractCount(result)
    log.Infof("Cleaned up %d stale edges", deletedCount)
    
    return nil
}
```

---

## F. Testing Strategy

### 1. Unit Tests

**File**: `internal/graph/sync/extractors/flux_helmrelease_test.go`

Test coverage includes:
- **Spec reference extraction**: Verify correct parsing of `valuesFrom`, `sourceRef`, etc.
- **Confidence scoring**: Test all evidence combinations
- **Temporal validation**: Clock skew handling
- **Missing resources**: Edge creation with pending state
- **Label matching**: Heuristic validation

**Example test cases**:
- `TestFluxHelmReleaseExtractor_ExtractSpecReferences`
- `TestFluxHelmReleaseExtractor_ConfidenceScoring`
- `TestFluxHelmReleaseExtractor_TemporalOrdering`
- `TestFluxHelmReleaseExtractor_MissingReferences`

### 2. End-to-End Tests

**File**: `tests/e2e/flux_helmrelease_test.go`

Test scenarios:
1. **HelmRelease → Deployment**: Verify `MANAGES` edge creation
2. **Spec references**: Verify `REFERENCES_SPEC` edges for Secrets/ConfigMaps
3. **Confidence decay**: Verify confidence drops over time without revalidation
4. **Multiple HelmReleases**: Verify correct attribution when labels overlap
5. **Cross-namespace references**: Verify `targetNamespace` handling

**Example**:
```go
func TestFluxHelmRelease_ManagedResourceDiscovery(t *testing.T) {
    testCtx := helpers.SetupE2ETest(t, helpers.E2EOptions{
        GraphEnabled: true,
    })
    defer testCtx.Cleanup()
    
    // 1. Deploy Flux HelmRelease
    helmRelease := createTestHelmRelease("frontend", "default")
    err := testCtx.K8sClient.ApplyUnstructured(helmRelease)
    require.NoError(t, err)
    
    // 2. Wait for resources to be created
    time.Sleep(10 * time.Second)
    
    // 3. Wait for graph to sync
    time.Sleep(5 * time.Second)
    
    // 4. Query graph for MANAGES edges
    query := graph.FindManagedResourcesQuery("frontend-uid", 0.5)
    result, err := testCtx.GraphClient.ExecuteQuery(context.Background(), query)
    require.NoError(t, err)
    
    // 5. Verify managed resources are discovered
    assert.Greater(t, len(result.Rows), 0, "Should discover at least one managed resource")
    
    // 6. Verify edge properties
    for _, row := range result.Rows {
        edge := extractManagesEdge(row)
        assert.GreaterOrEqual(t, edge.Confidence, 0.5, "Confidence should meet threshold")
        assert.NotEmpty(t, edge.Evidence, "Should have evidence")
        assert.Equal(t, graph.ValidationStateValid, edge.ValidationState)
    }
}
```

### 3. Assertions (Avoiding LLM Nondeterminism)

Use **deterministic assertions** on graph structure:

```go
// ✅ GOOD: Deterministic assertions
assert.Len(t, edges, expectedCount)
assert.Equal(t, graph.EdgeTypeManages, edge.Type)
assert.GreaterOrEqual(t, edge.Confidence, 0.5)
assert.Contains(t, edge.Evidence, expectedEvidenceType)

// ❌ BAD: LLM-dependent assertions
assert.Equal(t, "HelmRelease manages Deployment", edge.Reason) // Reason text varies
```

**Test Isolation**:
- Each test gets its own namespace
- Graph cleanup between tests (delete all nodes in test namespace)
- No shared state between tests

---

## G. MVP Scope & Rollout Plan

### 1. MVP Scope (Merge Target)

**In Scope**:
- ✅ New edge types: `REFERENCES_SPEC`, `MANAGES`, `ANNOTATES`, `CREATES_OBSERVED`
- ✅ Extractor framework (`RelationshipExtractor` interface, registry)
- ✅ Flux HelmRelease extractor (spec references + managed resources)
- ✅ Confidence scoring (4 evidence types)
- ✅ Temporal validation
- ✅ Incremental graph updates (no full rebuild)
- ✅ Unit tests (extractor logic)
- ✅ E2E tests (1 scenario: HelmRelease → Deployment)
- ✅ Documentation (schema, extractor guide)

**Explicit Non-Goals** (Follow-Up PRs):
- ❌ ArgoCD Application extractor
- ❌ Crossplane Composition extractor
- ❌ Revalidation scheduler (background job)
- ❌ Confidence decay logic
- ❌ MCP tool enhancements (e.g., `trace_cr_ownership`)
- ❌ UI visualization of CRD relationships
- ❌ Performance optimization (batch extraction)

### 2. Rollout Plan

**Phase 1: Core Infrastructure** (Week 1)
- [ ] Implement new edge types in `models.go`
- [ ] Add query builders to `schema.go`
- [ ] Create `extractor.go` interface
- [ ] Implement `ExtractorRegistry`
- [ ] Unit tests for registry

**Phase 2: Flux Extractor** (Week 2)
- [ ] Implement `FluxHelmReleaseExtractor`
- [ ] Add spec reference extraction
- [ ] Add managed resource discovery
- [ ] Implement confidence scoring
- [ ] Unit tests for extractor (15+ test cases)

**Phase 3: Integration** (Week 3)
- [ ] Integrate registry into `GraphBuilder`
- [ ] Add resource lookup implementation
- [ ] Test incremental updates
- [ ] E2E test: HelmRelease → Deployment

**Phase 4: Validation & Documentation** (Week 4)
- [ ] Add temporal validation logic
- [ ] Write extractor documentation
- [ ] Update graph schema docs
- [ ] Performance testing (1000 HelmReleases)
- [ ] Code review and merge

**Phase 5: Follow-Up Extensions** (Post-MVP)
- Revalidation scheduler (cron job)
- Confidence decay implementation
- ArgoCD Application extractor
- Crossplane Composition extractor
- MCP tool: `spectre.trace_cr_ownership(resource_uid)`

### 3. Rollback Plan

If issues arise post-merge:

1. **Feature flag**: Add `GRAPH_ENABLE_CR_EXTRACTORS` env var (default: `false`)
2. **Edge cleanup**: Provide migration script to remove CRD edges:
   ```cypher
   MATCH ()-[r:MANAGES|REFERENCES_SPEC|ANNOTATES|CREATES_OBSERVED]->()
   DELETE r
   ```
3. **Gradual rollout**: Enable in staging → canary → production

---

## H. Success Criteria

A successful implementation allows:

1. **✅ Safe Modeling**: CRD relationships carry explicit confidence scores and evidence
2. **✅ Causality Reasoning**: LLMs can trace failures through HelmRelease → Deployment → Pod chains
3. **✅ No Hallucination**: All relationships backed by evidence (labels, timing, spec refs)
4. **✅ Extensibility**: Adding ArgoCD extractor requires <200 LOC
5. **✅ Production Safety**: Partial failures don't corrupt graph, edges degrade gracefully

**Acceptance Criteria**:
- [ ] Unit tests: >90% coverage for extractor logic
- [ ] E2E test: HelmRelease manages Deployment (confidence ≥ 0.7)
- [ ] Performance: Extract relationships for 1000 HelmReleases in <10s
- [ ] No regressions: Existing graph tests still pass
- [ ] Documentation: Extractor implementation guide published

---

## I. Key Assumptions & Justifications

### Assumptions

1. **Flux uses consistent labels**: We assume `helm.toolkit.fluxcd.io/name` is reliably set by Flux
   - **Justification**: Flux v2 documentation guarantees this label on all managed resources
   - **Mitigation**: Fallback to temporal + namespace evidence if label missing

2. **Clock skew tolerance**: 5-second tolerance for temporal ordering
   - **Justification**: Kubernetes clusters typically use NTP with <1s skew
   - **Mitigation**: Configurable tolerance via environment variable

3. **Reconcile events are observable**: We can detect HelmRelease reconcile events
   - **Justification**: Flux emits status updates on reconcile
   - **Mitigation**: Degrade to label-only confidence if no recent events

4. **Graph queries complete in <500ms**: Resource lookups during extraction are fast
   - **Justification**: FalkorDB benchmarks show <100ms for UID lookups
   - **Mitigation**: Timeout extraction if lookups exceed 1s

5. **HelmRelease spec is stable**: Field paths don't change between Flux versions
   - **Justification**: Flux v2beta1/v2beta2 APIs are stable (GA since 2022)
   - **Mitigation**: Version detection logic to handle API changes

### Non-Assumptions (Explicitly Avoided)

1. ❌ **OwnerReferences are always set**: Flux does NOT set ownerReferences on managed resources
2. ❌ **Label selectors match resources**: HelmRelease doesn't use label selectors
3. ❌ **Resources are always in the same namespace**: `targetNamespace` may differ
4. ❌ **Confidence scores are permanent**: Scores decay and require revalidation

---

## J. Future Extensions

### 1. ArgoCD Application Extractor

Similar pattern to Flux, but different evidence:
- **Spec references**: `source.repoURL`, `destination.namespace`
- **Labels**: `app.kubernetes.io/instance`, `argocd.argoproj.io/instance`
- **Annotations**: `argocd.argoproj.io/tracking-id`
- **Confidence weights**: Label (0.5), Tracking ID (0.3), Temporal (0.2)

Estimated effort: **2-3 days** (reuse extractor framework)

### 2. Crossplane Composition Extractor

Tracks Crossplane resource relationships:
- **Spec references**: `compositeRef`, `resourceRefs`
- **Labels**: `crossplane.io/composite`, `crossplane.io/claim-name`
- **Evidence**: OwnerReferences (Crossplane sets these)
- **Confidence weights**: OwnerRef (0.6), Label (0.3), Temporal (0.1)

Estimated effort: **2-3 days**

### 3. Cert-Manager Certificate Extractor

Tracks certificate lifecycle:
- **Spec references**: `secretName`, `issuerRef`
- **Labels**: `cert-manager.io/certificate-name`
- **Evidence**: Secret creation after certificate issuance
- **Confidence weights**: SecretRef (0.5), Label (0.3), Temporal (0.2)

Estimated effort: **1-2 days**

---

## K. Open Questions

1. **Storage API Integration**: Should extractors query Spectre storage API for full resource data, or rely on graph-only information?
   - **Recommendation**: Graph-only for MVP (performance), storage API for future enhancement

2. **Label Caching**: Should we cache labels in graph nodes or always parse from events?
   - **Recommendation**: Parse on-demand (labels change frequently, avoid stale data)

3. **Confidence Threshold**: What minimum confidence should trigger edge creation?
   - **Recommendation**: 0.5 (50%) for MVP, configurable per extractor

4. **Revalidation Frequency**: How often should we revalidate edges?
   - **Recommendation**: Every 6 hours (balance between freshness and load)

5. **Cross-Cluster Support**: How do we handle HelmReleases managing resources in remote clusters?
   - **Recommendation**: Out of scope for MVP (requires multi-cluster graph)

---

## L. References

- **Flux HelmRelease API**: https://fluxcd.io/flux/components/helm/helmreleases/
- **Flux Label Conventions**: https://fluxcd.io/flux/components/helm/helmreleases/#status
- **FalkorDB Cypher Guide**: https://docs.falkordb.com/cypher.html
- **Spectre Graph Design**: `/docs/graph-reasoning-layer-design.md`
- **Spectre Graph Implementation**: `/internal/graph/README.md`

---

## Appendix: Implementation Checklist

### Phase 1: Core Infrastructure
- [ ] Add edge types to `internal/graph/models.go`
- [ ] Add edge property structs
- [ ] Add query builders to `internal/graph/schema.go`
- [ ] Create `internal/graph/sync/extractors/extractor.go`
- [ ] Create `internal/graph/sync/extractors/registry.go`
- [ ] Create `internal/graph/sync/extractors/lookup.go`
- [ ] Unit tests for registry (5 tests)

### Phase 2: Flux Extractor
- [ ] Create `internal/graph/sync/extractors/flux_helmrelease.go`
- [ ] Implement `Matches()` method
- [ ] Implement `extractSpecReferences()`
- [ ] Implement `extractManagedResources()`
- [ ] Implement `scoreManagementRelationship()`
- [ ] Implement confidence scoring logic
- [ ] Unit tests for extractor (15+ tests)

### Phase 3: Integration
- [ ] Modify `internal/graph/sync/builder.go` to use registry
- [ ] Add extractor registration in `NewGraphBuilderWithClient()`
- [ ] Implement `graphClientLookup` adapter
- [ ] Test incremental edge updates
- [ ] Verify idempotent MERGE operations

### Phase 4: Testing
- [ ] Create `tests/e2e/flux_helmrelease_test.go`
- [ ] Implement `TestFluxHelmRelease_ManagedResourceDiscovery`
- [ ] Implement `TestFluxHelmRelease_SpecReferences`
- [ ] Implement `TestFluxHelmRelease_ConfidenceScoring`
- [ ] Add test fixtures (HelmRelease YAML)
- [ ] Verify test isolation (namespace cleanup)

### Phase 5: Documentation
- [ ] Update `internal/graph/README.md` with new edge types
- [ ] Create `docs/extractor-implementation-guide.md`
- [ ] Add Flux extractor example to docs
- [ ] Update MCP tools documentation
- [ ] Add architecture diagrams

### Phase 6: Performance & Validation
- [ ] Benchmark extraction performance (1000 HelmReleases)
- [ ] Verify graph query performance (<500ms)
- [ ] Memory profiling (ensure no leaks)
- [ ] Run existing graph tests (no regressions)
- [ ] Load testing with concurrent extractors

### Phase 7: Code Review & Merge
- [ ] Self-review code changes
- [ ] Run linters (`make lint`)
- [ ] Run all tests (`make test`)
- [ ] Create pull request
- [ ] Address review comments
- [ ] Merge to main branch
