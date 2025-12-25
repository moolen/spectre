package analysis

import (
	"context"
	"fmt"
	"math"

	"github.com/moolen/spectre/internal/graph"
)

// collectSupportingEvidence consolidates evidence from the causal graph
func (a *RootCauseAnalyzer) collectSupportingEvidence(
	graph CausalGraph,
	rootCause *RootCauseHypothesis,
) []EvidenceItem {
	evidence := []EvidenceItem{}
	seenTypes := make(map[string]bool)

	// RELATIONSHIP evidence (MANAGES edges)
	for _, edge := range graph.Edges {
		if edge.RelationshipType == "MANAGES" && !seenTypes["MANAGES"] {
			// Find the nodes
			var fromNode, toNode *GraphNode
			for i := range graph.Nodes {
				if graph.Nodes[i].ID == edge.From {
					fromNode = &graph.Nodes[i]
				}
				if graph.Nodes[i].ID == edge.To {
					toNode = &graph.Nodes[i]
				}
			}
			
			if fromNode != nil && toNode != nil {
				evidence = append(evidence, EvidenceItem{
					Type:        "RELATIONSHIP",
					Description: fromNode.Reasoning,
					Confidence:  1.0,
					Details: map[string]interface{}{
						"relationshipType": "MANAGES",
						"from":             fromNode.Resource,
						"to":               toNode.Resource,
					},
				})
				seenTypes["MANAGES"] = true
			}
		}
	}

	// TEMPORAL evidence
	if rootCause.TimeLagMs > 0 && !seenTypes["TEMPORAL"] {
		evidence = append(evidence, EvidenceItem{
			Type:        "TEMPORAL",
			Description: fmt.Sprintf("Change occurred %d seconds before failure", rootCause.TimeLagMs/1000),
			Confidence:  math.Max(0, 1.0-(float64(rootCause.TimeLagMs)/600000.0)),
			Details: map[string]interface{}{
				"lagMs": rootCause.TimeLagMs,
			},
		})
		seenTypes["TEMPORAL"] = true
	}

	// STRUCTURAL evidence (ownership chain)
	spineNodeCount := 0
	for _, node := range graph.Nodes {
		if node.NodeType == "SPINE" {
			spineNodeCount++
		}
	}
	
	if spineNodeCount > 1 && !seenTypes["STRUCTURAL"] {
		// Build chain description
		spineNodes := []GraphNode{}
		for _, node := range graph.Nodes {
			if node.NodeType == "SPINE" {
				spineNodes = append(spineNodes, node)
			}
		}
		
		// Sort by step number (descending)
		for i := 0; i < len(spineNodes); i++ {
			for j := i + 1; j < len(spineNodes); j++ {
				if spineNodes[j].StepNumber > spineNodes[i].StepNumber {
					spineNodes[i], spineNodes[j] = spineNodes[j], spineNodes[i]
				}
			}
		}
		
		chainDesc := ""
		for i, node := range spineNodes {
			chainDesc += node.Resource.Kind
			if i < len(spineNodes)-1 {
				chainDesc += " → "
			}
		}
		
		evidence = append(evidence, EvidenceItem{
			Type:        "STRUCTURAL",
			Description: fmt.Sprintf("Ownership chain: %s", chainDesc),
			Confidence:  0.8,
			Details: map[string]interface{}{
				"chainLength": spineNodeCount,
			},
		})
		seenTypes["STRUCTURAL"] = true
	}

	// Limit to 5 most relevant items
	if len(evidence) > 5 {
		evidence = evidence[:5]
	}

	return evidence
}

// detectExcludedAlternatives identifies other hypotheses that were considered but rejected
func (a *RootCauseAnalyzer) detectExcludedAlternatives(
	ctx context.Context,
	symptom *ObservedSymptom,
	rootCause *RootCauseHypothesis,
	failureTimestamp int64,
) []ExcludedHypothesis {
	// Query for other changes in the time window
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp <= $failureTimestamp
			  AND e.timestamp >= $failureTimestamp - $lookback
			  AND r.uid <> $rootCauseUID
			  AND r.namespace = $namespace
			RETURN r, e
			ORDER BY e.timestamp DESC
			LIMIT 5
		`,
		Parameters: map[string]interface{}{
			"failureTimestamp": failureTimestamp,
			"lookback":         int64(600_000_000_000),
			"rootCauseUID":     rootCause.Resource.UID,
			"namespace":        symptom.Resource.Namespace,
		},
	}

	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		a.logger.Debug("Failed to query excluded alternatives: %v", err)
		return nil
	}

	excluded := []ExcludedHypothesis{}
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)

		eventProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil {
			continue
		}
		_ = graph.ParseChangeEventFromNode(eventProps) // Parse but not used directly

		// Generate hypothesis and reason for exclusion
		hypothesis := fmt.Sprintf("%s '%s' changed at similar time", resource.Kind, resource.Name)
		reason := "No ownership or management relationship to failed resource"

		excluded = append(excluded, ExcludedHypothesis{
			Resource: SymptomResource{
				UID:       resource.UID,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
				Name:      resource.Name,
			},
			Hypothesis:     hypothesis,
			ReasonExcluded: reason,
		})

		// Limit to 3 alternatives
		if len(excluded) >= 3 {
			break
		}
	}

	return excluded
}
