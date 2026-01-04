package analysis

import (
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// Node factory functions to eliminate duplication and standardize node creation

// createSpineNode creates a SPINE node with all required fields
func createSpineNode(
	nodeID string,
	resource SymptomResource,
	primaryEvent *ChangeEventInfo,
	allEvents []ChangeEventInfo,
	k8sEvents []K8sEventInfo,
	stepNumber int,
	reasoning string,
) GraphNode {
	return GraphNode{
		ID:          nodeID,
		Resource:    resource,
		ChangeEvent: primaryEvent,
		AllEvents:   allEvents,
		K8sEvents:   k8sEvents,
		NodeType:    "SPINE",
		StepNumber:  stepNumber,
		Reasoning:   reasoning,
	}
}

// createRelatedNode creates a RELATED node for supporting resources
func createRelatedNode(
	nodeID string,
	resource SymptomResource,
	allEvents []ChangeEventInfo,
) GraphNode {
	return GraphNode{
		ID:        nodeID,
		Resource:  resource,
		AllEvents: allEvents,
		NodeType:  "RELATED",
	}
}

// resourceIdentityToSymptomResource converts a graph.ResourceIdentity to SymptomResource
// This function centralizes the conversion logic used throughout the package
func resourceIdentityToSymptomResource(ri graph.ResourceIdentity) SymptomResource {
	return SymptomResource{
		UID:       ri.UID,
		Kind:      ri.Kind,
		Namespace: ri.Namespace,
		Name:      ri.Name,
	}
}

// createNodeID generates a consistent node ID from a resource UID
func createNodeID(resourceUID string) string {
	return fmt.Sprintf("node-%s", resourceUID)
}

// createEdgeID generates a consistent edge ID from node IDs
func createEdgeID(fromNodeID, toNodeID string) string {
	return fmt.Sprintf("edge-%s-%s", fromNodeID, toNodeID)
}

// createSpineEdgeID generates an edge ID for SPINE edges
func createSpineEdgeID(fromUID, toUID string) string {
	return fmt.Sprintf("edge-spine-%s-%s", fromUID, toUID)
}

// createAttachmentEdgeID generates an edge ID for ATTACHMENT edges
func createAttachmentEdgeID(fromNodeID, toNodeID string) string {
	return fmt.Sprintf("edge-attach-%s-%s", fromNodeID, toNodeID)
}
