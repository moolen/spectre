package causalpaths

import (
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// PathRanker handles ranking of causal paths
type PathRanker struct{}

// NewPathRanker creates a new PathRanker instance
func NewPathRanker() *PathRanker {
	return &PathRanker{}
}

// RankPaths sorts paths by composite score (higher = more likely root cause)
func (r *PathRanker) RankPaths(paths []CausalPath, symptomFirstFailure time.Time) []CausalPath {
	// Score each path
	for i := range paths {
		paths[i].Ranking = r.calculateRanking(paths[i], symptomFirstFailure)
		paths[i].ConfidenceScore = r.calculateConfidenceScore(paths[i])
	}

	// Sort by confidence score descending, with tie-breakers:
	// 1. Definitive root causes (ResourceDeleted, etc.) rank higher
	// 2. Earlier first anomaly ranks higher
	// 3. Longer path (deeper root cause) ranks higher
	sort.Slice(paths, func(i, j int) bool {
		// Primary: confidence score
		if paths[i].ConfidenceScore != paths[j].ConfidenceScore {
			return paths[i].ConfidenceScore > paths[j].ConfidenceScore
		}

		// Tie-breaker 1: Definitive root cause wins
		iDefinitive := HasDefinitiveRootCauseAnomaly(paths[i].CandidateRoot.Anomalies)
		jDefinitive := HasDefinitiveRootCauseAnomaly(paths[j].CandidateRoot.Anomalies)
		if iDefinitive != jDefinitive {
			return iDefinitive
		}

		// Tie-breaker 2: Earlier first anomaly wins (more likely to be the root cause)
		if !paths[i].FirstAnomalyAt.Equal(paths[j].FirstAnomalyAt) {
			return paths[i].FirstAnomalyAt.Before(paths[j].FirstAnomalyAt)
		}

		// Tie-breaker 3: Longer path (deeper root cause) wins
		return len(paths[i].Steps) > len(paths[j].Steps)
	})

	return paths
}

// calculateRanking computes the ranking factors for a path
func (r *PathRanker) calculateRanking(path CausalPath, symptomFirstFailure time.Time) PathRanking {
	// 1. Temporal Score: Earlier anomaly = higher score
	temporalScore := r.calculateTemporalScore(path.FirstAnomalyAt, symptomFirstFailure)

	// 2. Effective Causal Distance: Count only cause-introducing edges
	effectiveDistance := r.calculateEffectiveCausalDistance(path)

	// 3. Max Anomaly Severity in path
	maxSeverity, severityScore := r.calculateSeverityScore(path)

	// Generate human-readable explanations
	temporalExplanation := r.generateTemporalExplanation(path.FirstAnomalyAt, symptomFirstFailure)
	distanceExplanation := r.generateDistanceExplanation(effectiveDistance, path)
	severityExplanation := r.generateSeverityExplanation(maxSeverity)
	rankingExplanation := r.generateRankingExplanation(temporalExplanation, distanceExplanation, severityExplanation, temporalScore, effectiveDistance, severityScore)

	return PathRanking{
		TemporalScore:           temporalScore,
		EffectiveCausalDistance: effectiveDistance,
		MaxAnomalySeverity:      maxSeverity,
		SeverityScore:           severityScore,
		RankingExplanation:      rankingExplanation,
		TemporalExplanation:     temporalExplanation,
		DistanceExplanation:     distanceExplanation,
		SeverityExplanation:     severityExplanation,
	}
}

// calculateConfidenceScore computes the composite confidence score
// This takes the full CausalPath to allow intent owner boosting
func (r *PathRanker) calculateConfidenceScore(path CausalPath) float64 {
	ranking := path.Ranking

	// Temporal: Already 0.0-1.0 where higher = earlier (better)
	temporalComponent := ranking.TemporalScore * WeightTemporal

	// Check if this is a definitive root cause (like ResourceDeleted)
	// Definitive root causes should NOT be penalized for longer path distance
	// because deeper root causes are MORE valuable, not less
	isDefinitiveRootCause := HasDefinitiveRootCauseAnomaly(path.CandidateRoot.Anomalies)

	// Distance: Invert so shorter = higher score
	// BUT: For definitive root causes, don't penalize for longer paths
	var distanceComponent float64
	if isDefinitiveRootCause {
		// For definitive root causes, give full distance score
		// The longer path to the actual root cause is a feature, not a bug
		distanceComponent = 1.0 * WeightDistance
	} else {
		distanceNormalized := 1.0 - float64(ranking.EffectiveCausalDistance)/float64(MaxEffectiveCausalDistance)
		if distanceNormalized < 0 {
			distanceNormalized = 0
		}
		distanceComponent = distanceNormalized * WeightDistance
	}

	// Severity: Already 0.0-1.0
	severityComponent := ranking.SeverityScore * WeightSeverity

	baseScore := temporalComponent + distanceComponent + severityComponent

	// Apply intent owner boost if the root cause is an intent owner
	rootKind := path.CandidateRoot.Resource.Kind
	intentBoost := 0.0
	if IsIntentOwner(rootKind) {
		intentBoost += IntentOwnerBoost
		// Additional boost for GitOps controllers
		if IsGitOpsController(rootKind) {
			intentBoost += GitOpsControllerBoost
		}
	}

	// Apply definitive root cause boost
	// This ensures that deleted ConfigMaps/Secrets rank higher than the GitOps controllers
	// that reference them, since the deletion is the actual root cause
	definitiveBoost := 0.0
	if isDefinitiveRootCause {
		definitiveBoost = DefinitiveRootCauseBoost
	}

	// Cap final score at 1.0
	finalScore := baseScore + intentBoost + definitiveBoost
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	return finalScore
}

// calculateTemporalScore computes temporal proximity score
// Earlier anomaly = higher score (root causes precede symptoms)
// Anomalies occurring AFTER failure get score 0 (can't be cause)
//
// Scoring rationale:
// - Anomalies very close to failure (< 5s) are likely symptoms, not causes
// - Root causes typically precede failures by seconds to minutes
// - We use a bell-curve-like scoring that peaks around 30s-5min before failure
// - GitOps reconciliation can take 5-10 minutes, so we have a gentler decay beyond 5min
func (r *PathRanker) calculateTemporalScore(anomalyTime, failureTime time.Time) float64 {
	// If anomaly is after failure, it can't be the cause
	if anomalyTime.After(failureTime) {
		return 0
	}

	// Calculate time lag between anomaly and failure
	lag := failureTime.Sub(anomalyTime)
	lagNs := lag.Nanoseconds()

	// Define scoring bands based on typical Kubernetes propagation delays:
	// - GitOps -> Workload: 5-60s (but can be 5-10 min for complex reconciliations)
	// - Deployment -> Pod: 5-30s
	// - ConfigMap change -> Pod restart: 30s-5min
	// - Node condition -> Pod eviction: 30s-3min

	const (
		// Anomalies < 5s before failure are likely symptoms (low score)
		tooCloseThresholdNs = int64(5 * time.Second)
		// Sweet spot: 30s to 5min before failure (highest scores)
		// Extended from 3min to 5min to better accommodate GitOps reconciliation delays
		optimalMinNs = int64(30 * time.Second)
		optimalMaxNs = int64(5 * time.Minute)
	)

	// If anomaly is too close to failure, it's likely a symptom
	if lagNs < tooCloseThresholdNs {
		// Linear scale from 0.3 (at 0s) to 0.5 (at 5s)
		return 0.3 + (float64(lagNs)/float64(tooCloseThresholdNs))*0.2
	}

	// Optimal range: 30s to 5min before failure
	if lagNs >= optimalMinNs && lagNs <= optimalMaxNs {
		// Peak score of 1.0 in the optimal range
		return 1.0
	}

	// Between 5s and 30s: ramp up from 0.5 to 1.0
	if lagNs < optimalMinNs {
		progress := float64(lagNs-tooCloseThresholdNs) / float64(optimalMinNs-tooCloseThresholdNs)
		return 0.5 + progress*0.5
	}

	// Beyond 5min: gradually decrease but stay above 0.6
	// Root causes can precede failures by many minutes (especially GitOps), so don't penalize too much
	// Gentler decay than before: 0.3 reduction over remaining window (was 0.5)
	if lagNs > optimalMaxNs {
		remainingWindow := MaxTemporalLagNs - optimalMaxNs
		if remainingWindow <= 0 {
			return 0.6
		}
		excess := lagNs - optimalMaxNs
		// Slower decay: reduce by 0.3 over the remaining window (was 0.5)
		decay := float64(excess) / float64(remainingWindow) * 0.3
		score := 1.0 - decay
		// Higher floor: 0.6 instead of 0.5
		if score < 0.6 {
			return 0.6
		}
		return score
	}

	return 0.5 // Fallback
}

// calculateEffectiveCausalDistance counts cause-introducing edges
// Ownership edges (OWNS, etc.) do not count
func (r *PathRanker) calculateEffectiveCausalDistance(path CausalPath) int {
	distance := 0
	for _, step := range path.Steps {
		if step.Edge != nil && step.Edge.EdgeCategory == EdgeCategoryCauseIntroducing {
			distance++
		}
	}
	return distance
}

// calculateSeverityScore finds the maximum severity and returns numeric score
func (r *PathRanker) calculateSeverityScore(path CausalPath) (string, float64) {
	severityRank := map[anomaly.Severity]float64{
		anomaly.SeverityCritical: 1.0,
		anomaly.SeverityHigh:     0.75,
		anomaly.SeverityMedium:   0.5,
		anomaly.SeverityLow:      0.25,
	}

	maxSeverity := anomaly.SeverityLow
	maxScore := 0.25

	for _, step := range path.Steps {
		for _, a := range step.Node.Anomalies {
			if score, ok := severityRank[a.Severity]; ok && score > maxScore {
				maxScore = score
				maxSeverity = a.Severity
			}
		}
	}

	return string(maxSeverity), maxScore
}

// generateTemporalExplanation creates a human-readable explanation of the temporal score
func (r *PathRanker) generateTemporalExplanation(anomalyTime, failureTime time.Time) string {
	if anomalyTime.After(failureTime) {
		return "anomaly occurred after failure (cannot be cause)"
	}

	lag := failureTime.Sub(anomalyTime)

	// Format duration in human-readable form
	var lagStr string
	if lag < time.Second {
		lagStr = fmt.Sprintf("%dms", lag.Milliseconds())
	} else if lag < time.Minute {
		lagStr = fmt.Sprintf("%.1fs", lag.Seconds())
	} else if lag < time.Hour {
		lagStr = fmt.Sprintf("%.1fm", lag.Minutes())
	} else {
		lagStr = fmt.Sprintf("%.1fh", lag.Hours())
	}

	switch {
	case lag < 5*time.Second:
		return fmt.Sprintf("anomaly %s before failure (very close, likely symptom)", lagStr)
	case lag < 30*time.Second:
		return fmt.Sprintf("anomaly %s before failure (close, possibly propagation delay)", lagStr)
	case lag <= 3*time.Minute:
		return fmt.Sprintf("anomaly %s before failure (optimal range for root cause)", lagStr)
	default:
		return fmt.Sprintf("anomaly %s before failure (early, but still plausible)", lagStr)
	}
}

// generateDistanceExplanation creates a human-readable explanation of the causal distance
func (r *PathRanker) generateDistanceExplanation(distance int, path CausalPath) string {
	if distance == 0 {
		return "direct cause (no intermediate cause-introducing edges)"
	}

	// Count edge types
	edgeTypes := make(map[string]int)
	for _, step := range path.Steps {
		if step.Edge != nil && step.Edge.EdgeCategory == EdgeCategoryCauseIntroducing {
			edgeTypes[step.Edge.RelationshipType]++
		}
	}

	if len(edgeTypes) == 1 {
		for edgeType, count := range edgeTypes {
			if count == 1 {
				return fmt.Sprintf("1 hop via %s relationship", edgeType)
			}
			return fmt.Sprintf("%d hops via %s relationships", count, edgeType)
		}
	}

	return fmt.Sprintf("%d cause-introducing hops", distance)
}

// generateSeverityExplanation creates a human-readable explanation of the severity
func (r *PathRanker) generateSeverityExplanation(severity string) string {
	switch severity {
	case string(anomaly.SeverityCritical):
		return "critical severity anomaly detected"
	case string(anomaly.SeverityHigh):
		return "high severity anomaly detected"
	case string(anomaly.SeverityMedium):
		return "medium severity anomaly detected"
	case string(anomaly.SeverityLow):
		return "low severity anomaly detected"
	default:
		return fmt.Sprintf("anomaly severity: %s", severity)
	}
}

// generateRankingExplanation creates an overall explanation combining all factors
func (r *PathRanker) generateRankingExplanation(
	temporalExpl, distanceExpl, severityExpl string,
	temporalScore float64, distance int, severityScore float64,
) string {
	// Identify the strongest contributing factor
	temporalWeight := temporalScore * WeightTemporal
	distanceNorm := 1.0 - float64(distance)/float64(MaxEffectiveCausalDistance)
	if distanceNorm < 0 {
		distanceNorm = 0
	}
	distanceWeight := distanceNorm * WeightDistance
	severityWeight := severityScore * WeightSeverity

	// Build explanation starting with strongest factor
	type factor struct {
		name   string
		weight float64
		expl   string
	}
	factors := []factor{
		{"temporal", temporalWeight, temporalExpl},
		{"distance", distanceWeight, distanceExpl},
		{"severity", severityWeight, severityExpl},
	}

	// Sort by weight descending
	sort.Slice(factors, func(i, j int) bool {
		return factors[i].weight > factors[j].weight
	})

	// Generate explanation emphasizing top factor
	topFactor := factors[0]

	switch topFactor.name {
	case "temporal":
		if temporalScore >= 0.9 {
			return fmt.Sprintf("Ranked high because: %s", temporalExpl)
		}
		return fmt.Sprintf("Temporal factor: %s", temporalExpl)
	case "distance":
		if distance == 0 {
			return fmt.Sprintf("Ranked high because: %s", distanceExpl)
		}
		return fmt.Sprintf("Distance factor: %s", distanceExpl)
	case "severity":
		return fmt.Sprintf("Ranked due to severity: %s", severityExpl)
	default:
		return fmt.Sprintf("Ranking based on: %s; %s; %s", temporalExpl, distanceExpl, severityExpl)
	}
}
