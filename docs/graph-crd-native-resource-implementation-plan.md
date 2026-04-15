# CRD Relationship Extractors Implementation Plan for Spectre

**Document Version:** 1.0  
**Date:** 2025-12-20  
**Status:** APPROVED FOR IMPLEMENTATION

---

## 1. Executive Summary

This document provides a staged implementation plan for extending Spectre's graph reasoning layer with relationship extractors for Kubernetes native resources and high-impact CRDs used by platform teams. The goal is to improve root cause analysis accuracy by making implicit relationships **explicit, typed, and confidence-scored**.

**Key Findings:**
- Spectre already has a **mature extractor framework** (`internal/graph/sync/extractors/`)
- Native K8s relationships are **partially implemented** in `builder.go` but lack completeness
- Flux HelmRelease extractor provides a **proven pattern** for CRD modeling with evidence-based confidence
- The `ResourceLookup` interface enables bidirectional relationship discovery
- The graph is **rebuildable from storage**, making safe iteration possible

**Strategic Approach:**
1. Complete native K8s extractors (ownership, traffic, config) as foundational layer
2. Extend CRD framework using proven Flux patterns
3. Prioritize CRDs by **runtime impact** (ownership/traffic > config > observability)
4. Track evidence and confidence for all inferred relationships
5. Design for incremental deployment without breaking existing functionality

---

## 2. Findings from Codebase Research

### A. Extension Points Located

**Primary Extension Point: `internal/graph/sync/extractors/`**

```
extractors/
├── extractor.go          # RelationshipExtractor interface
├── registry.go           # ExtractorRegistry for managing extractors
├── lookup.go            # ResourceLookup for graph queries
├── rbac.go              # RBAC extractor (RoleBinding → Role/ServiceAccount)
├── flux_helmrelease.go  # Flux HelmRelease extractor (MANAGES, REFERENCES_SPEC)
└── flux_managed_resource.go  # Reverse lookup for Flux-labeled resources
```

**Integration Point: `internal/graph/sync/builder.go`**
- Line 174-182: Calls `extractorRegistry.Extract()` after native K8s extraction
- Native K8s extractors live directly in `builder.go`:
  - `extractOwnershipRelationships()` (line 435) - OWNS edges from ownerReferences
  - `extractSelectorRelationships()` (line 495) - SELECTS edges (incomplete label matching)
  - `extractSchedulingRelationship()` (line 657) - SCHEDULED_ON for Pod→Node
  - `extractVolumeRelationships()` (line 700+) - Pod→PVC MOUNTS
  - `extractServiceAccountRelationship()` - Pod→ServiceAccount

**Current Registration (builder.go:39-41)**:
```go
registry.Register(extractors.NewRBACExtractor())
registry.Register(extractors.NewFluxHelmReleaseExtractor())
```

### B. Existing Abstractions

**1. RelationshipExtractor Interface (extractor.go:11-25)**
```go
type RelationshipExtractor interface {
    Name() string
    Matches(event models.Event) bool
    ExtractRelationships(ctx context.Context, event models.Event, lookup ResourceLookup) ([]graph.Edge, error)
    Priority() int  // Lower = earlier execution
}
```

**2. ResourceLookup Interface (extractor.go:27-40)**
```go
type ResourceLookup interface {
    FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error)
    FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error)
    FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error)
    QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
}
```

**3. Edge Types (models.go:17-40)**

**Observed (High Confidence):**
- `OWNS` - ownerReferences (100% confidence)
- `SCHEDULED_ON` - Pod.spec.nodeName
- `MOUNTS` - Pod volume mounts
- `USES_SERVICE_ACCOUNT` - Pod.spec.serviceAccountName
- `BINDS_ROLE` / `GRANTS_TO` - RBAC explicit bindings
- `REFERENCES_SPEC` - Explicit spec references (e.g., HelmRelease.spec.valuesFrom)
- `EMITTED_EVENT` - K8s Event.involvedObject

**Inferred (Evidence-Based):**
- `MANAGES` - Lifecycle management (Flux/ArgoCD → resources)
- `SELECTS` - Label selectors (Service/Deployment → Pod)
- `ANNOTATES` - Label/annotation linkage
- `CREATES_OBSERVED` - Temporal correlation

**Causal:**
- `TRIGGERED_BY` - Causality inference
- `PRECEDED_BY` - Temporal ordering

**4. Evidence & Confidence System (models.go:141-205)**
```go
type EvidenceItem struct {
    Type      EvidenceType  // label, temporal, annotation, ownership, reconcile
    Value     string
    Weight    float64       // Contribution to confidence
    Timestamp int64
}

type ManagesEdge struct {
    Confidence      float64         // 0.0-1.0
    Evidence        []EvidenceItem
    ValidationState ValidationState  // valid, stale, invalid, pending
}
```

### C. Gaps Identified

**Native K8s Gaps:**
1. **Service→Pod SELECTS**: Label matching not implemented (line 654 returns empty)
2. **Ingress→Service**: No extractor for Ingress traffic routing
3. **NetworkPolicy→Pod**: No network isolation modeling
4. **PVC→PV→StorageClass**: PVC chain incomplete
5. **Job/CronJob→Pod**: No batch workload ownership
6. **DaemonSet/StatefulSet OWNS**: Limited to ownerReferences, no Set-specific logic

**CRD Gaps:**
- Only Flux HelmRelease implemented
- No ArgoCD Application support
- No Gateway API (Gateway, HTTPRoute)
- No Cert-Manager (Certificate, Issuer)
- No External Secrets
- No KEDA, Crossplane

**Architecture Gaps:**
- No revalidation logic for stale MANAGES edges
- No confidence decay over time
- No extractor dependency ordering beyond priority
- No extractor disablement/feature flags

---

## 3. Proposed Architecture for Relationship Extractors

### A. Architectural Principles

**1. Separation of Concerns**
```
builder.go (native K8s)
   ↓ calls
extractorRegistry.Extract() (CRD/custom logic)
   ↓ uses
ResourceLookup (graph queries)
```

**2. Extractor Categories**

| Category | Priority Range | Examples | Confidence |
|----------|---------------|----------|------------|
| **Native Observed** | 0-49 | Ownership, Scheduling, Volumes | 1.0 (explicit) |
| **Native Inferred** | 50-99 | Service→Pod selectors | 0.7-1.0 (label match) |
| **CRD Observed** | 100-199 | RBAC, spec references | 1.0 (explicit) |
| **CRD Inferred** | 200-299 | Flux MANAGES, ArgoCD ownership | 0.5-1.0 (evidence-based) |

**3. Evidence-Based Confidence Calculation**

```go
// Proven pattern from flux_helmrelease.go:297-406
func (e *Extractor) scoreRelationship(
    ctx context.Context,
    manager, managed *graph.ResourceIdentity,
    lookup ResourceLookup,
) (float64, []graph.EvidenceItem) {
    
    evidence := []graph.EvidenceItem{}
    totalWeight := 0.0
    earnedWeight := 0.0
    
    // Evidence 1: Label match (weight: 0.4)
    if hasLabelMatch(managed, manager.Name) {
        earnedWeight += 0.4
        evidence = append(evidence, EvidenceItem{
            Type: EvidenceTypeLabel,
            Weight: 0.4,
        })
    }
    
    // Evidence 2: Temporal proximity (weight: 0.3)
    lagMs := (managed.FirstSeen - manager.LastSeen) / 1_000_000
    if lagMs >= 0 && lagMs <= 30000 {
        proximityScore := 1.0 - (float64(lagMs) / 30000.0)
        earnedWeight += 0.3 * proximityScore
        evidence = append(evidence, EvidenceItem{
            Type: EvidenceTypeTemporal,
            Weight: 0.3 * proximityScore,
        })
    }
    
    // Evidence 3: Namespace co-location (weight: 0.2)
    if managed.Namespace == manager.Namespace {
        earnedWeight += 0.2
        evidence = append(evidence, EvidenceItem{
            Type: EvidenceTypeNamespace,
            Weight: 0.2,
        })
    }
    
    // Evidence 4: Reconcile events (weight: 0.1)
    // Check for recent reconcile events on manager
    
    totalWeight = 1.0
    confidence := earnedWeight / totalWeight
    
    return confidence, evidence
}
```

### B. Extractor Interface Extension (Proposed)

**Current interface is sufficient, but add helper base:**

```go
// BaseExtractor provides common functionality
type BaseExtractor struct {
    name     string
    priority int
    logger   *logging.Logger
}

func (b *BaseExtractor) Name() string     { return b.name }
func (b *BaseExtractor) Priority() int    { return b.priority }

// Helper: Create observed edge (100% confidence)
func (b *BaseExtractor) CreateObservedEdge(
    edgeType graph.EdgeType,
    fromUID, toUID string,
    props interface{},
) graph.Edge {
    propsJSON, _ := json.Marshal(props)
    return graph.Edge{
        Type:       edgeType,
        FromUID:    fromUID,
        ToUID:      toUID,
        Properties: propsJSON,
    }
}

// Helper: Create inferred edge with evidence
func (b *BaseExtractor) CreateInferredEdge(
    edgeType graph.EdgeType,
    fromUID, toUID string,
    confidence float64,
    evidence []graph.EvidenceItem,
) graph.Edge {
    props := graph.ManagesEdge{
        Confidence:      confidence,
        Evidence:        evidence,
        FirstObserved:   time.Now().UnixNano(),
        LastValidated:   time.Now().UnixNano(),
        ValidationState: graph.ValidationStateValid,
    }
    propsJSON, _ := json.Marshal(props)
    return graph.Edge{
        Type:       edgeType,
        FromUID:    fromUID,
        ToUID:      toUID,
        Properties: propsJSON,
    }
}
```

### C. Directory Structure (Proposed)

```
internal/graph/sync/extractors/
├── extractor.go              # Interface + BaseExtractor
├── registry.go               # Registry
├── lookup.go                 # ResourceLookup implementation
├── helpers.go                # Shared evidence scoring, label matching
│
├── native/
│   ├── ownership.go          # Complete ownership chains
│   ├── traffic.go            # Service→Pod, Ingress→Service
│   ├── config.go             # Pod→ConfigMap/Secret
│   ├── scheduling.go         # Pod→Node (already in builder.go)
│   ├── storage.go            # PVC→PV→StorageClass
│   └── rbac.go               # Move from extractors/rbac.go
│
├── flux/
│   ├── helmrelease.go        # Existing
│   ├── kustomization.go      # NEW: Kustomization support
│   ├── gitrepository.go      # NEW: GitRepository→HelmRelease/Kustomization
│   └── managed_resource.go   # Existing reverse lookup
│
├── argocd/
│   └── application.go        # NEW: ArgoCD Application→resources
│
├── certmanager/
│   ├── certificate.go        # NEW: Certificate→Secret, Certificate→Issuer
│   └── issuer.go             # NEW: Issuer validation
│
├── externalsecrets/
│   └── externalsecret.go     # NEW: ExternalSecret→Secret
│
└── gateway/
    ├── gateway.go            # NEW: Gateway→HTTPRoute
    └── httproute.go          # NEW: HTTPRoute→Service
```

---

## 4. CRD Support Strategy

### A. Tier 1 CRDs (Ownership & Traffic - Highest Impact)

#### **1. Flux HelmRelease** ✅ IMPLEMENTED
- **Status**: Complete with MANAGES and REFERENCES_SPEC
- **No action needed**

#### **2. Flux Kustomization** (NEW)

**Relationships:**
```
Kustomization
  ├─REFERENCES_SPEC→ GitRepository (spec.sourceRef)
  ├─REFERENCES_SPEC→ Secret (spec.decryption.secretRef)
  └─MANAGES (inferred)→ Deployment/Service/... (via kustomize labels)
```

**Evidence for MANAGES:**
- Label: `kustomize.toolkit.fluxcd.io/name` + `kustomize.toolkit.fluxcd.io/namespace` (100% confidence)
- Temporal: Created within 30s of reconcile (0.3 weight)
- Namespace: Same namespace (0.2 weight)

**Implementation:**
```go
type FluxKustomizationExtractor struct {
    BaseExtractor
}

func (e *FluxKustomizationExtractor) Matches(event models.Event) bool {
    return event.Resource.Group == "kustomize.toolkit.fluxcd.io" &&
           event.Resource.Kind == "Kustomization"
}

func (e *FluxKustomizationExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
    // 1. Extract sourceRef → GitRepository (REFERENCES_SPEC)
    // 2. Extract decryption.secretRef → Secret (REFERENCES_SPEC)
    // 3. Query for resources with kustomize labels (MANAGES)
}
```

#### **3. ArgoCD Application** (NEW - CRITICAL)

**Relationships:**
```
Application
  ├─REFERENCES_SPEC→ Secret (spec.source.*.passwordSecret, tokenSecret)
  └─MANAGES (inferred)→ Deployment/Service/... (via ArgoCD labels)
```

**Evidence for MANAGES:**
- Label: `argocd.argoproj.io/instance` matches Application.metadata.name (100% confidence)
- Namespace: `spec.destination.namespace` matches (0.3 weight)
- Temporal: Created during sync operation (0.2 weight)

**Special Considerations:**
- ArgoCD deploys to different namespace than where Application lives
- Need to track sync events (look for K8sEvents with reason="Synced")
- Application can manage cluster-scoped resources

**Implementation:**
```go
type ArgoCDApplicationExtractor struct {
    BaseExtractor
}

func (e *ArgoCDApplicationExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
    // 1. Extract source.*.passwordSecret, tokenSecret (REFERENCES_SPEC)
    // 2. Determine target namespace from spec.destination.namespace
    // 3. Query for resources with argocd.argoproj.io/instance label
    // 4. Score confidence based on label match + sync events
}
```

#### **4. Gateway API HTTPRoute** (NEW)

**Relationships:**
```
HTTPRoute
  ├─REFERENCES_SPEC→ Gateway (spec.parentRefs)
  └─REFERENCES_SPEC→ Service (spec.rules[].backendRefs)
```

**Implementation:**
```go
type HTTPRouteExtractor struct {
    BaseExtractor
}

func (e *HTTPRouteExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
    // 1. Extract parentRefs → Gateway (REFERENCES_SPEC)
    // 2. Extract backendRefs → Service (REFERENCES_SPEC)
    // All relationships are explicit (100% confidence)
}
```

#### **5. Gateway API Gateway** (NEW)

**Relationships:**
```
Gateway
  └─REFERENCES_SPEC→ GatewayClass (spec.gatewayClassName)
```

### B. Tier 2 CRDs (Configuration & Secrets)

#### **6. Cert-Manager Certificate** (NEW)

**Relationships:**
```
Certificate
  ├─REFERENCES_SPEC→ Issuer/ClusterIssuer (spec.issuerRef)
  └─CREATES_OBSERVED→ Secret (status.secretName)
```

**Evidence for CREATES_OBSERVED:**
- Temporal: Secret created within 60s of Certificate becoming Ready (0.8 weight)
- Ownership: Check if Secret has ownerReference to Certificate (1.0 if present)
- Annotation: Secret has `cert-manager.io/certificate-name` (0.9 weight)

**Implementation:**
```go
type CertificateExtractor struct {
    BaseExtractor
}

func (e *CertificateExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
    // 1. Extract issuerRef → Issuer/ClusterIssuer (REFERENCES_SPEC)
    // 2. Check if status.conditions[] has type=Ready, status=True
    // 3. Look up Secret by status.secretName
    // 4. Verify Secret created after Certificate (CREATES_OBSERVED with evidence)
}
```

#### **7. External Secrets ExternalSecret** (NEW)

**Relationships:**
```
ExternalSecret
  ├─REFERENCES_SPEC→ SecretStore/ClusterSecretStore (spec.secretStoreRef)
  └─CREATES_OBSERVED→ Secret (spec.target.name)
```

**Evidence for CREATES_OBSERVED:**
- Ownership: Secret should have ownerReference to ExternalSecret (1.0 if present)
- Temporal: Secret updated when ExternalSecret syncs (0.7 weight)
- Name match: spec.target.name matches Secret name (0.9 weight)

### C. Tier 3 CRDs (Advanced/Optional)

#### **8. KEDA ScaledObject**

**Relationships:**
```
ScaledObject
  └─REFERENCES_SPEC→ Deployment/StatefulSet (spec.scaleTargetRef)
```

#### **9. Crossplane Composite**

**Relationships:**
```
Composite
  └─MANAGES (ownership)→ ManagedResource (via ownerReferences)
```

---

## 5. Incremental Rollout Plan

### **Phase 1: Complete Native K8s Foundation** (Week 1-2)

**Goals:**
- Move native extractors from `builder.go` to `extractors/native/`
- Fix Service→Pod label matching
- Add missing native relationships

**Tasks:**

1. **Refactor existing native extractors** (2 days)
   ```
   - Move extractOwnershipRelationships → native/ownership.go
   - Move extractSchedulingRelationship → native/scheduling.go
   - Move extractVolumeRelationships → native/storage.go
   - Move extractServiceAccountRelationship → native/rbac.go
   - Keep in builder.go as thin wrappers for backward compatibility
   ```

2. **Fix Service→Pod SELECTS** (2 days)
   - **Problem**: Line 654 returns empty array - label matching not implemented
   - **Solution**: Store labels as indexed properties on ResourceIdentity
   - **Alternative (MVP)**: Use FalkorDB full-text search on labels JSON field
   
   ```cypher
   MATCH (s:ResourceIdentity {kind: 'Service', uid: $serviceUID})
   MATCH (p:ResourceIdentity {kind: 'Pod', namespace: $namespace})
   WHERE p.deleted = false
     AND p.labels CONTAINS '"app":"frontend"'  -- JSON substring match
   RETURN p.uid
   ```

3. **Add Ingress→Service** (1 day)
   ```go
   type IngressExtractor struct {
       BaseExtractor
   }
   
   func (e *IngressExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
       // Parse spec.rules[].http.paths[].backend.service
       // Create REFERENCES_SPEC edge to Service
   }
   ```

4. **Add NetworkPolicy→Pod** (1 day)
   ```go
   type NetworkPolicyExtractor struct {
       BaseExtractor
   }
   
   func (e *NetworkPolicyExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
       // Parse spec.podSelector
       // Find matching Pods (like Service selector)
       // Create ISOLATES edge
   }
   ```

**Deliverables:**
- [ ] Extractors moved to `native/` package
- [ ] Service→Pod label matching working
- [ ] Ingress→Service extraction
- [ ] NetworkPolicy→Pod extraction
- [ ] Unit tests for all extractors
- [ ] Integration test with kind cluster

**Testing Strategy:**
```bash
# Test with kind cluster
kind create cluster
kubectl apply -f tests/fixtures/native-resources.yaml
# Fixture includes: Service, Deployment, Pods, Ingress, NetworkPolicy

# Verify graph edges
redis-cli GRAPH.QUERY spectre "
  MATCH (s:ResourceIdentity {kind: 'Service'})-[:SELECTS]->(p:ResourceIdentity {kind: 'Pod'})
  RETURN s.name, p.name
"
```

---

### **Phase 2: Tier 1 CRDs - Flux Extensions** (Week 3-4)

**Goals:**
- Add Kustomization support
- Add GitRepository support
- Complete Flux ecosystem

**Tasks:**

1. **Flux GitRepository Extractor** (2 days)
   ```go
   type FluxGitRepositoryExtractor struct {
       BaseExtractor
   }
   
   func (e *FluxGitRepositoryExtractor) ExtractRelationships(...) ([]graph.Edge, error) {
       // Extract secretRef → Secret (REFERENCES_SPEC)
       // No MANAGES - GitRepository is a source, not a controller
   }
   ```

2. **Flux Kustomization Extractor** (3 days)
   - Implement as described in Section 4.A.2
   - Use same evidence scoring as HelmRelease
   - Test with real Kustomization deployments

3. **Bidirectional Flux Managed Resource Extractor** (1 day)
   - Extend existing `flux_managed_resource.go` to handle both HelmRelease and Kustomization
   - Check for both sets of labels

**Deliverables:**
- [ ] GitRepository extractor with REFERENCES_SPEC
- [ ] Kustomization extractor with MANAGES inference
- [ ] Updated FluxManagedResourceExtractor
- [ ] Integration tests with Flux controllers

**Testing:**
```yaml
# tests/fixtures/flux-kustomization.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: frontend
  namespace: flux-system
spec:
  sourceRef:
    kind: GitRepository
    name: app-repo
  path: ./deploy/frontend
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: production
  labels:
    kustomize.toolkit.fluxcd.io/name: frontend
    kustomize.toolkit.fluxcd.io/namespace: flux-system
```

---

### **Phase 3: Tier 1 CRDs - ArgoCD** (Week 5-6)

**Goals:**
- Add ArgoCD Application support
- Handle cross-namespace deployment
- Model sync events

**Tasks:**

1. **ArgoCD Application Extractor** (5 days)
   - Implement as described in Section 4.A.3
   - Handle cross-namespace deployments
   - Correlate with sync K8sEvents
   
2. **Special handling for ArgoCD labels** (2 days)
   - ArgoCD uses `argocd.argoproj.io/instance` label
   - May also use `app.kubernetes.io/instance` (configurable)
   - Need to query across all namespaces, not just Application namespace

**Implementation:**
```go
func (e *ArgoCDApplicationExtractor) extractManagedResources(...) ([]graph.Edge, error) {
    // Get target namespace from spec.destination.namespace
    targetNamespace := extractDestinationNamespace(spec)
    
    // Query all resources with ArgoCD label
    query := `
        MATCH (r:ResourceIdentity)
        WHERE r.labels CONTAINS '"argocd.argoproj.io/instance":"` + appName + `"'
          AND r.deleted = false
        RETURN r.uid
    `
    
    // Score each resource
    for _, resourceUID := range results {
        confidence, evidence := e.scoreArgoCDManagement(
            appResource,
            resourceUID,
            targetNamespace,
            lookup,
        )
        
        if confidence >= 0.7 {  // Higher threshold for cross-namespace
            edges = append(edges, e.CreateInferredEdge(...))
        }
    }
}
```

**Deliverables:**
- [ ] ArgoCD Application extractor
- [ ] Cross-namespace resource discovery
- [ ] Sync event correlation
- [ ] Integration tests with ArgoCD

---

### **Phase 4: Tier 1 CRDs - Gateway API** (Week 7)

**Goals:**
- Model modern Kubernetes traffic routing
- Support HTTPRoute→Service
- Support Gateway→GatewayClass

**Tasks:**

1. **Gateway Extractor** (1 day)
   - Extract gatewayClassName → GatewayClass
   - All relationships are explicit (100% confidence)

2. **HTTPRoute Extractor** (2 days)
   - Extract parentRefs → Gateway
   - Extract backendRefs → Service
   - Handle weight-based routing (store in edge properties)

**Deliverables:**
- [ ] Gateway extractor
- [ ] HTTPRoute extractor
- [ ] Integration tests with Gateway API

---

### **Phase 5: Tier 2 CRDs - Secrets & Certs** (Week 8-9)

**Goals:**
- Model secret lifecycle
- Track certificate issuance

**Tasks:**

1. **Cert-Manager Certificate Extractor** (3 days)
   - Implement as described in Section 4.B.6
   - Model temporal correlation for Secret creation
   - Check ownerReferences for strong evidence

2. **External Secrets Extractor** (2 days)
   - Implement as described in Section 4.B.7
   - Similar pattern to Cert-Manager

**Deliverables:**
- [ ] Certificate extractor with CREATES_OBSERVED
- [ ] ExternalSecret extractor
- [ ] Integration tests

---

### **Phase 6: Production Hardening** (Week 10-11)

**Goals:**
- Add revalidation logic
- Implement confidence decay
- Performance optimization

**Tasks:**

1. **Revalidation Background Job** (3 days)
   ```go
   type EdgeRevalidator struct {
       client   graph.Client
       interval time.Duration
   }
   
   func (r *EdgeRevalidator) Run(ctx context.Context) error {
       ticker := time.NewTicker(r.interval)
       for {
           select {
           case <-ticker.C:
               // Query MANAGES edges older than 1 hour
               // Re-check evidence (labels still present, resources still exist)
               // Update ValidationState and Confidence
           case <-ctx.Done():
               return nil
           }
       }
   }
   ```

2. **Confidence Decay** (2 days)
   - Edges older than 6 hours without revalidation: confidence *= 0.9
   - Edges older than 24 hours: confidence *= 0.7
   - Edges older than 7 days: mark as ValidationStateStale

3. **Performance Optimization** (2 days)
   - Add caching for label queries
   - Batch edge creation
   - Profile graph query performance

**Deliverables:**
- [ ] Revalidation background job
- [ ] Confidence decay logic
- [ ] Performance benchmarks

---

### **Phase 7: Additional CRDs** (Week 12+)

**Optional future work:**
- KEDA ScaledObject
- Crossplane Composites
- Prometheus ServiceMonitor/PodMonitor
- Istio VirtualService/DestinationRule

---

## 6. Testing & Validation Plan

### A. Unit Testing Strategy

**Test Categories:**

1. **Extractor Matching Tests**
   ```go
   func TestFluxKustomizationExtractor_Matches(t *testing.T) {
       tests := []struct {
           event    models.Event
           expected bool
       }{
           {
               event: models.Event{
                   Resource: models.ResourceMetadata{
                       Group: "kustomize.toolkit.fluxcd.io",
                       Kind:  "Kustomization",
                   },
               },
               expected: true,
           },
           // ... more cases
       }
   }
   ```

2. **Relationship Extraction Tests**
   ```go
   func TestFluxKustomizationExtractor_ExtractRelationships(t *testing.T) {
       // Mock ResourceLookup
       lookup := &MockResourceLookup{
           resources: map[string]*graph.ResourceIdentity{
               "git-repo-uid": {
                   UID:  "git-repo-uid",
                   Kind: "GitRepository",
                   Name: "app-repo",
               },
           },
       }
       
       extractor := NewFluxKustomizationExtractor()
       edges, err := extractor.ExtractRelationships(ctx, event, lookup)
       
       require.NoError(t, err)
       assert.Len(t, edges, 1)
       assert.Equal(t, graph.EdgeTypeReferencesSpec, edges[0].Type)
   }
   ```

3. **Evidence Scoring Tests**
   ```go
   func TestArgoCDApplicationExtractor_ScoreManagement(t *testing.T) {
       tests := []struct {
           name             string
           appResource      *graph.ResourceIdentity
           managedResource  *graph.ResourceIdentity
           expectedMin      float64
           expectedMax      float64
       }{
           {
               name: "Perfect label match",
               // App and resource with matching argocd.argoproj.io/instance
               expectedMin: 0.95,
               expectedMax: 1.0,
           },
           {
               name: "Label + namespace match",
               // ...
               expectedMin: 0.7,
               expectedMax: 0.8,
           },
       }
   }
   ```

### B. Integration Testing Strategy

**Test Environment:**
```yaml
# tests/e2e/crd-extractors/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
```

**Test Scenarios:**

1. **Flux Kustomization E2E**
   ```go
   func TestFluxKustomization_E2E(t *testing.T) {
       // 1. Apply Kustomization + GitRepository
       // 2. Wait for reconciliation
       // 3. Apply managed resources with Flux labels
       // 4. Verify MANAGES edges in graph
       // 5. Check confidence scores
       
       edges := queryGraphEdges(t, "Kustomization", "frontend", graph.EdgeTypeManages)
       assert.Len(t, edges, 3) // Deployment, Service, ConfigMap
       
       for _, edge := range edges {
           props := parseManagesEdge(edge)
           assert.GreaterOrEqual(t, props.Confidence, 0.95)
           assert.Contains(t, props.Evidence[0].Value, "kustomize.toolkit.fluxcd.io")
       }
   }
   ```

2. **ArgoCD Application E2E**
   ```go
   func TestArgoCDApplication_E2E(t *testing.T) {
       // 1. Install ArgoCD
       // 2. Create Application CR
       // 3. Wait for sync
       // 4. Verify MANAGES edges across namespaces
       
       edges := queryGraphEdges(t, "Application", "frontend", graph.EdgeTypeManages)
       assert.NotEmpty(t, edges)
       
       // Verify cross-namespace edges
       for _, edge := range edges {
           managedResource := getResource(t, edge.ToUID)
           assert.Equal(t, "production", managedResource.Namespace)
       }
   }
   ```

3. **Gateway API E2E**
   ```go
   func TestHTTPRoute_E2E(t *testing.T) {
       // 1. Install Gateway API CRDs
       // 2. Create Gateway + HTTPRoute
       // 3. Verify REFERENCES_SPEC edges
       
       edges := queryGraphEdges(t, "HTTPRoute", "frontend-route", graph.EdgeTypeReferencesSpec)
       assert.Len(t, edges, 2) // Gateway + Service
   }
   ```

### C. LLM-Safe Assertions

**Avoid nondeterministic tests:**

❌ **BAD:**
```go
// Temporal confidence varies based on timing
assert.Equal(t, 0.87, confidence)
```

✅ **GOOD:**
```go
// Range-based assertions
assert.GreaterOrEqual(t, confidence, 0.8)
assert.LessOrEqual(t, confidence, 1.0)
```

❌ **BAD:**
```go
// Exact timestamp matching
assert.Equal(t, expectedTimestamp, edge.FirstObserved)
```

✅ **GOOD:**
```go
// Time window assertions
assert.WithinDuration(t, time.Now(), time.Unix(0, edge.FirstObserved), 5*time.Second)
```

**Deterministic Evidence Checks:**
```go
func TestEvidence_Deterministic(t *testing.T) {
    evidence := extractEvidence(...)
    
    // Check evidence types present
    evidenceTypes := getEvidenceTypes(evidence)
    assert.Contains(t, evidenceTypes, graph.EvidenceTypeLabel)
    assert.Contains(t, evidenceTypes, graph.EvidenceTypeNamespace)
    
    // Check weights sum correctly
    totalWeight := sumEvidenceWeights(evidence)
    assert.InDelta(t, 1.0, totalWeight, 0.1)
}
```

### D. Test Data Fixtures

```
tests/
├── fixtures/
│   ├── native/
│   │   ├── service-pod-selector.yaml
│   │   ├── ingress-service.yaml
│   │   └── networkpolicy.yaml
│   │
│   ├── flux/
│   │   ├── kustomization-basic.yaml
│   │   ├── kustomization-with-managed.yaml
│   │   └── gitrepository.yaml
│   │
│   ├── argocd/
│   │   ├── application-basic.yaml
│   │   ├── application-cross-namespace.yaml
│   │   └── deployed-resources.yaml
│   │
│   └── gateway/
│       ├── gateway-httproute.yaml
│       └── services.yaml
│
└── e2e/
    ├── flux_test.go
    ├── argocd_test.go
    └── gateway_test.go
```

---

## 7. Open Questions & Assumptions

### A. Open Questions

1. **Label Storage Strategy**
   - **Question**: Should labels be denormalized into indexed properties on ResourceIdentity?
   - **Impact**: Critical for Service→Pod SELECTS performance
   - **Options**:
     - A) Store as JSON string, use substring search (current, slow)
     - B) Denormalize into individual properties (fast, schema inflation)
     - C) Use FalkorDB's map properties (middle ground)
   - **Recommendation**: Option C - store as FalkorDB map property with indexing

2. **Extractor Disablement**
   - **Question**: Should users be able to disable specific extractors?
   - **Impact**: Useful for clusters without certain CRDs installed
   - **Proposal**: Add environment variable `GRAPH_DISABLED_EXTRACTORS=argocd,keda`

3. **Revalidation Frequency**
   - **Question**: How often should MANAGES edges be revalidated?
   - **Current**: Not implemented
   - **Proposal**: Every 1 hour for edges <24h old, every 6 hours for older edges

4. **Cross-Namespace Query Performance**
   - **Question**: Will ArgoCD queries across all namespaces cause performance issues?
   - **Impact**: High for large clusters (1000+ resources)
   - **Mitigation**: Index on labels JSON field, limit query results

### B. Assumptions

1. **FalkorDB Capacity**
   - **Assumption**: FalkorDB can handle 10,000+ nodes with sub-second query times
   - **Risk**: Medium - needs load testing
   - **Validation**: Run performance benchmarks in Phase 6

2. **Label Stability**
   - **Assumption**: Flux/ArgoCD labels don't change after resource creation
   - **Risk**: Low - labels are set by controllers and rarely change
   - **Mitigation**: Revalidation logic will detect label changes

3. **OwnerReference Reliability**
   - **Assumption**: ownerReferences are always set correctly by controllers
   - **Risk**: Low - core Kubernetes guarantee
   - **Exception**: Some Helm charts don't set ownerReferences (use MANAGES instead)

4. **Temporal Correlation Window**
   - **Assumption**: Resources created by controllers appear within 30 seconds
   - **Risk**: Medium - may need adjustment for slow controllers
   - **Mitigation**: Make window configurable via environment variable

5. **Graph Rebuild Safety**
   - **Assumption**: Graph can be rebuilt from storage without data loss
   - **Risk**: Low - graph is a derived view
   - **Validation**: Already proven in existing implementation

### C. Impact of Assumptions

**If Label Storage Assumption Fails:**
- Service→Pod selectors will be slow or incomplete
- Fallback: Disable SELECTS extraction, rely on ownership chains

**If Cross-Namespace Query Performance Assumption Fails:**
- ArgoCD extractor may time out or return partial results
- Mitigation: Add pagination, limit to recently updated resources

**If Temporal Correlation Window Assumption Fails:**
- MANAGES confidence scores will be lower than expected
- Mitigation: Increase window to 60s, adjust weight distribution

---

## 8. Success Metrics

### A. Correctness Metrics

1. **Edge Accuracy**
   - Target: >95% of observed edges correct (verified by ownerReferences)
   - Target: >80% of inferred edges correct (verified by manual inspection)

2. **Confidence Calibration**
   - Target: Edges with confidence >0.9 should be correct >90% of the time
   - Target: Edges with confidence 0.5-0.7 should be correct >50% of the time

### B. Performance Metrics

1. **Extraction Latency**
   - Target: <100ms per extractor per event
   - Target: <1s total extraction time per event

2. **Query Performance**
   - Target: Root cause queries complete in <2s
   - Target: Blast radius queries complete in <3s

### C. Coverage Metrics

1. **CRD Coverage**
   - Phase 1: 100% native K8s resources
   - Phase 3: Flux + ArgoCD (covers 70% of platform teams)
   - Phase 5: +Gateway API, Cert-Manager, External Secrets (covers 90%)

2. **Relationship Coverage**
   - Target: >90% of runtime-affecting relationships modeled
   - Exclude: Observability-only relationships (Prometheus, Jaeger)

### D. LLM Integration Metrics

1. **RCA Accuracy**
   - Baseline: Current `find_root_cause` without CRD edges
   - Target: +30% improvement in identifying correct root cause
   - Measurement: Manual review of 100 real incidents

2. **Blast Radius Precision**
   - Target: <5% false positives in blast radius queries
   - Measurement: Compare predicted vs. actual affected resources

---

## 9. Next Actions

### Immediate (This Week)

1. **Review & Approve Plan**
   - Share with stakeholders
   - Gather feedback on prioritization
   - Adjust timeline based on team capacity

2. **Set Up Test Environment**
   - Create kind cluster with CRD operators
   - Install Flux, ArgoCD, Gateway API CRDs
   - Verify baseline functionality

3. **Spike: Label Storage**
   - Test FalkorDB map properties vs. JSON substring search
   - Measure query performance with 1000+ Pods
   - Make decision on implementation approach

### Week 1 (Phase 1 Start)

1. **Refactor Native Extractors**
   - Move code to `extractors/native/`
   - Create BaseExtractor helper
   - Write unit tests

2. **Fix Service→Pod SELECTS**
   - Implement chosen label storage strategy
   - Add integration test

### Monthly Checkpoints

- **End of Month 1**: Phase 1-2 complete (Native K8s + Flux)
- **End of Month 2**: Phase 3-4 complete (ArgoCD + Gateway API)
- **End of Month 3**: Phase 5-6 complete (Secrets + Hardening)

---

## 10. Appendix: Reference Implementation

### Example: Complete Kustomization Extractor

```go
package flux

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const (
	kustomizeAPIGroup    = "kustomize.toolkit.fluxcd.io"
	kustomizationKind    = "Kustomization"
	kustomizeNameLabel   = "kustomize.toolkit.fluxcd.io/name"
	kustomizeNsLabel     = "kustomize.toolkit.fluxcd.io/namespace"
)

type FluxKustomizationExtractor struct {
	logger *logging.Logger
}

func NewFluxKustomizationExtractor() *FluxKustomizationExtractor {
	return &FluxKustomizationExtractor{
		logger: logging.GetLogger("extractors.flux-kustomization"),
	}
}

func (e *FluxKustomizationExtractor) Name() string {
	return "flux-kustomization"
}

func (e *FluxKustomizationExtractor) Priority() int {
	return 100 // Same as HelmRelease
}

func (e *FluxKustomizationExtractor) Matches(event models.Event) bool {
	return event.Resource.Group == kustomizeAPIGroup &&
		event.Resource.Kind == kustomizationKind
}

func (e *FluxKustomizationExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var kustomization map[string]interface{}
	if err := json.Unmarshal(event.Data, &kustomization); err != nil {
		return nil, fmt.Errorf("failed to parse Kustomization: %w", err)
	}

	spec, ok := kustomization["spec"].(map[string]interface{})
	if !ok {
		return edges, nil
	}

	// 1. Extract sourceRef (REFERENCES_SPEC)
	if sourceRef, ok := spec["sourceRef"].(map[string]interface{}); ok {
		kind, _ := sourceRef["kind"].(string)
		name, _ := sourceRef["name"].(string)
		namespace, _ := sourceRef["namespace"].(string)

		if namespace == "" {
			namespace = event.Resource.Namespace
		}

		if kind != "" && name != "" {
			targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
			targetUID := ""
			if targetResource != nil {
				targetUID = targetResource.UID
			}

			edge := e.createReferencesSpecEdge(
				event.Resource.UID,
				targetUID,
				"spec.sourceRef",
				kind,
				name,
				namespace,
			)
			edges = append(edges, edge)
		}
	}

	// 2. Extract decryption secretRef (REFERENCES_SPEC)
	if decryption, ok := spec["decryption"].(map[string]interface{}); ok {
		if secretRef, ok := decryption["secretRef"].(map[string]interface{}); ok {
			name, _ := secretRef["name"].(string)
			if name != "" {
				targetResource, _ := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Secret", name)
				targetUID := ""
				if targetResource != nil {
					targetUID = targetResource.UID
				}

				edge := e.createReferencesSpecEdge(
					event.Resource.UID,
					targetUID,
					"spec.decryption.secretRef",
					"Secret",
					name,
					event.Resource.Namespace,
				)
				edges = append(edges, edge)
			}
		}
	}

	// 3. Extract managed resources (MANAGES)
	if event.Type != models.EventTypeDelete {
		managedEdges, err := e.extractManagedResources(ctx, event, spec, lookup)
		if err != nil {
			e.logger.Warn("Failed to extract managed resources: %v", err)
		} else {
			edges = append(edges, managedEdges...)
		}
	}

	return edges, nil
}

func (e *FluxKustomizationExtractor) extractManagedResources(
	ctx context.Context,
	event models.Event,
	spec map[string]interface{},
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	kustomizationName := event.Resource.Name
	kustomizationNamespace := event.Resource.Namespace

	// Determine target namespace
	targetNamespace := kustomizationNamespace
	if ns, ok := spec["targetNamespace"].(string); ok && ns != "" {
		targetNamespace = ns
	}

	// Query for resources with Kustomize labels
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE (r.namespace = $namespace OR r.namespace = "")
			  AND r.deleted = false
			  AND r.uid <> $kustomizationUID
			RETURN r
			LIMIT 500
		`,
		Parameters: map[string]interface{}{
			"namespace":         targetNamespace,
			"kustomizationUID": event.Resource.UID,
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query potential managed resources: %w", err)
	}

	for _, row := range result.Rows {
		candidateUID := extractors.ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		confidence, evidence := e.scoreManagementRelationship(
			ctx,
			event,
			candidateUID,
			kustomizationName,
			kustomizationNamespace,
			lookup,
		)

		if confidence >= 0.5 {
			edge := e.createManagesEdge(
				event.Resource.UID,
				candidateUID,
				confidence,
				evidence,
				event.Timestamp,
			)
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

func (e *FluxKustomizationExtractor) scoreManagementRelationship(
	ctx context.Context,
	kustomizationEvent models.Event,
	candidateUID string,
	kustomizationName string,
	kustomizationNamespace string,
	lookup extractors.ResourceLookup,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}

	candidate, err := lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		return 0.0, evidence
	}

	// Check for Kustomize labels (100% confidence if both present)
	if candidate.Labels != nil {
		if nameLabel, ok := candidate.Labels[kustomizeNameLabel]; ok && nameLabel == kustomizationName {
			if nsLabel, ok := candidate.Labels[kustomizeNsLabel]; ok && nsLabel == kustomizationNamespace {
				evidence = append(evidence, graph.EvidenceItem{
					Type:      graph.EvidenceTypeLabel,
					Value:     fmt.Sprintf("Kustomize labels match: %s=%s, %s=%s", kustomizeNameLabel, kustomizationName, kustomizeNsLabel, kustomizationNamespace),
					Weight:    1.0,
					Timestamp: time.Now().UnixNano(),
				})
				return 1.0, evidence
			}
		}
	}

	// Fallback to heuristic scoring
	totalWeight := 0.0
	earnedWeight := 0.0

	// Evidence 1: Name prefix match (0.4)
	totalWeight += 0.4
	if strings.HasPrefix(strings.ToLower(candidate.Name), strings.ToLower(kustomizationName)) {
		earnedWeight += 0.4
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("name prefix matches: %s", kustomizationName),
			Weight:    0.4,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 2: Temporal proximity (0.3)
	totalWeight += 0.3
	lagMs := (candidate.FirstSeen - kustomizationEvent.Timestamp) / 1_000_000
	if lagMs >= 0 && lagMs <= 30000 {
		proximityScore := 1.0 - (float64(lagMs) / 30000.0)
		earnedWeight += 0.3 * proximityScore
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeTemporal,
			Value:     fmt.Sprintf("created %dms after reconcile", lagMs),
			Weight:    0.3 * proximityScore,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 3: Namespace match (0.3)
	totalWeight += 0.3
	if candidate.Namespace == kustomizationEvent.Resource.Namespace {
		earnedWeight += 0.3
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     kustomizationEvent.Resource.Namespace,
			Weight:    0.3,
			Timestamp: time.Now().UnixNano(),
		})
	}

	confidence := 0.0
	if totalWeight > 0 {
		confidence = earnedWeight / totalWeight
	}

	return confidence, evidence
}

// Helper functions

func (e *FluxKustomizationExtractor) createReferencesSpecEdge(
	sourceUID, targetUID, fieldPath, kind, name, namespace string,
) graph.Edge {
	props := graph.ReferencesSpecEdge{
		FieldPath:    fieldPath,
		RefKind:      kind,
		RefName:      name,
		RefNamespace: namespace,
	}
	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeReferencesSpec,
		FromUID:    sourceUID,
		ToUID:      targetUID,
		Properties: propsJSON,
	}
}

func (e *FluxKustomizationExtractor) createManagesEdge(
	managerUID, managedUID string,
	confidence float64,
	evidence []graph.EvidenceItem,
	timestamp int64,
) graph.Edge {
	props := graph.ManagesEdge{
		Confidence:      confidence,
		Evidence:        evidence,
		FirstObserved:   timestamp,
		LastValidated:   timestamp,
		ValidationState: graph.ValidationStateValid,
	}
	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeManages,
		FromUID:    managerUID,
		ToUID:      managedUID,
		Properties: propsJSON,
	}
}
```

---

**End of Implementation Plan**

This plan provides a comprehensive roadmap for extending Spectre's graph reasoning capabilities while maintaining code quality, test coverage, and production readiness. The phased approach allows for incremental value delivery while managing risk.
