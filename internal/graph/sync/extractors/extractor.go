package extractors

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
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

// BaseExtractor provides common functionality for all extractors
type BaseExtractor struct {
	name     string
	priority int
	logger   *logging.Logger
}

// NewBaseExtractor creates a new BaseExtractor
func NewBaseExtractor(name string, priority int) *BaseExtractor {
	return &BaseExtractor{
		name:     name,
		priority: priority,
		logger:   logging.GetLogger("extractors." + name),
	}
}

// Name returns the extractor identifier
func (b *BaseExtractor) Name() string {
	return b.name
}

// Priority returns extraction priority
func (b *BaseExtractor) Priority() int {
	return b.priority
}

// Logger returns the logger instance
func (b *BaseExtractor) Logger() *logging.Logger {
	return b.logger
}

// CreateObservedEdge creates an observed edge (100% confidence, explicit relationship)
// Returns a zero-value edge if toUID is empty (caller should check and skip)
func (b *BaseExtractor) CreateObservedEdge(
	edgeType graph.EdgeType,
	fromUID, toUID string,
	props interface{},
) graph.Edge {
	if toUID == "" {
		b.Logger().Debug("Skipping edge creation: toUID is empty (edgeType=%v, fromUID=%s)", edgeType, fromUID)
		return graph.Edge{} // Return zero-value edge
	}

	propsJSON, _ := json.Marshal(props)
	return graph.Edge{
		Type:       edgeType,
		FromUID:    fromUID,
		ToUID:      toUID,
		Properties: propsJSON,
	}
}

// CreateInferredEdge creates an inferred edge with evidence and confidence
// Returns a zero-value edge if toUID is empty (caller should check and skip)
func (b *BaseExtractor) CreateInferredEdge(
	edgeType graph.EdgeType,
	fromUID, toUID string,
	confidence float64,
	evidence []graph.EvidenceItem,
) graph.Edge {
	if toUID == "" {
		b.Logger().Debug("Skipping edge creation: toUID is empty (edgeType=%v, fromUID=%s, confidence=%f)", edgeType, fromUID, confidence)
		return graph.Edge{} // Return zero-value edge
	}

	now := time.Now().UnixNano()
	props := graph.ManagesEdge{
		Confidence:      confidence,
		Evidence:        evidence,
		FirstObserved:   now,
		LastValidated:   now,
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

// CreateReferencesSpecEdge creates a REFERENCES_SPEC edge for explicit spec references
// Returns a zero-value edge if targetUID is empty (caller should check and skip)
func (b *BaseExtractor) CreateReferencesSpecEdge(
	sourceUID, targetUID, fieldPath, kind, name, namespace string,
) graph.Edge {
	if targetUID == "" {
		b.Logger().Debug("Skipping REFERENCES_SPEC edge: targetUID is empty (sourceUID=%s, fieldPath=%s, refKind=%s, refName=%s, refNamespace=%s)", sourceUID, fieldPath, kind, name, namespace)
		return graph.Edge{} // Return zero-value edge
	}

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

// IsValidEdge checks if an edge has a non-empty ToUID
// Use this to filter out edges that were skipped due to missing target resources
func IsValidEdge(edge graph.Edge) bool {
	return edge.ToUID != ""
}
