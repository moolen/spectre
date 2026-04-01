package grafana

import (
	"math"
)

// AnomalyScore represents the result of anomaly detection for a signal value.
// Score ranges from 0.0 (normal) to 1.0 (highly anomalous).
// Threshold for anomaly is 0.5 per CONTEXT.md.
type AnomalyScore struct {
	// Score is the normalized anomaly score (0.0-1.0).
	// >= 0.5 indicates anomalous per CONTEXT.md threshold.
	Score float64

	// Confidence represents statistical confidence in the score.
	// Calculated as MIN(sampleConfidence, qualityScore).
	// - sampleConfidence = min(1.0, 0.5 + (sampleCount-10)/180.0)
	// - qualityScore comes from SignalAnchor's dashboard quality
	Confidence float64

	// Method indicates which scoring method produced the final score.
	// Values: "z-score", "percentile", or "alert-override"
	Method string

	// ZScore is the raw z-score for debugging and analysis.
	// zScore = (currentValue - mean) / stddev
	ZScore float64
}

// ComputeAnomalyScore computes an anomaly score using hybrid z-score + percentile comparison.
// The final score is MAX of both methods (per CONTEXT.md: "anomaly if EITHER method flags it").
//
// Z-Score Method (ANOM-01):
//   - zScore = (currentValue - mean) / stddev
//   - Normalized: zScoreNormalized = 1.0 - exp(-|zScore|/2.0)
//   - z=2 -> ~0.63, z=3 -> ~0.78 (sigmoid-like mapping to 0-1 range)
//
// Percentile Method (ANOM-02):
//   - If currentValue > P99: score starts at 0.5, scales up based on distance
//   - If currentValue < Min: score starts at 0.5, scales up based on distance
//   - Otherwise: 0.0
//
// Confidence (ANOM-03):
//   - sampleConfidence = min(1.0, 0.5 + (sampleCount-10)/180.0)
//   - confidence = MIN(sampleConfidence, qualityScore)
//
// Cold Start (ANOM-04):
//   - If sampleCount < MinSamplesRequired (10): returns InsufficientSamplesError
//
// Parameters:
//   - currentValue: The current metric value to score
//   - baseline: SignalBaseline with rolling statistics (must have >= 10 samples)
//   - qualityScore: Dashboard quality score (0.0-1.0) from SignalAnchor
//
// Returns:
//   - *AnomalyScore with computed score, confidence, method, and raw z-score
//   - error if baseline has insufficient samples (cold start)
func ComputeAnomalyScore(currentValue float64, baseline SignalBaseline, qualityScore float64) (*AnomalyScore, error) {
	// Cold start check (ANOM-04): require minimum samples
	if baseline.SampleCount < MinSamplesRequired {
		return nil, &InsufficientSamplesError{
			Available: baseline.SampleCount,
			Required:  MinSamplesRequired,
		}
	}

	// Compute z-score (ANOM-01)
	var zScore float64
	if baseline.StdDev > 0 {
		zScore = (currentValue - baseline.Mean) / baseline.StdDev
	}
	// If stddev == 0, zScore remains 0 (all values identical)

	// Normalize z-score to 0-1 range using sigmoid-like mapping
	// zScoreNormalized = 1.0 - exp(-|zScore|/2.0)
	// This maps: z=0 -> 0, z=2 -> ~0.63, z=3 -> ~0.78, z->inf -> 1.0
	zScoreNormalized := 1.0 - math.Exp(-math.Abs(zScore)/2.0)

	// Compute percentile score (ANOM-02)
	var percentileScore float64

	if currentValue > baseline.P99 && baseline.P99 > baseline.P50 {
		// Value exceeds P99 - score starts at 0.5, scales with distance
		excess := currentValue - baseline.P99
		range99 := baseline.P99 - baseline.P50
		percentileScore = math.Min(1.0, 0.5+(excess/range99)*0.5)
	} else if currentValue < baseline.Min {
		// Value below minimum - also anomalous
		deficit := baseline.Min - currentValue
		rangeLow := baseline.P50 - baseline.Min
		if rangeLow > 0 {
			percentileScore = math.Min(1.0, 0.5+(deficit/rangeLow)*0.5)
		} else {
			// P50 == Min edge case: just flag as anomalous
			percentileScore = 0.5
		}
	}

	// Hybrid score = MAX of both methods (per CONTEXT.md)
	score := math.Max(zScoreNormalized, percentileScore)

	// Determine which method dominated
	method := "z-score"
	if percentileScore > zScoreNormalized {
		method = "percentile"
	}

	// Compute confidence (ANOM-03)
	// sampleConfidence scales from 0.5 at 10 samples to 1.0 at 190 samples
	// Formula: min(1.0, 0.5 + (sampleCount-10)/180.0)
	sampleConfidence := math.Min(1.0, 0.5+float64(baseline.SampleCount-MinSamplesRequired)/180.0)

	// Final confidence is MIN of sample confidence and quality score
	confidence := math.Min(sampleConfidence, qualityScore)

	return &AnomalyScore{
		Score:      score,
		Confidence: confidence,
		Method:     method,
		ZScore:     zScore,
	}, nil
}

// ApplyAlertOverride modifies an anomaly score based on Grafana alert state.
// If alert is firing, the score is overridden to 1.0 with confidence 1.0.
// This implements ANOM-06: "Grafana alert firing -> override anomaly score to 1.0".
//
// Per CONTEXT.md: "Human already decided" - a firing alert is a definitive signal,
// not probabilistic. The computed z-score is preserved for debugging.
//
// Parameters:
//   - score: The computed AnomalyScore to potentially override
//   - alertState: Grafana alert state ("firing", "pending", "normal", or "")
//
// Returns:
//   - If alertState == "firing": new AnomalyScore with Score=1.0, Confidence=1.0, Method="alert-override"
//   - Otherwise: the original score unchanged
func ApplyAlertOverride(score *AnomalyScore, alertState string) *AnomalyScore {
	if alertState == "firing" {
		return &AnomalyScore{
			Score:      1.0,
			Confidence: 1.0,
			Method:     "alert-override",
			ZScore:     score.ZScore, // Preserve for debugging
		}
	}
	return score
}
