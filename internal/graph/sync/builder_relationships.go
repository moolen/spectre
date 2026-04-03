package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

// ExtractRelationships extracts relationships from resource data
func (b *graphBuilder) ExtractRelationships(ctx context.Context, event models.Event) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	if len(event.Data) == 0 {
		return edges, nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
		return nil, fmt.Errorf("failed to parse resource data: %w", err)
	}

	edges = append(edges, b.extractOwnershipRelationships(event.Resource.UID, resourceData)...)

	if event.Resource.Kind == "Service" || event.Resource.Kind == kindDeployment || event.Resource.Kind == kindReplicaSet || event.Resource.Kind == "StatefulSet" || event.Resource.Kind == "DaemonSet" {
		edges = append(edges, b.extractSelectorRelationships(event.Resource.UID, event.Resource.Kind, resourceData)...)
	}

	if event.Resource.Kind == kindPod {
		if schedEdge := b.extractSchedulingRelationship(event.Resource.UID, resourceData); schedEdge != nil {
			edges = append(edges, *schedEdge)
		}
	}

	if event.Resource.Kind == kindPod {
		edges = append(edges, b.extractVolumeRelationships(event.Resource.UID, resourceData)...)
	}

	if event.Resource.Kind == kindPod {
		if saEdge := b.extractServiceAccountRelationship(event.Resource.UID, resourceData); saEdge != nil {
			edges = append(edges, *saEdge)
		}
	}

	if b.extractorRegistry != nil {
		crEdges, err := b.extractorRegistry.Extract(ctx, event)
		if err != nil {
			b.logger.Warn("Custom resource extraction failed for event %s: %v", event.ID, err)
		} else {
			b.logger.Debug("Custom resource extractors produced %d edges", len(crEdges))
			edges = append(edges, crEdges...)
		}
	}

	return edges, nil
}

// extractOwnershipRelationships extracts OWNS edges from ownerReferences
func (b *graphBuilder) extractOwnershipRelationships(ownedUID string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return edges
	}

	ownerRefsRaw, ok := metadata["ownerReferences"]
	if !ok {
		return edges
	}

	ownerRefs, ok := ownerRefsRaw.([]interface{})
	if !ok {
		return edges
	}

	for _, refRaw := range ownerRefs {
		ref, ok := refRaw.(map[string]interface{})
		if !ok {
			continue
		}

		ownerUID, ok := ref["uid"].(string)
		if !ok {
			continue
		}

		controller := false
		if ctrl, ok := ref["controller"].(bool); ok {
			controller = ctrl
		}

		blockOwnerDeletion := false
		if bod, ok := ref["blockOwnerDeletion"].(bool); ok {
			blockOwnerDeletion = bod
		}

		props := graph.OwnsEdge{
			Controller:         controller,
			BlockOwnerDeletion: blockOwnerDeletion,
		}

		propsJSON, _ := json.Marshal(props)

		edges = append(edges, graph.Edge{
			Type:       graph.EdgeTypeOwns,
			FromUID:    ownerUID,
			ToUID:      ownedUID,
			Properties: propsJSON,
		})
	}

	return edges
}

// extractSelectorRelationships extracts SELECTS edges for Services, Deployments, etc. -> Pods
func (b *graphBuilder) extractSelectorRelationships(selectorUID, kind string, resourceData map[string]interface{}) []graph.Edge {
	edges := []graph.Edge{}

	if b.client == nil {
		return edges
	}

	var selector map[string]string
	var namespace string

	metadata, ok := resourceData["metadata"].(map[string]interface{})
	if !ok {
		return edges
	}

	namespace, _ = metadata["namespace"].(string)

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return edges
	}

	switch kind {
	case "Service":
		if selectorRaw, ok := spec["selector"].(map[string]interface{}); ok {
			selector = make(map[string]string)
			for key, value := range selectorRaw {
				if strVal, ok := value.(string); ok {
					selector[key] = strVal
				}
			}
		}
	case kindDeployment, kindReplicaSet, "StatefulSet", "DaemonSet":
		if selectorRaw, ok := spec["selector"].(map[string]interface{}); ok {
			if matchLabels, ok := selectorRaw["matchLabels"].(map[string]interface{}); ok {
				selector = make(map[string]string)
				for key, value := range matchLabels {
					if strVal, ok := value.(string); ok {
						selector[key] = strVal
					}
				}
			}
		}
	}

	if len(selector) == 0 {
		return edges
	}

	matchingPodUIDs, err := b.findPodsMatchingLabels(context.Background(), selector, namespace)
	if err != nil {
		b.logger.Debug("Failed to find Pods matching selector for %s %s: %v", kind, selectorUID, err)
		return edges
	}

	selectorLabelsJSON, _ := json.Marshal(selector)
	for _, podUID := range matchingPodUIDs {
		propsJSON, _ := json.Marshal(graph.SelectsEdge{
			SelectorLabels: selector,
		})

		edges = append(edges, graph.Edge{
			Type:       graph.EdgeTypeSelects,
			FromUID:    selectorUID,
			ToUID:      podUID,
			Properties: json.RawMessage(propsJSON),
		})

		b.logger.Debug("Created SELECTS edge: %s %s -> Pod %s (selector: %s)", kind, selectorUID, podUID, string(selectorLabelsJSON))
	}

	return edges
}

// findPodsMatchingLabels finds Pods with labels matching the selector.
// It first checks the in-memory label index (O(1)), falling back to graph queries
// only if the index doesn't have data for the namespace (e.g., during bootstrap).
func (b *graphBuilder) findPodsMatchingLabels(ctx context.Context, selector map[string]string, namespace string) ([]string, error) {
	if b.labelIndex != nil && namespace != "" {
		uids := b.labelIndex.FindBySelector(namespace, kindPod, selector)
		if uids != nil {
			b.logger.Debug("Label index hit: found %d Pods matching selector in namespace %s", len(uids), namespace)
			return uids, nil
		}
		if !b.labelIndex.Contains(namespace, kindPod, "") {
			hits, _, _, totalResources := b.labelIndex.GetStats()
			if totalResources > 0 || hits > 0 {
				b.logger.Debug("Label index: no matching Pods in namespace %s for selector %v", namespace, selector)
				return []string{}, nil
			}
			b.logger.Debug("Label index empty, falling back to graph query for namespace %s", namespace)
		} else {
			b.logger.Debug("Label index: no matching Pods in namespace %s for selector %v", namespace, selector)
			return []string{}, nil
		}
	}

	if b.client == nil {
		return nil, fmt.Errorf("no graph client available for Pod lookup")
	}

	var query graph.GraphQuery
	if namespace != "" {
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity {kind: $kind, namespace: $namespace})
				WHERE NOT p.deleted
				RETURN p.uid as uid, p.labels as labels
				LIMIT 100
			`,
			Parameters: map[string]interface{}{
				"kind":      kindPod,
				"namespace": namespace,
			},
		}
	} else {
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity {kind: $kind})
				WHERE NOT p.deleted
				RETURN p.uid as uid, p.labels as labels
				LIMIT 100
			`,
			Parameters: map[string]interface{}{
				"kind": kindPod,
			},
		}
	}

	result, err := b.client.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var podUIDs []string
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		uid, ok := row[0].(string)
		if !ok {
			continue
		}

		var podLabels map[string]string
		if labelsStr, ok := row[1].(string); ok && labelsStr != "" {
			_ = json.Unmarshal([]byte(labelsStr), &podLabels)
		}

		if extractors.LabelsMatchSelector(podLabels, selector) {
			podUIDs = append(podUIDs, uid)
		}
	}

	b.logger.Debug("Graph query found %d Pod matches in namespace %s for selector %v", len(podUIDs), namespace, selector)

	return podUIDs, nil
}
