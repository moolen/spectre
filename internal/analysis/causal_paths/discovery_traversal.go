package causalpaths

import (
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// traverseUpstream performs DFS from symptom toward root causes.
func (d *PathDiscoverer) traverseUpstream(
	symptomNode *analysis.GraphNode,
	adjacency map[string][]upstreamEdge,
	nodeMap map[string]*analysis.GraphNode,
	nodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
	maxDepth int,
) []CausalPath {
	var paths []CausalPath

	d.logger.Debug("traverseUpstream: starting from %s (Kind=%s, Name=%s), maxDepth=%d",
		symptomNode.ID, symptomNode.Resource.Kind, symptomNode.Resource.Name, maxDepth)
	d.logger.Debug("traverseUpstream: adjacency map has %d entries", len(adjacency))
	for nodeID, edges := range adjacency {
		node := nodeMap[nodeID]
		if node != nil {
			d.logger.Debug("traverseUpstream: adjacency[%s (%s/%s)] has %d upstream edges",
				nodeID, node.Resource.Kind, node.Resource.Name, len(edges))
			for _, edge := range edges {
				targetNode := nodeMap[edge.TargetNodeID]
				if targetNode != nil {
					d.logger.Debug("traverseUpstream:   -> %s (%s/%s) via %s",
						edge.TargetNodeID, targetNode.Resource.Kind, targetNode.Resource.Name, edge.Edge.RelationshipType)
				}
			}
		}
	}

	initialEntry := traversalEntry{
		CurrentNodeID: symptomNode.ID,
		Path: []pathElement{
			{NodeID: symptomNode.ID, Edge: nil},
		},
		Depth:        0,
		VisitedNodes: map[string]bool{symptomNode.ID: true},
	}

	stack := []traversalEntry{initialEntry}

	for len(stack) > 0 {
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if entry.Depth >= maxDepth {
			continue
		}

		upstreamEdges := adjacency[entry.CurrentNodeID]

		d.logger.Debug("traverseUpstream: at node %s (Kind=%s), depth=%d, upstreamEdges=%d",
			entry.CurrentNodeID, nodeMap[entry.CurrentNodeID].Resource.Kind, entry.Depth, len(upstreamEdges))

		if len(upstreamEdges) == 0 && entry.Depth > 0 {
			currentNode := nodeMap[entry.CurrentNodeID]
			currentAnomalies := nodeAnomalies[entry.CurrentNodeID]
			if currentNode != nil && HasCauseIntroducingAnomaly(currentAnomalies, symptomFirstFailure) {
				paths = append(paths, d.buildCausalPath(entry.Path, nodeMap, nodeAnomalies))
			}
			continue
		}

		for _, upstream := range upstreamEdges {
			if entry.VisitedNodes[upstream.TargetNodeID] {
				d.logger.Debug("traverseUpstream: skipping already visited node %s", upstream.TargetNodeID)
				continue
			}

			upstreamNode := nodeMap[upstream.TargetNodeID]
			if upstreamNode == nil {
				d.logger.Debug("traverseUpstream: upstreamNode is nil for %s", upstream.TargetNodeID)
				continue
			}

			upstreamAnomalies := nodeAnomalies[upstream.TargetNodeID]

			d.logger.Debug("traverseUpstream: considering upstream %s/%s via %s, anomalies=%d",
				upstreamNode.Resource.Kind, upstreamNode.Resource.Name, upstream.Edge.RelationshipType, len(upstreamAnomalies))

			edgeCategory := ClassifyEdge(upstream.Edge.RelationshipType)
			newElement := pathElement{
				NodeID: upstream.TargetNodeID,
				Edge:   upstream.Edge,
			}
			newPath := append([]pathElement{newElement}, entry.Path...)

			hasCauseAnomaly := HasCauseIntroducingAnomalyWithContext(upstreamAnomalies, nodeAnomalies, symptomFirstFailure)
			isCauseEdge := edgeCategory == EdgeCategoryCauseIntroducing
			isMaterializationEdge := edgeCategory == EdgeCategoryMaterialization
			hasDefinitiveCauseAnomaly := hasDefinitiveCauseIntroducingAnomalyWithContext(upstreamAnomalies, nodeAnomalies, symptomFirstFailure)
			hasUpstreamManager := d.hasManagesEdgeUpstream(upstream.TargetNodeID, adjacency)
			hasUpstreamReferencesSpec := d.hasReferencesSpecEdgeUpstream(upstream.TargetNodeID, adjacency)
			isReconciliationEffect := IsReconciliationEffectAnomaly(upstreamAnomalies, upstreamNode.Resource.Kind, hasUpstreamManager)

			isDefinitiveStopCandidate := hasDefinitiveCauseAnomaly &&
				(isCauseEdge || (isMaterializationEdge && entry.Depth > 0)) &&
				!hasUpstreamManager &&
				!isReconciliationEffect

			d.logger.Debug("traverseUpstream: %s/%s: hasCause=%v, hasDefinitive=%v, isCauseEdge=%v, hasUpstreamManager=%v, hasUpstreamRefsSpec=%v, isReconEffect=%v, isDefinitiveStop=%v",
				upstreamNode.Resource.Kind, upstreamNode.Resource.Name,
				hasCauseAnomaly, hasDefinitiveCauseAnomaly, isCauseEdge, hasUpstreamManager, hasUpstreamReferencesSpec, isReconciliationEffect, isDefinitiveStopCandidate)

			shouldStop := isDefinitiveStopCandidate && !hasUpstreamReferencesSpec

			d.logger.Debug("traverseUpstream: %s/%s: shouldStop=%v", upstreamNode.Resource.Kind, upstreamNode.Resource.Name, shouldStop)

			if shouldStop {
				paths = append(paths, d.buildCausalPath(newPath, nodeMap, nodeAnomalies))
				continue
			}

			if isDefinitiveStopCandidate && hasUpstreamReferencesSpec {
				paths = append(paths, d.buildCausalPath(newPath, nodeMap, nodeAnomalies))
			}

			if hasCauseAnomaly && !hasDefinitiveCauseAnomaly && (isCauseEdge || (isMaterializationEdge && entry.Depth > 0)) {
				paths = append(paths, d.buildCausalPath(newPath, nodeMap, nodeAnomalies))
			}

			shouldContinue := d.shouldContinueTraversal(upstreamAnomalies, edgeCategory)
			if isDefinitiveStopCandidate && hasUpstreamReferencesSpec {
				shouldContinue = true
			}

			d.logger.Debug("traverseUpstream: %s/%s: shouldContinue=%v", upstreamNode.Resource.Kind, upstreamNode.Resource.Name, shouldContinue)

			if shouldContinue {
				newVisited := make(map[string]bool, len(entry.VisitedNodes)+1)
				for key, value := range entry.VisitedNodes {
					newVisited[key] = value
				}
				newVisited[upstream.TargetNodeID] = true

				stack = append(stack, traversalEntry{
					CurrentNodeID: upstream.TargetNodeID,
					Path:          newPath,
					Depth:         entry.Depth + 1,
					VisitedNodes:  newVisited,
				})
			}
		}
	}

	return paths
}

// hasManagesEdgeUpstream checks if a node has a MANAGES edge pointing upstream.
func (d *PathDiscoverer) hasManagesEdgeUpstream(nodeID string, adjacency map[string][]upstreamEdge) bool {
	for _, upstream := range adjacency[nodeID] {
		if upstream.Edge != nil && upstream.Edge.RelationshipType == edgeTypeManages {
			return true
		}
	}
	return false
}

// hasReferencesSpecEdgeUpstream checks if a node has REFERENCES_SPEC edges pointing upstream.
func (d *PathDiscoverer) hasReferencesSpecEdgeUpstream(nodeID string, adjacency map[string][]upstreamEdge) bool {
	for _, upstream := range adjacency[nodeID] {
		if upstream.Edge != nil && upstream.Edge.RelationshipType == "REFERENCES_SPEC" {
			return true
		}
	}
	return false
}

// shouldContinueTraversal determines if traversal should continue through this node.
func (d *PathDiscoverer) shouldContinueTraversal(
	nodeAnomalies []anomaly.Anomaly,
	edgeCategory string,
) bool {
	if edgeCategory == EdgeCategoryMaterialization {
		return true
	}

	if HasOnlyDerivedAnomalies(nodeAnomalies) {
		return true
	}

	if hasOnlyIntermediateCauseAnomalies(nodeAnomalies) {
		return true
	}

	return false
}
