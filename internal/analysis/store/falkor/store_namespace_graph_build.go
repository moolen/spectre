package falkor

import "github.com/moolen/spectre/internal/analysis/store"

func (s *Store) buildNamespaceGraphNodes(
	resources []resourceResult,
	latestEvents map[string]*store.NamespaceGraphChangeEvent,
) []store.NamespaceGraphNode {
	nodes := make([]store.NamespaceGraphNode, 0, len(resources))
	for _, resource := range resources {
		if event, ok := latestEvents[resource.UID]; ok && event.EventType == "DELETE" {
			continue
		}
		if resource.Deleted {
			continue
		}

		node := store.NamespaceGraphNode{
			UID:         resource.UID,
			Kind:        resource.Kind,
			APIGroup:    resource.APIGroup,
			Namespace:   resource.Namespace,
			Name:        resource.Name,
			Status:      statusUnknown,
			LatestEvent: latestEvents[resource.UID],
			Labels:      resource.Labels,
		}
		if event := latestEvents[resource.UID]; event != nil && event.Status != "" {
			node.Status = event.Status
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *Store) buildNamespaceGraphEdges(edgeResults []edgeResult) []store.NamespaceGraphEdge {
	edges := make([]store.NamespaceGraphEdge, 0, len(edgeResults))
	for _, edge := range edgeResults {
		edges = append(edges, store.NamespaceGraphEdge{
			ID:               edge.EdgeID,
			Source:           edge.SourceUID,
			Target:           edge.TargetUID,
			RelationshipType: edge.RelationshipType,
		})
	}
	return edges
}
