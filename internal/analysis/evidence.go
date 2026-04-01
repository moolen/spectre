package analysis

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

const edgeTypeManages = "MANAGES"

// collectSupportingEvidence consolidates evidence from the causal graph
func (a *RootCauseAnalyzer) collectSupportingEvidence(
	graph CausalGraph,
	rootCause *RootCauseHypothesis,
) []EvidenceItem {
	evidence := []EvidenceItem{}
	seenTypes := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.RelationshipType == edgeTypeManages && !seenTypes["MANAGES"] {
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

	spineNodeCount := 0
	for _, node := range graph.Nodes {
		if node.NodeType == nodeTypeSpine {
			spineNodeCount++
		}
	}

	if spineNodeCount > 1 && !seenTypes["STRUCTURAL"] {
		spineNodes := []GraphNode{}
		for _, node := range graph.Nodes {
			if node.NodeType == nodeTypeSpine {
				spineNodes = append(spineNodes, node)
			}
		}

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
	if symptom.Resource.Namespace == "" {
		return nil
	}

	const lookback = int64(10 * time.Minute)

	graphData, err := a.store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   symptom.Resource.Namespace,
		TimestampNs: failureTimestamp,
		LookbackNs:  lookback,
		Limit:       500,
	})
	if err != nil || graphData == nil {
		a.logger.Debug("Failed to query excluded alternatives: %v", err)
		return nil
	}

	resourceByUID := make(map[string]analysisstore.NamespaceGraphNode, len(graphData.Graph.Nodes))
	resourceUIDs := make([]string, 0, len(graphData.Graph.Nodes))
	for _, node := range graphData.Graph.Nodes {
		if node.UID == rootCause.Resource.UID {
			continue
		}
		resourceByUID[node.UID] = node
		resourceUIDs = append(resourceUIDs, node.UID)
	}

	if len(resourceUIDs) == 0 {
		return nil
	}

	eventData, err := a.store.GetChangeEvents(ctx, resourceUIDs, analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         lookback,
	})
	if err != nil {
		a.logger.Debug("Failed to query excluded alternatives: %v", err)
		return nil
	}

	type excludedCandidate struct {
		resource analysisstore.NamespaceGraphNode
		event    ChangeEventInfo
	}

	candidates := []excludedCandidate{}
	for uid, events := range eventData {
		resource, ok := resourceByUID[uid]
		if !ok {
			continue
		}
		for _, event := range convertStoreChangeEventList(events) {
			candidates = append(candidates, excludedCandidate{
				resource: resource,
				event:    event,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].event.Timestamp.After(candidates[j].event.Timestamp)
	})

	excluded := []ExcludedHypothesis{}
	seenUIDs := make(map[string]bool)
	for _, candidate := range candidates {
		if seenUIDs[candidate.resource.UID] {
			continue
		}
		seenUIDs[candidate.resource.UID] = true

		hypothesis := fmt.Sprintf("%s '%s' changed at similar time", candidate.resource.Kind, candidate.resource.Name)
		reason := "No ownership or management relationship to failed resource"

		excluded = append(excluded, ExcludedHypothesis{
			Resource: SymptomResource{
				UID:       candidate.resource.UID,
				Kind:      candidate.resource.Kind,
				Namespace: candidate.resource.Namespace,
				Name:      candidate.resource.Name,
			},
			Hypothesis:     hypothesis,
			ReasonExcluded: reason,
		})

		if len(excluded) >= 3 {
			break
		}
	}

	return excluded
}
