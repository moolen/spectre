package analysis

import "time"

func (a *RootCauseAnalyzer) buildAnalysisResponse(
	input AnalyzeInput,
	symptom *ObservedSymptom,
	graph CausalGraph,
	rootCause *RootCauseHypothesis,
	confidence ConfidenceScore,
	evidence []EvidenceItem,
	excluded []ExcludedHypothesis,
	quality ResultQuality,
	perfMetrics *PerformanceMetrics,
	executedAt time.Time,
	queryExecutionMs int64,
) *RootCauseAnalysisV2 {
	a.applyResponseFormat(&graph, rootCause, symptom, input)

	return &RootCauseAnalysisV2{
		Incident: IncidentAnalysis{
			ObservedSymptom: *symptom,
			Graph:           graph,
			RootCause:       *rootCause,
			Confidence:      confidence,
		},
		SupportingEvidence:   evidence,
		ExcludedAlternatives: excluded,
		QueryMetadata: QueryMetadata{
			QueryExecutionMs:   queryExecutionMs,
			AlgorithmVersion:   "v2.0-graph",
			ExecutedAt:         executedAt,
			ResultQuality:      quality,
			PerformanceMetrics: perfMetrics,
		},
	}
}

func (a *RootCauseAnalyzer) applyResponseFormat(
	graph *CausalGraph,
	rootCause *RootCauseHypothesis,
	symptom *ObservedSymptom,
	input AnalyzeInput,
) {
	if input.Format != FormatDiff {
		return
	}

	errorPatterns := ExtractErrorPatterns(symptom.ErrorMessage)
	failureTime := time.Unix(0, input.FailureTimestamp)
	a.applyDiffFormat(graph, failureTime, errorPatterns)

	if rootCause.ChangeEvent.Data == nil {
		return
	}

	rootCause.ChangeEvent.Significance = CalculateChangeEventSignificance(
		&rootCause.ChangeEvent,
		rootCause.Resource.Kind,
		true,
		failureTime,
		errorPatterns,
	)
	ConvertSingleEventToDiff(&rootCause.ChangeEvent, nil, true)
}
