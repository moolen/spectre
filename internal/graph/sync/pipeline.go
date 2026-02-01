package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// pipeline implements the Pipeline interface
type pipeline struct {
	config    PipelineConfig
	client    graph.Client
	schema    *graph.Schema
	builder   GraphBuilder
	causality CausalityEngine
	retention RetentionManager
	logger    *logging.Logger

	// Statistics (atomic counters)
	stats     PipelineStats
	statsLock sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPipeline creates a new sync pipeline
func NewPipeline(config PipelineConfig, client graph.Client) Pipeline {
	p := &pipeline{
		config:    config,
		client:    client,
		schema:    graph.NewSchema(client),
		builder:   NewGraphBuilderWithClientAndCacheSize(client, config.StateCacheSize),
		causality: NewCausalityEngine(config.CausalityMaxLag, config.CausalityMinConfidence),
		retention: NewRetentionManager(client, config.RetentionWindow),
		logger:    logging.GetLogger("graph.sync.pipeline"),
		stats:     PipelineStats{},
	}

	return p
}

// Start begins the sync pipeline
func (p *pipeline) Start(ctx context.Context) error {
	p.logger.Info("Starting graph sync pipeline")

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Initialize graph schema
	if err := p.schema.Initialize(p.ctx); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Bootstrap label index from existing graph data
	if err := p.bootstrapLabelIndex(p.ctx); err != nil {
		p.logger.Warn("Failed to bootstrap label index: %v (selector lookups will use graph queries initially)", err)
	}

	// Start periodic retention cleanup
	if p.config.RetentionWindow > 0 {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			cleanupInterval := 1 * time.Hour // Run cleanup every hour
			ticker := time.NewTicker(cleanupInterval)
			defer ticker.Stop()

			for {
				select {
				case <-p.ctx.Done():
					p.logger.Info("Stopping periodic cleanup")
					return
				case <-ticker.C:
					if err := p.retention.Cleanup(p.ctx); err != nil {
						p.logger.Error("Retention cleanup failed: %v", err)
					}
				}
			}
		}()
	}

	p.logger.Info("Graph sync pipeline started")
	return nil
}

// Stop gracefully stops the sync pipeline
func (p *pipeline) Stop(ctx context.Context) error {
	p.logger.Info("Stopping graph sync pipeline")

	if p.cancel != nil {
		p.cancel()
	}

	// Wait for background tasks with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Graph sync pipeline stopped gracefully")
	case <-ctx.Done():
		p.logger.Warn("Graph sync pipeline stop timed out")
		return ctx.Err()
	}

	return nil
}

// ProcessEvent processes a single event
func (p *pipeline) ProcessEvent(ctx context.Context, event models.Event) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.EventsProcessed, 1)
		p.updateProcessingRate()
	}()

	// Build graph update from event
	update, err := p.builder.BuildFromEvent(ctx, event)
	if err != nil {
		atomic.AddInt64(&p.stats.Errors, 1)
		return fmt.Errorf("failed to build graph update: %w", err)
	}

	// Apply update to graph
	if err := p.applyGraphUpdate(ctx, update); err != nil {
		atomic.AddInt64(&p.stats.Errors, 1)
		return fmt.Errorf("failed to apply graph update: %w", err)
	}

	// Update stats
	p.statsLock.Lock()
	p.stats.LastEventTime = time.Unix(0, event.Timestamp)
	p.stats.LastSyncTime = time.Now()
	p.stats.SyncLagMs = time.Since(time.Unix(0, event.Timestamp)).Milliseconds()
	p.statsLock.Unlock()

	p.logger.Debug("Processed event %s in %v", event.ID, time.Since(start))
	return nil
}

// ProcessBatch processes a batch of events using two-phase processing
// Phase 1: Write all resource nodes to graph
// Phase 2: Extract and write all relationship edges
// This ensures all resources in the batch exist before relationship extraction,
// eliminating race conditions where edges fail because target resources haven't been written yet.
func (p *pipeline) ProcessBatch(ctx context.Context, events []models.Event) error {
	if len(events) == 0 {
		return nil
	}

	start := time.Now()
	p.logger.Info("Processing batch of %d events (two-phase)", len(events))

	// Set batch cache for change detection
	// This allows detectChanges to find previous events from the same batch
	p.builder.SetBatchCache(events)
	defer p.builder.ClearBatchCache()

	// PHASE 1: Create all resource nodes
	phase1Start := time.Now()
	p.logger.Debug("Phase 1: Creating %d resource nodes", len(events))

	nodeUpdates := make([]*GraphUpdate, 0, len(events))
	for _, event := range events {
		update, err := p.builder.BuildResourceNodes(event)
		if err != nil {
			p.logger.Warn("Failed to build nodes for event %s: %v", event.ID, err)
			atomic.AddInt64(&p.stats.Errors, 1)
			continue
		}
		nodeUpdates = append(nodeUpdates, update)
	}

	// Apply all node updates using batch queries
	nodesCreated, err := p.applyBatchedNodeUpdates(ctx, nodeUpdates)
	if err != nil {
		p.logger.Warn("Batch node update failed, falling back to individual: %v", err)
		// Fallback to individual updates
		nodesCreated = 0
		for _, update := range nodeUpdates {
			if err := p.applyGraphUpdate(ctx, update); err != nil {
				p.logger.Warn("Failed to apply node update for event %s: %v", update.SourceEventID, err)
				atomic.AddInt64(&p.stats.Errors, 1)
				continue
			}
			nodesCreated++
		}
	}

	phase1Duration := time.Since(phase1Start)
	p.logger.Info("Phase 1 complete: Created %d resource nodes from %d events in %v", nodesCreated, len(events), phase1Duration)

	// PHASE 2: Extract all relationship edges
	phase2Start := time.Now()
	p.logger.Debug("Phase 2: Extracting relationships for %d events", len(events))

	edgeUpdates := make([]*GraphUpdate, 0, len(events))
	totalEdges := 0
	for _, event := range events {
		update, err := p.builder.BuildRelationshipEdges(ctx, event)
		if err != nil {
			p.logger.Warn("Failed to extract relationships for event %s: %v", event.ID, err)
			atomic.AddInt64(&p.stats.Errors, 1)
			continue
		}
		totalEdges += len(update.Edges)
		edgeUpdates = append(edgeUpdates, update)
	}

	// Apply all edge updates using batch queries
	edgesCreated, err := p.applyBatchedEdgeUpdates(ctx, edgeUpdates)
	if err != nil {
		p.logger.Warn("Batch edge update failed: %v", err)
	}

	phase2Duration := time.Since(phase2Start)
	p.logger.Info("Phase 2 complete: Created %d/%d edges in %v", edgesCreated, totalEdges, phase2Duration)

	// PHASE 3: Infer causality (existing logic)
	if p.config.EnableCausality && len(events) > 1 {
		causalityStart := time.Now()
		if err := p.inferCausality(ctx, events); err != nil {
			p.logger.Warn("Failed to infer causality: %v", err)
		} else {
			p.logger.Debug("Causality inference complete in %v", time.Since(causalityStart))
		}
	}

	// Update stats
	atomic.AddInt64(&p.stats.EventsProcessed, int64(len(events)))
	p.statsLock.Lock()
	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		p.stats.LastEventTime = time.Unix(0, lastEvent.Timestamp)
		p.stats.LastSyncTime = time.Now()
		p.stats.SyncLagMs = time.Since(time.Unix(0, lastEvent.Timestamp)).Milliseconds()
	}
	p.statsLock.Unlock()
	p.updateProcessingRate()

	totalDuration := time.Since(start)
	p.logger.Info("Batch complete: %d events processed in %v (Phase1: %v, Phase2: %v)",
		len(events), totalDuration, phase1Duration, phase2Duration)
	return nil
}

// GetStats returns pipeline statistics
func (p *pipeline) GetStats() PipelineStats {
	p.statsLock.RLock()
	defer p.statsLock.RUnlock()
	return p.stats
}

// applyGraphUpdate applies a graph update to the graph database
func (p *pipeline) applyGraphUpdate(ctx context.Context, update *GraphUpdate) error {
	// Upsert ResourceIdentity nodes
	for _, resource := range update.ResourceNodes {
		query := graph.UpsertResourceIdentityQuery(resource)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to upsert resource %s: %w", resource.UID, err)
		}
		atomic.AddInt64(&p.stats.NodesCreated, 1)
		if resource.Deleted {
			p.logger.Debug("Wrote ResourceIdentity node (DELETED): %s/%s deleted=%v deletedAt=%d (stats: %d nodes created, %d props set)",
				resource.Kind, resource.Name, resource.Deleted, resource.DeletedAt, result.Stats.NodesCreated, result.Stats.PropertiesSet)
		} else {
			p.logger.Debug("Wrote ResourceIdentity node: %s (stats: %d nodes created, %d props set)",
				resource.UID, result.Stats.NodesCreated, result.Stats.PropertiesSet)
		}
	}

	// Create ChangeEvent nodes
	for _, event := range update.EventNodes {
		query := graph.CreateChangeEventQuery(event)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to create change event %s: %w", event.ID, err)
		}
		atomic.AddInt64(&p.stats.NodesCreated, 1)
		p.logger.Debug("Wrote ChangeEvent node: %s (stats: %d nodes created, %d props set)",
			event.ID, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	// Create K8sEvent nodes
	for _, k8sEvent := range update.K8sEventNodes {
		query := graph.CreateK8sEventQuery(k8sEvent)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to create k8s event %s: %w", k8sEvent.ID, err)
		}
		atomic.AddInt64(&p.stats.NodesCreated, 1)
		p.logger.Debug("Wrote K8sEvent node: %s (stats: %d nodes created, %d props set)",
			k8sEvent.ID, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	// Create edges
	for _, edge := range update.Edges {
		if err := p.createEdge(ctx, edge); err != nil {
			p.logger.Warn("Failed to create edge %s (%s -> %s): %v",
				edge.Type, edge.FromUID, edge.ToUID, err)
			continue
		}
		atomic.AddInt64(&p.stats.EdgesCreated, 1)
		p.logger.Debug("Wrote edge: %s (%s -> %s)", edge.Type, edge.FromUID, edge.ToUID)
	}

	return nil
}

// createEdge creates an edge in the graph
func (p *pipeline) createEdge(ctx context.Context, edge graph.Edge) error {
	var query graph.GraphQuery

	switch edge.Type {
	case graph.EdgeTypeOwns:
		var props graph.OwnsEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateOwnsEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeChanged:
		var props graph.ChangedEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateChangedEdgeQuery(edge.FromUID, edge.ToUID, props.SequenceNumber)

	case graph.EdgeTypeEmittedEvent:
		query = graph.CreateEmittedEventEdgeQuery(edge.FromUID, edge.ToUID)

	case graph.EdgeTypeScheduledOn:
		var props graph.ScheduledOnEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateScheduledOnEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeUsesServiceAccount:
		query = graph.CreateUsesServiceAccountEdgeQuery(edge.FromUID, edge.ToUID)

	// RBAC edge types
	case graph.EdgeTypeBindsRole:
		var props graph.BindsRoleEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateBindsRoleEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeGrantsTo:
		var props graph.GrantsToEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateGrantsToEdgeQuery(edge.FromUID, edge.ToUID, props)

	// Custom Resource edge types
	case graph.EdgeTypeReferencesSpec:
		var props graph.ReferencesSpecEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateReferencesSpecEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeManages:
		var props graph.ManagesEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateManagesEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeCreatesObserved:
		var props graph.CreatesObservedEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateCreatesObservedEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeSelects:
		var props graph.SelectsEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateSelectsEdgeQuery(edge.FromUID, edge.ToUID, props)

	case graph.EdgeTypeMounts:
		var props graph.MountsEdge
		if err := json.Unmarshal(edge.Properties, &props); err != nil {
			return err
		}
		query = graph.CreateMountsEdgeQuery(edge.FromUID, edge.ToUID, props)

	default:
		return fmt.Errorf("unsupported edge type: %s", edge.Type)
	}

	_, err := p.client.ExecuteQuery(ctx, query)
	return err
}

// inferCausality infers causal relationships between events
func (p *pipeline) inferCausality(ctx context.Context, events []models.Event) error {
	links, err := p.causality.InferCausality(ctx, events)
	if err != nil {
		return err
	}

	if len(links) == 0 {
		return nil
	}

	p.logger.Debug("Creating %d causality links", len(links))

	// Create TRIGGERED_BY edges
	for _, link := range links {
		props := graph.TriggeredByEdge{
			Confidence: link.Confidence,
			LagMs:      link.LagMs,
			Reason:     link.Reason,
		}

		query := graph.CreateTriggeredByEdgeQuery(link.EffectEventID, link.CauseEventID, props)
		if _, err := p.client.ExecuteQuery(ctx, query); err != nil {
			p.logger.Warn("Failed to create causality link (%s -> %s): %v",
				link.CauseEventID, link.EffectEventID, err)
			continue
		}

		atomic.AddInt64(&p.stats.CausalityLinksFound, 1)
		atomic.AddInt64(&p.stats.EdgesCreated, 1)
	}

	return nil
}

// updateProcessingRate updates the processing rate statistic
func (p *pipeline) updateProcessingRate() {
	p.statsLock.Lock()
	defer p.statsLock.Unlock()

	if p.stats.LastEventTime.IsZero() {
		p.stats.ProcessingRate = 0
		return
	}

	// Calculate events per second based on time window
	duration := time.Since(p.stats.LastSyncTime)
	if duration > 0 {
		p.stats.ProcessingRate = float64(p.stats.EventsProcessed) / duration.Seconds()
	}
}

// applyBatchedNodeUpdates applies multiple graph updates using batch queries.
// This reduces N individual MERGE queries to a small number of batched operations.
func (p *pipeline) applyBatchedNodeUpdates(ctx context.Context, updates []*GraphUpdate) (nodesCreated int, err error) {
	// Collect all nodes across updates, separating deletions for special handling
	var nonDeletedResources []graph.ResourceIdentity
	var deletedResources []graph.ResourceIdentity
	var allChangeEvents []graph.ChangeEvent
	var allK8sEvents []graph.K8sEvent

	for _, update := range updates {
		for _, resource := range update.ResourceNodes {
			if resource.Deleted {
				deletedResources = append(deletedResources, resource)
			} else {
				nonDeletedResources = append(nonDeletedResources, resource)
			}
		}
		allChangeEvents = append(allChangeEvents, update.EventNodes...)
		allK8sEvents = append(allK8sEvents, update.K8sEventNodes...)
	}

	// Batch upsert non-deleted ResourceIdentity nodes
	if len(nonDeletedResources) > 0 {
		query := graph.BatchUpsertResourceIdentitiesQuery(nonDeletedResources)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch upsert resources: %w", err)
		}
		nodesCreated += len(nonDeletedResources)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(nonDeletedResources)))
		p.logger.Debug("Batch upserted %d ResourceIdentity nodes (stats: %d nodes created, %d props set)",
			len(nonDeletedResources), result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	// Handle deletions individually (they have special logic to prevent un-deletion)
	for _, resource := range deletedResources {
		query := graph.UpsertResourceIdentityQuery(resource)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			p.logger.Warn("Failed to upsert deleted resource %s: %v", resource.UID, err)
			continue
		}
		nodesCreated++
		atomic.AddInt64(&p.stats.NodesCreated, 1)
		p.logger.Debug("Wrote ResourceIdentity node (DELETED): %s/%s deleted=%v deletedAt=%d (stats: %d nodes created, %d props set)",
			resource.Kind, resource.Name, resource.Deleted, resource.DeletedAt, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	// Batch create ChangeEvent nodes
	if len(allChangeEvents) > 0 {
		query := graph.BatchCreateChangeEventsQuery(allChangeEvents)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch create change events: %w", err)
		}
		nodesCreated += len(allChangeEvents)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(allChangeEvents)))
		p.logger.Debug("Batch created %d ChangeEvent nodes (stats: %d nodes created, %d props set)",
			len(allChangeEvents), result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	// Batch create K8sEvent nodes
	if len(allK8sEvents) > 0 {
		query := graph.BatchCreateK8sEventsQuery(allK8sEvents)
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch create K8s events: %w", err)
		}
		nodesCreated += len(allK8sEvents)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(allK8sEvents)))
		p.logger.Debug("Batch created %d K8sEvent nodes (stats: %d nodes created, %d props set)",
			len(allK8sEvents), result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	return nodesCreated, nil
}

// applyBatchedEdgeUpdates applies multiple edge updates using batch queries.
// Edges are grouped by type and then batched together.
func (p *pipeline) applyBatchedEdgeUpdates(ctx context.Context, updates []*GraphUpdate) (edgesCreated int, err error) {
	// Group edges by type
	edgesByType := make(map[graph.EdgeType][]graph.Edge)
	for _, update := range updates {
		for _, edge := range update.Edges {
			edgesByType[edge.Type] = append(edgesByType[edge.Type], edge)
		}
	}

	// Apply batched edges for each type
	for edgeType, edges := range edgesByType {
		if len(edges) == 0 {
			continue
		}

		batchParams := make([]graph.BatchEdgeParams, len(edges))
		for i, edge := range edges {
			var props map[string]interface{}
			if edge.Properties != nil {
				json.Unmarshal(edge.Properties, &props)
			}
			if props == nil {
				props = make(map[string]interface{})
			}
			batchParams[i] = graph.BatchEdgeParams{
				FromUID:    edge.FromUID,
				ToUID:      edge.ToUID,
				Properties: props,
			}
		}

		var query graph.GraphQuery
		switch edgeType {
		case graph.EdgeTypeOwns:
			query = graph.BatchCreateOwnsEdgesQuery(batchParams)
		case graph.EdgeTypeChanged:
			query = graph.BatchCreateChangedEdgesQuery(batchParams)
		case graph.EdgeTypeSelects:
			query = graph.BatchCreateSelectsEdgesQuery(batchParams)
		case graph.EdgeTypeScheduledOn:
			query = graph.BatchCreateScheduledOnEdgesQuery(batchParams)
		case graph.EdgeTypeMounts:
			query = graph.BatchCreateMountsEdgesQuery(batchParams)
		case graph.EdgeTypeReferencesSpec:
			query = graph.BatchCreateReferencesSpecEdgesQuery(batchParams)
		case graph.EdgeTypeManages:
			query = graph.BatchCreateManagesEdgesQuery(batchParams)
		case graph.EdgeTypeEmittedEvent:
			query = graph.BatchCreateEmittedEventEdgesQuery(batchParams)
		case graph.EdgeTypeUsesServiceAccount:
			query = graph.BatchCreateUsesServiceAccountEdgesQuery(batchParams)
		case graph.EdgeTypeBindsRole:
			query = graph.BatchCreateBindsRoleEdgesQuery(batchParams)
		case graph.EdgeTypeGrantsTo:
			query = graph.BatchCreateGrantsToEdgesQuery(batchParams)
		case graph.EdgeTypeCreatesObserved:
			query = graph.BatchCreateCreatesObservedEdgesQuery(batchParams)
		default:
			// Fall back to individual queries for unsupported edge types
			for _, edge := range edges {
				if err := p.createEdge(ctx, edge); err != nil {
					p.logger.Warn("Failed to create edge %s (%s -> %s): %v",
						edge.Type, edge.FromUID, edge.ToUID, err)
					continue
				}
				edgesCreated++
				atomic.AddInt64(&p.stats.EdgesCreated, 1)
			}
			continue
		}

		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			p.logger.Warn("Failed to batch create %s edges: %v", edgeType, err)
			// Fall back to individual queries on batch failure
			for _, edge := range edges {
				if err := p.createEdge(ctx, edge); err != nil {
					p.logger.Warn("Failed to create edge %s (%s -> %s): %v",
						edge.Type, edge.FromUID, edge.ToUID, err)
					continue
				}
				edgesCreated++
				atomic.AddInt64(&p.stats.EdgesCreated, 1)
			}
			continue
		}

		edgesCreated += len(edges)
		atomic.AddInt64(&p.stats.EdgesCreated, int64(len(edges)))
		p.logger.Debug("Batch created %d %s edges (stats: %d rels created)",
			len(edges), edgeType, result.Stats.RelationshipsCreated)
	}

	return edgesCreated, nil
}

// bootstrapLabelIndex populates the label index from existing Pod data in the graph.
// This is called during pipeline startup to enable fast selector lookups immediately.
func (p *pipeline) bootstrapLabelIndex(ctx context.Context) error {
	labelIndex := p.builder.GetLabelIndex()
	if labelIndex == nil {
		p.logger.Debug("Label index not enabled, skipping bootstrap")
		return nil
	}

	p.logger.Info("Bootstrapping label index from graph...")

	// Query all non-deleted Pods from the graph
	query := graph.GraphQuery{
		Query: `
			MATCH (p:ResourceIdentity {kind: 'Pod'})
			WHERE NOT p.deleted
			RETURN p.namespace, p.uid, p.labels
			LIMIT 50000
		`,
		Timeout: 30000, // 30 second timeout
	}

	result, err := p.client.ExecuteQuery(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query pods for label index: %w", err)
	}

	count := 0
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}

		namespace, _ := row[0].(string)
		uid, _ := row[1].(string)
		labelsJSON, _ := row[2].(string)

		if namespace == "" || uid == "" {
			continue
		}

		var labels map[string]string
		if labelsJSON != "" && labelsJSON != "{}" {
			if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
				p.logger.Debug("Failed to parse labels for Pod %s: %v", uid, err)
				continue
			}
		}

		labelIndex.Update(namespace, "Pod", uid, labels)
		count++
	}

	hits, misses, namespaces, resources := labelIndex.GetStats()
	p.logger.Info("Label index bootstrapped: %d Pods indexed across %d namespaces (hits=%d, misses=%d, total=%d)",
		count, namespaces, hits, misses, resources)

	return nil
}
