package sync

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ProcessEvent processes a single event.
func (p *pipeline) ProcessEvent(ctx context.Context, event models.Event) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.EventsProcessed, 1)
		p.updateProcessingRate()
	}()

	update, err := p.builder.BuildFromEvent(ctx, event)
	if err != nil {
		atomic.AddInt64(&p.stats.Errors, 1)
		return fmt.Errorf("failed to build graph update: %w", err)
	}

	if err := p.applyGraphUpdate(ctx, update); err != nil {
		atomic.AddInt64(&p.stats.Errors, 1)
		return fmt.Errorf("failed to apply graph update: %w", err)
	}

	p.statsLock.Lock()
	p.stats.LastEventTime = time.Unix(0, event.Timestamp)
	p.stats.LastSyncTime = time.Now()
	p.stats.SyncLagMs = time.Since(time.Unix(0, event.Timestamp)).Milliseconds()
	p.statsLock.Unlock()

	p.logger.Debug("Processed event %s in %v", event.ID, time.Since(start))
	return nil
}

// ProcessBatch processes a batch of events using a two-phase write flow.
func (p *pipeline) ProcessBatch(ctx context.Context, events []models.Event) error {
	if len(events) == 0 {
		return nil
	}

	start := time.Now()
	p.logger.Info("Processing batch of %d events (two-phase)", len(events))

	p.builder.SetBatchCache(events)
	defer p.builder.ClearBatchCache()

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

	nodesCreated, err := p.applyBatchedNodeUpdates(ctx, nodeUpdates)
	if err != nil {
		p.logger.Warn("Batch node update failed, falling back to individual: %v", err)
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
	p.logger.InfoWithFields("Phase 1 complete",
		logging.Field("nodes_created", nodesCreated),
		logging.Field("events_processed", len(events)),
		logging.Field("duration_ms", phase1Duration.Milliseconds()))

	phase2Start := time.Now()
	p.logger.Debug("Phase 2: Extracting relationships for %d events", len(events))

	batchOptions := batchProcessingOptionsFromContext(ctx)
	edgeUpdates := make([]*GraphUpdate, 0, len(events)*2)
	totalEdges := 0

	for _, update := range nodeUpdates {
		if len(update.Edges) == 0 {
			continue
		}
		totalEdges += len(update.Edges)
		edgeUpdates = append(edgeUpdates, update)
	}

	if batchOptions.TimelineOnly {
		p.logger.Info("Skipping semantic relationship extraction for batch due to timeline-only processing override")
	} else {
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
	}

	edgesCreated, err := p.applyBatchedEdgeUpdates(ctx, edgeUpdates)
	if err != nil {
		p.logger.Warn("Batch edge update failed: %v", err)
	}

	phase2Duration := time.Since(phase2Start)
	edgesMissing := totalEdges - edgesCreated
	p.logger.InfoWithFields("Phase 2 complete",
		logging.Field("total_edges_attempted", totalEdges),
		logging.Field("edges_created", edgesCreated),
		logging.Field("edges_missing", edgesMissing),
		logging.Field("duration_ms", phase2Duration.Milliseconds()))

	if edgesMissing > 0 && totalEdges > 0 {
		failureRate := float64(edgesMissing) / float64(totalEdges) * 100
		if failureRate > 5 {
			p.logger.Warn("Significant edge creation failure: %d/%d edges missing (%.1f%%) - nodes may not exist for MATCH queries",
				edgesMissing, totalEdges, failureRate)
		}
	}

	enableCausality := p.config.EnableCausality && !batchOptions.DisableCausality && !batchOptions.TimelineOnly
	if enableCausality && len(events) > 1 {
		causalityStart := time.Now()
		if err := p.inferCausality(ctx, events); err != nil {
			p.logger.Warn("Failed to infer causality: %v", err)
		} else {
			p.logger.Debug("Causality inference complete in %v", time.Since(causalityStart))
		}
	} else if p.config.EnableCausality && batchOptions.TimelineOnly {
		p.logger.Info("Skipping causality inference for batch due to timeline-only processing override")
	} else if p.config.EnableCausality && batchOptions.DisableCausality {
		p.logger.Info("Skipping causality inference for batch due to processing override")
	}

	atomic.AddInt64(&p.stats.EventsProcessed, int64(len(events)))
	p.statsLock.Lock()
	lastEvent := events[len(events)-1]
	p.stats.LastEventTime = time.Unix(0, lastEvent.Timestamp)
	p.stats.LastSyncTime = time.Now()
	p.stats.SyncLagMs = time.Since(time.Unix(0, lastEvent.Timestamp)).Milliseconds()
	p.statsLock.Unlock()
	p.updateProcessingRate()

	p.logger.Info("Batch complete: %d events processed in %v (Phase1: %v, Phase2: %v)",
		len(events), time.Since(start), phase1Duration, phase2Duration)
	return nil
}

// inferCausality infers causal relationships between events.
func (p *pipeline) inferCausality(ctx context.Context, events []models.Event) error {
	links, err := p.causality.InferCausality(ctx, events)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}

	p.logger.Debug("Creating %d causality links", len(links))

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
