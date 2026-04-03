package causalpaths

import "github.com/moolen/spectre/internal/analysis"

// buildNodeMap creates a lookup map from node ID to GraphNode.
func (d *PathDiscoverer) buildNodeMap(graph analysis.CausalGraph) map[string]*analysis.GraphNode {
	nodeMap := make(map[string]*analysis.GraphNode, len(graph.Nodes))
	for i := range graph.Nodes {
		nodeMap[graph.Nodes[i].ID] = &graph.Nodes[i]
	}
	return nodeMap
}

// buildUpstreamAdjacency creates an adjacency map for upstream traversal.
// Edges point FROM the downstream node TO the upstream node.
func (d *PathDiscoverer) buildUpstreamAdjacency(graph analysis.CausalGraph) map[string][]upstreamEdge {
	adjacency := make(map[string][]upstreamEdge)

	d.logger.Debug("buildUpstreamAdjacency: processing %d edges", len(graph.Edges))

	for i := range graph.Edges {
		edge := &graph.Edges[i]
		edgeCategory := ClassifyEdge(edge.RelationshipType)

		d.logger.Debug("buildUpstreamAdjacency: edge %s -> %s (type=%s, category=%s)",
			edge.From, edge.To, edge.RelationshipType, edgeCategory)

		if edge.RelationshipType == edgeTypeManages || edge.RelationshipType == "GRANTS_TO" {
			d.logger.Debug("buildUpstreamAdjacency: MANAGES/GRANTS_TO edge: adding adjacency[%s] -> %s", edge.To, edge.From)
			adjacency[edge.To] = append(adjacency[edge.To], upstreamEdge{
				TargetNodeID: edge.From,
				Edge:         edge,
			})
		} else if edgeCategory == EdgeCategoryCauseIntroducing {
			d.logger.Debug("buildUpstreamAdjacency: CauseIntroducing edge: adding adjacency[%s] -> %s", edge.From, edge.To)
			adjacency[edge.From] = append(adjacency[edge.From], upstreamEdge{
				TargetNodeID: edge.To,
				Edge:         edge,
			})
		} else {
			d.logger.Debug("buildUpstreamAdjacency: Materialization edge: adding adjacency[%s] -> %s", edge.To, edge.From)
			adjacency[edge.To] = append(adjacency[edge.To], upstreamEdge{
				TargetNodeID: edge.From,
				Edge:         edge,
			})
		}
	}

	return adjacency
}

// findSymptomNode finds the node matching the symptom resource UID.
func (d *PathDiscoverer) findSymptomNode(graph analysis.CausalGraph, resourceUID string) *analysis.GraphNode {
	for i := range graph.Nodes {
		if graph.Nodes[i].Resource.UID == resourceUID {
			return &graph.Nodes[i]
		}
	}
	return nil
}
