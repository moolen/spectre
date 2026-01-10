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
		builder:   NewGraphBuilderWithClient(client), // Pass client to builder for Node lookups
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

	// Apply all node updates
	nodesCreated := 0
	for _, update := range nodeUpdates {
		if err := p.applyGraphUpdate(ctx, update); err != nil {
			p.logger.Warn("Failed to apply node update for event %s: %v", update.SourceEventID, err)
			atomic.AddInt64(&p.stats.Errors, 1)
			continue
		}
		nodesCreated++
	}

	phase1Duration := time.Since(phase1Start)
	p.logger.Info("Phase 1 complete: Created %d/%d resource nodes in %v", nodesCreated, len(events), phase1Duration)

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

	// Apply all edge updates
	edgesCreated := 0
	for _, update := range edgeUpdates {
		if err := p.applyGraphUpdate(ctx, update); err != nil {
			p.logger.Warn("Failed to apply edge update for event %s: %v", update.SourceEventID, err)
			atomic.AddInt64(&p.stats.Errors, 1)
			continue
		}
		edgesCreated += len(update.Edges)
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
