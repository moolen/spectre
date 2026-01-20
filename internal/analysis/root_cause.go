package analysis

import (
	"fmt"
)

const (
	nodeTypeSpine   = "SPINE"
	eventTypeCreate = "CREATE"
)

// identifyRootCause extracts the root cause from the causal graph
func (a *RootCauseAnalyzer) identifyRootCause(
	graph CausalGraph,
	failureTimestamp int64,
) (*RootCauseHypothesis, error) {
	if len(graph.Nodes) == 0 {
		return nil, fmt.Errorf("empty causal graph")
	}

	// Find SPINE nodes sorted by step number
	spineNodes := []GraphNode{}
	for _, node := range graph.Nodes {
		if node.NodeType == nodeTypeSpine {
			spineNodes = append(spineNodes, node)
		}
	}

	if len(spineNodes) == 0 {
		return nil, fmt.Errorf("no spine nodes in graph")
	}

	// Sort by step number (descending) to get root cause (last step)
	for i := 0; i < len(spineNodes); i++ {
		for j := i + 1; j < len(spineNodes); j++ {
			if spineNodes[j].StepNumber > spineNodes[i].StepNumber {
				spineNodes[i], spineNodes[j] = spineNodes[j], spineNodes[i]
			}
		}
	}

	// Root cause is the last step in the chain (highest step number)
	rootNode := spineNodes[0]

	// If no change event at root, use first node with change event
	var rootEvent *ChangeEventInfo
	for _, node := range spineNodes {
		if node.ChangeEvent != nil {
			rootEvent = node.ChangeEvent
			rootNode = node
			break
		}
	}

	if rootEvent == nil {
		return nil, fmt.Errorf("no change event found in causal graph")
	}

	// Find relationship type for this node
	relationshipType := ""
	for _, edge := range graph.Edges {
		if edge.From == rootNode.ID {
			relationshipType = edge.RelationshipType
			break
		}
	}

	// Classify the causation type
	causationType := classifyCausationType(rootEvent, relationshipType)

	// Generate explanation
	explanation := generateRootCauseExplanation(rootNode, causationType, spineNodes)

	// Calculate time lag
	timeLagMs := (failureTimestamp - rootEvent.Timestamp.UnixNano()) / 1_000_000

	return &RootCauseHypothesis{
		Resource: rootNode.Resource,
		ChangeEvent: ChangeEventInfo{
			EventID:       rootEvent.EventID,
			Timestamp:     rootEvent.Timestamp,
			EventType:     rootEvent.EventType,
			ConfigChanged: rootEvent.ConfigChanged,
			StatusChanged: rootEvent.StatusChanged,
			Description:   rootEvent.Description,
		},
		CausationType: causationType,
		Explanation:   explanation,
		TimeLagMs:     timeLagMs,
	}, nil
}

// classifyCausationType determines the type of root cause
func classifyCausationType(event *ChangeEventInfo, relationshipType string) string {
	if event.ConfigChanged {
		return "ConfigChange"
	}
	switch event.EventType {
	case eventTypeCreate:
		return "ResourceCreation"
	case "UPDATE":
		if relationshipType == edgeTypeManages {
			return "DeploymentUpdate"
		}
		return "ResourceUpdate"
	case "DELETE":
		return "ResourceDeletion"
	default:
		return "Unknown"
	}
}

// generateRootCauseExplanation creates a human-readable explanation
func generateRootCauseExplanation(
	rootNode GraphNode,
	causationType string,
	spineNodes []GraphNode,
) string {
	explanation := fmt.Sprintf("%s '%s' ", rootNode.Resource.Kind, rootNode.Resource.Name)

	switch causationType {
	case "ConfigChange":
		explanation += "configuration was changed"
	case "DeploymentUpdate":
		explanation += "was updated (deployment)"
	case "ResourceCreation":
		explanation += "was created"
	case "ResourceUpdate":
		explanation += "was updated"
	case "ResourceDeletion":
		explanation += "was deleted"
	default:
		explanation += "changed"
	}

	// Add propagation path if chain is longer than just root
	if len(spineNodes) > 1 {
		explanation += ", which cascaded through "
		for i := len(spineNodes) - 2; i >= 0; i-- {
			explanation += spineNodes[i].Resource.Kind
			if i > 0 {
				explanation += " → "
			}
		}
	}

	return explanation
}
