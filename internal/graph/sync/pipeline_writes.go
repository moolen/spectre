package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/moolen/spectre/internal/graph"
)

// applyGraphUpdate applies a graph update to the graph database.
func (p *pipeline) applyGraphUpdate(ctx context.Context, update *GraphUpdate) error {
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

// createEdge creates an edge in the graph.
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

	result, err := p.client.ExecuteQuery(ctx, query)
	if err != nil {
		return err
	}
	if result.Stats.RelationshipsCreated == 0 {
		p.logger.Warn("Edge %s not created (%s -> %s): MATCH may have failed to find nodes (stats: rels=%d, nodes=%d)",
			edge.Type, edge.FromUID, edge.ToUID, result.Stats.RelationshipsCreated, result.Stats.NodesCreated)
	}

	return nil
}

// bootstrapLabelIndex populates the label index from existing Pod data in the graph.
func (p *pipeline) bootstrapLabelIndex(ctx context.Context) error {
	labelIndex := p.builder.GetLabelIndex()
	if labelIndex == nil {
		p.logger.Debug("Label index not enabled, skipping bootstrap")
		return nil
	}

	query := graph.GraphQuery{
		Query: `
			MATCH (p:ResourceIdentity {kind: 'Pod'})
			WHERE NOT p.deleted
			RETURN p.namespace, p.uid, p.labels
			LIMIT 50000
		`,
		Timeout: 30000,
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
