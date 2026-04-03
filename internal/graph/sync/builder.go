package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/graph/sync/extractors/argocd"
	"github.com/moolen/spectre/internal/graph/sync/extractors/certmanager"
	"github.com/moolen/spectre/internal/graph/sync/extractors/externalsecrets"
	"github.com/moolen/spectre/internal/graph/sync/extractors/gateway"
	"github.com/moolen/spectre/internal/graph/sync/extractors/native"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const (
	kindPod         = "Pod"
	kindReplicaSet  = "ReplicaSet"
	kindConfigMap   = "ConfigMap"
	kindClusterRole = "ClusterRole"
	kindDeployment  = "Deployment"
)

// graphBuilder implements the GraphBuilder interface
type graphBuilder struct {
	logger            *logging.Logger
	client            graph.Client // Graph client for querying existing nodes
	extractorRegistry *extractors.ExtractorRegistry
	// batchCache stores events from the current batch for change detection
	// Key: resource UID, Value: list of events for that resource (ordered by timestamp)
	batchCache map[string][]models.Event
	// stateCache provides LRU caching of recent resource states to avoid
	// database queries during change detection for UPDATE events
	stateCache *StateCache
	// labelIndex provides fast in-memory lookup of Pods by label selector
	// This eliminates graph queries when processing Service/Deployment selector edges
	labelIndex *LabelIndex
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder() GraphBuilder {
	// Create state cache for change detection optimization
	stateCache, _ := NewStateCache(DefaultStateCacheSize)

	return &graphBuilder{
		logger:     logging.GetLogger("graph.sync.builder"),
		batchCache: make(map[string][]models.Event),
		stateCache: stateCache,
		labelIndex: NewLabelIndex(),
	}
}

// NewGraphBuilderWithClient creates a new graph builder with client access
func NewGraphBuilderWithClient(client graph.Client) GraphBuilder {
	return NewGraphBuilderWithClientAndCacheSize(client, DefaultStateCacheSize)
}

// NewGraphBuilderWithClientAndCacheSize creates a new graph builder with custom cache size
func NewGraphBuilderWithClientAndCacheSize(client graph.Client, stateCacheSize int) GraphBuilder {
	// Create resource lookup adapter
	lookup := extractors.NewGraphClientLookup(client)

	// Create extractor registry
	registry := extractors.NewExtractorRegistry(lookup)

	// Register native K8s extractors (priority 50-99)
	registry.Register(native.NewServiceExtractor())         // Service→Pod SELECTS
	registry.Register(native.NewIngressExtractor())         // Ingress→Service REFERENCES_SPEC
	registry.Register(native.NewNetworkPolicyExtractor())   // NetworkPolicy→Pod SELECTS
	registry.Register(native.NewPodConfigSecretExtractor()) // Pod→ConfigMap/Secret REFERENCES_SPEC

	// Register CRD extractors (priority 100+)
	registry.Register(extractors.NewRBACExtractor())

	// Flux extractors (priority 10-15)
	registry.Register(extractors.NewFluxGitRepositoryExtractor())   // GitRepository→Secret
	registry.Register(extractors.NewFluxHelmReleaseExtractor())     // HelmRelease→Secret, MANAGES
	registry.Register(extractors.NewFluxKustomizationExtractor())   // Kustomization→GitRepository, MANAGES
	registry.Register(extractors.NewFluxManagedResourceExtractor()) // Reverse lookup for Flux-managed resources

	// ArgoCD extractors (priority 20)
	registry.Register(argocd.NewArgoCDApplicationExtractor()) // Application→Secret, MANAGES

	// Gateway API extractors (priority 100)
	registry.Register(gateway.NewGatewayExtractor())   // Gateway→GatewayClass REFERENCES_SPEC
	registry.Register(gateway.NewHTTPRouteExtractor()) // HTTPRoute→Gateway, HTTPRoute→Service REFERENCES_SPEC

	// Secrets & Certs extractors (priority 200)
	registry.Register(certmanager.NewCertificateExtractor())        // Certificate→Issuer/ClusterIssuer, Certificate→Secret
	registry.Register(externalsecrets.NewExternalSecretExtractor()) // ExternalSecret→SecretStore/ClusterSecretStore, ExternalSecret→Secret

	// Create state cache for change detection optimization
	cacheSize := stateCacheSize
	if cacheSize <= 0 {
		cacheSize = DefaultStateCacheSize
	}
	stateCache, err := NewStateCache(cacheSize)
	if err != nil {
		logging.GetLogger("graph.sync.builder").Warn("Failed to create state cache: %v (change detection will use database queries)", err)
	}

	return &graphBuilder{
		logger:            logging.GetLogger("graph.sync.builder"),
		client:            client,
		extractorRegistry: registry,
		batchCache:        make(map[string][]models.Event),
		stateCache:        stateCache,
		labelIndex:        NewLabelIndex(),
	}
}

// SetBatchCache sets the batch cache with events from the current batch
// This allows detectChanges to find previous events from the same batch
func (b *graphBuilder) SetBatchCache(events []models.Event) {
	b.batchCache = make(map[string][]models.Event)
	for _, event := range events {
		uid := event.Resource.UID
		b.batchCache[uid] = append(b.batchCache[uid], event)
	}
}

// ClearBatchCache clears the batch cache after processing is complete
func (b *graphBuilder) ClearBatchCache() {
	b.batchCache = make(map[string][]models.Event)
}

// GetStateCacheStats returns state cache statistics (hits, misses, size)
// Returns (0, 0, 0) if state cache is not enabled
func (b *graphBuilder) GetStateCacheStats() (hits, misses int64, size int) {
	if b.stateCache == nil {
		return 0, 0, 0
	}
	return b.stateCache.GetStats()
}

// GetLabelIndex returns the label index for Pod selector lookups
// Returns nil if label index is not enabled
func (b *graphBuilder) GetLabelIndex() *LabelIndex {
	return b.labelIndex
}

// GetLabelIndexStats returns label index statistics (hits, misses, namespaces, resources)
// Returns (0, 0, 0, 0) if label index is not enabled
func (b *graphBuilder) GetLabelIndexStats() (hits, misses int64, namespaces, resources int) {
	if b.labelIndex == nil {
		return 0, 0, 0, 0
	}
	return b.labelIndex.GetStats()
}

// BuildResourceNodes creates just the resource and event nodes (Phase 1 of two-phase processing)
// This method creates the ResourceIdentity and ChangeEvent/K8sEvent nodes along with their
// immediate structural edges (CHANGED, EMITTED_EVENT). It does NOT extract relationship edges.
func (b *graphBuilder) BuildResourceNodes(event models.Event) (*GraphUpdate, error) {
	update := &GraphUpdate{
		SourceEventID: event.ID,
		Timestamp:     time.Now(),
		ResourceNodes: []graph.ResourceIdentity{},
		EventNodes:    []graph.ChangeEvent{},
		K8sEventNodes: []graph.K8sEvent{},
		Edges:         []graph.Edge{},
	}

	// Create ResourceIdentity node
	resourceNode := b.buildResourceIdentityNode(event)
	update.ResourceNodes = append(update.ResourceNodes, resourceNode)

	// Create ChangeEvent node (unless this is a K8s Event object)
	if event.Resource.Kind != "Event" {
		changeEventNode := b.buildChangeEventNode(event)
		update.EventNodes = append(update.EventNodes, changeEventNode)

		// Create CHANGED edge (Resource → ChangeEvent)
		changedEdge := b.buildChangedEdge(event.Resource.UID, event.ID)
		update.Edges = append(update.Edges, changedEdge)
	} else {
		// This is a K8s Event object - create K8sEvent node
		k8sEventNode, err := b.buildK8sEventNode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to build k8s event: %w", err)
		}
		update.K8sEventNodes = append(update.K8sEventNodes, k8sEventNode)

		// Create EMITTED_EVENT edge if InvolvedObjectUID is present
		if event.Resource.InvolvedObjectUID != "" {
			// Extract involvedObject metadata and create ResourceIdentity node if possible
			// This ensures the target node exists for the edge, even if we haven't seen
			// a CREATE/UPDATE event for the resource yet (common for long-lived resources)
			if involvedResource := b.extractInvolvedObjectMetadata(event); involvedResource != nil {
				update.ResourceNodes = append(update.ResourceNodes, *involvedResource)
			}

			emittedEdge := b.buildEmittedEventEdge(event.Resource.InvolvedObjectUID, event.ID)
			update.Edges = append(update.Edges, emittedEdge)
		}
	}

	return update, nil
}

// BuildRelationshipEdges extracts relationship edges only (Phase 2 of two-phase processing)
// This method runs extractors to create edges between resources. It assumes all resources
// in the current batch have already been written to the graph by Phase 1.
func (b *graphBuilder) BuildRelationshipEdges(ctx context.Context, event models.Event) (*GraphUpdate, error) {
	update := &GraphUpdate{
		SourceEventID: event.ID,
		Timestamp:     time.Now(),
		Edges:         []graph.Edge{},
	}

	// Extract relationships from the resource data
	relationships, err := b.ExtractRelationships(ctx, event)
	if err != nil {
		b.logger.Warn("Failed to extract relationships from event %s: %v", event.ID, err)
		return update, nil // Return empty update rather than error
	}

	update.Edges = append(update.Edges, relationships...)
	return update, nil
}

// BuildFromEvent creates graph nodes/edges from a Spectre event (combines both phases)
// This method is kept for backward compatibility and single-event processing scenarios.
// For batch processing, use BuildResourceNodes + BuildRelationshipEdges separately.
func (b *graphBuilder) BuildFromEvent(ctx context.Context, event models.Event) (*GraphUpdate, error) {
	// Phase 1: Build resource nodes
	nodeUpdate, err := b.BuildResourceNodes(event)
	if err != nil {
		return nil, err
	}

	// Phase 2: Extract relationships
	edgeUpdate, err := b.BuildRelationshipEdges(ctx, event)
	if err != nil {
		return nil, err
	}

	// Combine both updates
	nodeUpdate.Edges = append(nodeUpdate.Edges, edgeUpdate.Edges...)
	return nodeUpdate, nil
}

// BuildFromBatch processes multiple events and returns graph updates
func (b *graphBuilder) BuildFromBatch(ctx context.Context, events []models.Event) ([]*GraphUpdate, error) {
	updates := make([]*GraphUpdate, 0, len(events))

	for _, event := range events {
		update, err := b.BuildFromEvent(ctx, event)
		if err != nil {
			b.logger.Warn("Failed to build update for event %s: %v", event.ID, err)
			continue
		}
		updates = append(updates, update)
	}

	return updates, nil
}
