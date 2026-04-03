package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/moolen/spectre/internal/graph"
)

// applyBatchedNodeUpdates applies multiple graph updates using batch queries.
func (p *pipeline) applyBatchedNodeUpdates(ctx context.Context, updates []*GraphUpdate) (nodesCreated int, err error) {
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

	for i := 0; i < len(nonDeletedResources); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(nonDeletedResources) {
			end = len(nonDeletedResources)
		}
		batch := nonDeletedResources[i:end]

		query := graph.BatchUpsertResourceIdentitiesQuery(batch)
		query.Timeout = batchQueryTimeout
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch upsert resources (batch %d-%d): %w", i, end, err)
		}

		if result.Stats.NodesCreated == 0 && result.Stats.PropertiesSet == 0 && len(batch) > 0 {
			p.logger.Warn("Batch upsert may have failed: expected %d resources, stats show 0 created and 0 props set (batch %d-%d)",
				len(batch), i, end)
		}

		nodesCreated += len(batch)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(batch)))
		p.logger.Debug("Batch upserted %d ResourceIdentity nodes (batch %d-%d, stats: %d nodes created, %d props set)",
			len(batch), i, end, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

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

	for i := 0; i < len(allChangeEvents); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(allChangeEvents) {
			end = len(allChangeEvents)
		}
		batch := allChangeEvents[i:end]

		query := graph.BatchCreateChangeEventsQuery(batch)
		query.Timeout = batchQueryTimeout
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch create change events (batch %d-%d): %w", i, end, err)
		}

		if result.Stats.NodesCreated == 0 && result.Stats.PropertiesSet == 0 && len(batch) > 0 {
			p.logger.Warn("Batch create ChangeEvents may have failed: expected %d events, stats show 0 created and 0 props set (batch %d-%d)",
				len(batch), i, end)
		}

		nodesCreated += len(batch)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(batch)))
		p.logger.Debug("Batch created %d ChangeEvent nodes (batch %d-%d, stats: %d nodes created, %d props set)",
			len(batch), i, end, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	for i := 0; i < len(allK8sEvents); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(allK8sEvents) {
			end = len(allK8sEvents)
		}
		batch := allK8sEvents[i:end]

		query := graph.BatchCreateK8sEventsQuery(batch)
		query.Timeout = batchQueryTimeout
		result, err := p.client.ExecuteQuery(ctx, query)
		if err != nil {
			return nodesCreated, fmt.Errorf("failed to batch create K8s events (batch %d-%d): %w", i, end, err)
		}
		nodesCreated += len(batch)
		atomic.AddInt64(&p.stats.NodesCreated, int64(len(batch)))
		p.logger.Debug("Batch created %d K8sEvent nodes (batch %d-%d, stats: %d nodes created, %d props set)",
			len(batch), i, end, result.Stats.NodesCreated, result.Stats.PropertiesSet)
	}

	return nodesCreated, nil
}

// applyBatchedEdgeUpdates applies multiple edge updates using batch queries.
func (p *pipeline) applyBatchedEdgeUpdates(ctx context.Context, updates []*GraphUpdate) (edgesCreated int, err error) {
	edgesByType := make(map[graph.EdgeType][]graph.Edge)
	for _, update := range updates {
		for _, edge := range update.Edges {
			edgesByType[edge.Type] = append(edgesByType[edge.Type], edge)
		}
	}

	for edgeType, edges := range edgesByType {
		if len(edges) == 0 {
			continue
		}

		for batchStart := 0; batchStart < len(edges); batchStart += maxBatchSize {
			batchEnd := batchStart + maxBatchSize
			if batchEnd > len(edges) {
				batchEnd = len(edges)
			}
			edgeBatch := edges[batchStart:batchEnd]

			batchParams := make([]graph.BatchEdgeParams, len(edgeBatch))
			for i, edge := range edgeBatch {
				var props map[string]any
				if edge.Properties != nil {
					_ = json.Unmarshal(edge.Properties, &props)
				}
				if props == nil {
					props = make(map[string]any)
				}
				batchParams[i] = graph.BatchEdgeParams{
					FromUID:    edge.FromUID,
					ToUID:      edge.ToUID,
					Properties: props,
				}
			}

			query, supported := batchedEdgeQuery(edgeType, batchParams)
			if !supported {
				created := p.applyEdgeBatchIndividually(ctx, edgeBatch)
				edgesCreated += created
				continue
			}

			query.Timeout = batchQueryTimeout
			result, err := p.client.ExecuteQuery(ctx, query)
			if err != nil {
				p.logger.Warn("Failed to batch create %s edges (batch %d-%d): %v", edgeType, batchStart, batchEnd, err)
				created := p.applyEdgeBatchIndividually(ctx, edgeBatch)
				edgesCreated += created
				continue
			}

			actualEdgesCreated := result.Stats.RelationshipsCreated
			if actualEdgesCreated < len(edgeBatch) {
				p.logger.Warn("Batch edge creation partial: expected %d %s edges, created %d (missing %d, batch %d-%d)",
					len(edgeBatch), edgeType, actualEdgesCreated, len(edgeBatch)-actualEdgesCreated, batchStart, batchEnd)
			}

			edgesCreated += actualEdgesCreated
			atomic.AddInt64(&p.stats.EdgesCreated, int64(actualEdgesCreated))
			p.logger.Debug("Batch created %d %s edges (attempted: %d, batch %d-%d)",
				actualEdgesCreated, edgeType, len(edgeBatch), batchStart, batchEnd)
		}
	}

	return edgesCreated, nil
}

func batchedEdgeQuery(edgeType graph.EdgeType, batchParams []graph.BatchEdgeParams) (graph.GraphQuery, bool) {
	switch edgeType {
	case graph.EdgeTypeOwns:
		return graph.BatchCreateOwnsEdgesQuery(batchParams), true
	case graph.EdgeTypeChanged:
		return graph.BatchCreateChangedEdgesQuery(batchParams), true
	case graph.EdgeTypeSelects:
		return graph.BatchCreateSelectsEdgesQuery(batchParams), true
	case graph.EdgeTypeScheduledOn:
		return graph.BatchCreateScheduledOnEdgesQuery(batchParams), true
	case graph.EdgeTypeMounts:
		return graph.BatchCreateMountsEdgesQuery(batchParams), true
	case graph.EdgeTypeReferencesSpec:
		return graph.BatchCreateReferencesSpecEdgesQuery(batchParams), true
	case graph.EdgeTypeManages:
		return graph.BatchCreateManagesEdgesQuery(batchParams), true
	case graph.EdgeTypeEmittedEvent:
		return graph.BatchCreateEmittedEventEdgesQuery(batchParams), true
	case graph.EdgeTypeUsesServiceAccount:
		return graph.BatchCreateUsesServiceAccountEdgesQuery(batchParams), true
	case graph.EdgeTypeBindsRole:
		return graph.BatchCreateBindsRoleEdgesQuery(batchParams), true
	case graph.EdgeTypeGrantsTo:
		return graph.BatchCreateGrantsToEdgesQuery(batchParams), true
	case graph.EdgeTypeCreatesObserved:
		return graph.BatchCreateCreatesObservedEdgesQuery(batchParams), true
	default:
		return graph.GraphQuery{}, false
	}
}

func (p *pipeline) applyEdgeBatchIndividually(ctx context.Context, edges []graph.Edge) int {
	created := 0
	for _, edge := range edges {
		if err := p.createEdge(ctx, edge); err != nil {
			p.logger.Warn("Failed to create edge %s (%s -> %s): %v",
				edge.Type, edge.FromUID, edge.ToUID, err)
			continue
		}
		created++
		atomic.AddInt64(&p.stats.EdgesCreated, 1)
	}

	return created
}
