package analysis

import (
	"fmt"
	"strings"
)

// calculateConfidence computes a deterministic confidence score.
// Weights are defined in constants.go and must sum to ~1.0 for proper scoring.
func (a *RootCauseAnalyzer) calculateConfidence(
	symptom *ObservedSymptom,
	graph CausalGraph,
	rootCause *RootCauseHypothesis,
) ConfidenceScore {
	// Calculate each factor
	factors := ConfidenceFactors{
		DirectSpecChange:     calculateSpecChangeFactor(rootCause),
		TemporalProximity:    calculateTemporalFactor(rootCause.TimeLagMs),
		RelationshipStrength: calculateRelationshipFactor(graph),
		ErrorMessageMatch:    calculateErrorMatchFactor(symptom),
		ChainCompleteness:    calculateCompletenessFactor(graph),
	}

	// Weighted average using constants
	score := factors.DirectSpecChange*ConfidenceWeightSpecChange +
		factors.TemporalProximity*ConfidenceWeightTemporal +
		factors.RelationshipStrength*ConfidenceWeightRelationship +
		factors.ErrorMessageMatch*ConfidenceWeightErrorMatch +
		factors.ChainCompleteness*ConfidenceWeightChain

	// Generate rationale
	rationale := generateConfidenceRationale(factors, score)

	return ConfidenceScore{
		Score:     score,
		Rationale: rationale,
		Factors:   factors,
	}
}

// calculateSpecChangeFactor: 1.0 if configChanged=true, SpecChangeFactorUpdate if UPDATE, 0.0 otherwise
func calculateSpecChangeFactor(rootCause *RootCauseHypothesis) float64 {
	if rootCause.ChangeEvent.ConfigChanged {
		return 1.0
	}
	if rootCause.ChangeEvent.EventType == "UPDATE" {
		return SpecChangeFactorUpdate
	}
	return 0.0
}

// calculateTemporalFactor: 1.0 - (timeLagMs / TemporalFactorMaxLag) capped at [0, 1]
func calculateTemporalFactor(timeLagMs int64) float64 {
	maxLagMs := float64(TemporalFactorMaxLag)
	if timeLagMs < 0 {
		timeLagMs = 0
	}
	factor := 1.0 - (float64(timeLagMs) / maxLagMs)
	if factor < 0 {
		return 0
	}
	if factor > 1 {
		return 1
	}
	return factor
}

// calculateRelationshipFactor: Uses relationship strength constants
func calculateRelationshipFactor(graph CausalGraph) float64 {
	if len(graph.Edges) == 0 {
		return 0.0
	}

	// Find the strongest relationship in the graph
	maxStrength := 0.0
	for _, edge := range graph.Edges {
		var strength float64
		switch edge.RelationshipType {
		case "MANAGES":
			strength = RelationshipStrengthManages
		case "OWNS":
			strength = RelationshipStrengthOwns
		case "TRIGGERED_BY":
			strength = RelationshipStrengthTriggeredBy
		default:
			strength = RelationshipStrengthDefault
		}
		if strength > maxStrength {
			maxStrength = strength
		}
	}
	return maxStrength
}

// calculateErrorMatchFactor: 1.0 if error mentions config/image, ErrorMatchFactorGeneric if generic, 0.0 if none
func calculateErrorMatchFactor(symptom *ObservedSymptom) float64 {
	errorLower := strings.ToLower(symptom.ErrorMessage)

	// Check if error mentions configuration or image issues
	if strings.Contains(errorLower, "image") ||
		strings.Contains(errorLower, "config") ||
		strings.Contains(errorLower, "invalid") ||
		strings.Contains(errorLower, "pull") {
		return 1.0
	}

	// Generic error messages
	if symptom.ErrorMessage != "" {
		return ErrorMatchFactorGeneric
	}

	return 0.0
}

// calculateCompletenessFactor: nodes in graph / expected nodes
func calculateCompletenessFactor(graph CausalGraph) float64 {
	// Expected: Pod <- ReplicaSet <- Deployment <- [Manager] = ChainCompletenessMinNodes
	expectedNodes := float64(ChainCompletenessMinNodes)
	actualNodes := 0.0
	for _, node := range graph.Nodes {
		if node.NodeType == nodeTypeSpine {
			actualNodes++
		}
	}

	factor := actualNodes / expectedNodes
	if factor > 1.0 {
		return 1.0
	}
	return factor
}

// generateConfidenceRationale creates a human-readable explanation of the score
func generateConfidenceRationale(factors ConfidenceFactors, score float64) string {
	rationale := fmt.Sprintf("Confidence: %.0f%%. ", score*100)

	// List contributing factors
	contributions := []string{}
	if factors.DirectSpecChange > 0.5 {
		contributions = append(contributions, "direct spec change detected")
	}
	if factors.TemporalProximity > 0.7 {
		contributions = append(contributions, "change occurred shortly before failure")
	}
	if factors.RelationshipStrength > 0.8 {
		contributions = append(contributions, "strong management relationship")
	}
	if factors.ErrorMessageMatch > 0.5 {
		contributions = append(contributions, "error message correlates")
	}
	if factors.ChainCompleteness > 0.8 {
		contributions = append(contributions, "complete causal chain")
	}

	if len(contributions) > 0 {
		rationale += "Based on: " + strings.Join(contributions, ", ") + "."
	}

	return rationale
}
