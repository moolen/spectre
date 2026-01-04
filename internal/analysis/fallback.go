package analysis

import (
	"fmt"
)

// Fallback logic for cases where no causal chain can be built
// This provides a minimal but valid response when graph analysis fails

// createSymptomOnlyGraph creates a minimal causal graph containing only the symptom resource
// This is used as a fallback when no ownership chain or causal relationships can be determined
func createSymptomOnlyGraph(symptom *ObservedSymptom) CausalGraph {
	nodeID := createNodeID(symptom.Resource.UID)

	return CausalGraph{
		Nodes: []GraphNode{
			createSpineNode(
				nodeID,
				symptom.Resource,
				&ChangeEventInfo{
					EventID:       "",
					Timestamp:     symptom.ObservedAt,
					EventType:     "OBSERVED",
					ConfigChanged: false,
					StatusChanged: true,
					Description:   "Observed failure",
				},
				nil, // No additional events
				nil, // No K8s events
				1,   // Step number
				fmt.Sprintf("Direct observation of %s", symptom.SymptomType),
			),
		},
		Edges: []GraphEdge{},
	}
}

// createSymptomOnlyRootCause creates a minimal root cause hypothesis for symptom-only scenarios
// This is used when no causal chain can be determined from the graph
func createSymptomOnlyRootCause(symptom *ObservedSymptom) *RootCauseHypothesis {
	return &RootCauseHypothesis{
		Resource: symptom.Resource,
		ChangeEvent: ChangeEventInfo{
			EventID:       "",
			Timestamp:     symptom.ObservedAt,
			EventType:     "OBSERVED",
			ConfigChanged: false,
			StatusChanged: true,
			Description:   "Observed failure",
		},
		CausationType: "DirectObservation",
		Explanation: fmt.Sprintf("%s '%s' failed with %s. No causal chain found in graph data.",
			symptom.Resource.Kind, symptom.Resource.Name, symptom.SymptomType),
		TimeLagMs: 0,
	}
}

// isGraphEmpty checks if a causal graph is empty or invalid
func isGraphEmpty(graph CausalGraph) bool {
	return len(graph.Nodes) == 0
}

// hasValidSpineNodes checks if the graph has at least one SPINE node with events
func hasValidSpineNodes(graph CausalGraph) bool {
	for _, node := range graph.Nodes {
		if node.NodeType == "SPINE" && node.ChangeEvent != nil {
			return true
		}
	}
	return false
}
