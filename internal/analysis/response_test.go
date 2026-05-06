package analysis

import (
	"testing"
	"time"
)

func TestBuildAnalysisResponse_AppliesDiffFormat(t *testing.T) {
	analyzer := &RootCauseAnalyzer{}
	failureTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	eventTime := failureTime.Add(-10 * time.Second)
	data := []byte(`{"spec":{"replicas":3}}`)

	result := analyzer.buildAnalysisResponse(
		AnalyzeInput{
			FailureTimestamp: failureTime.UnixNano(),
			Format:           FormatDiff,
		},
		&ObservedSymptom{
			ErrorMessage: "replicas mismatch",
		},
		CausalGraph{
			Nodes: []GraphNode{
				{
					NodeType: nodeTypeSpine,
					Resource: SymptomResource{Kind: "Deployment"},
					ChangeEvent: &ChangeEventInfo{
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Data:          data,
					},
					AllEvents: []ChangeEventInfo{
						{
							Timestamp:     eventTime,
							EventType:     "UPDATE",
							ConfigChanged: true,
							Data:          data,
						},
					},
				},
			},
		},
		&RootCauseHypothesis{
			Resource: SymptomResource{Kind: "Deployment"},
			ChangeEvent: ChangeEventInfo{
				Timestamp:     eventTime,
				EventType:     "UPDATE",
				ConfigChanged: true,
				Data:          data,
			},
		},
		ConfidenceScore{},
		nil,
		nil,
		ResultQuality{},
		&PerformanceMetrics{},
		failureTime,
		42,
	)

	nodeEvent := result.Incident.Graph.Nodes[0].ChangeEvent
	if nodeEvent.Significance == nil {
		t.Fatal("expected graph node change event significance")
	}
	if nodeEvent.Data != nil {
		t.Fatal("expected graph node change event legacy data to be cleared")
	}
	if len(result.Incident.Graph.Nodes[0].AllEvents[0].Diff) == 0 && len(result.Incident.Graph.Nodes[0].AllEvents[0].FullSnapshot) == 0 {
		t.Fatal("expected graph node all events to be converted to diff format")
	}
	if result.Incident.RootCause.ChangeEvent.Significance == nil {
		t.Fatal("expected root cause change event significance")
	}
	if result.Incident.RootCause.ChangeEvent.Data != nil {
		t.Fatal("expected root cause change event legacy data to be cleared")
	}
}

func TestBuildAnalysisResponse_PreservesLegacyFormat(t *testing.T) {
	analyzer := &RootCauseAnalyzer{}
	failureTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	data := []byte(`{"status":"failed"}`)

	result := analyzer.buildAnalysisResponse(
		AnalyzeInput{
			FailureTimestamp: failureTime.UnixNano(),
			Format:           FormatLegacy,
		},
		&ObservedSymptom{},
		CausalGraph{
			Nodes: []GraphNode{
				{
					NodeType: nodeTypeSpine,
					Resource: SymptomResource{Kind: "Pod"},
					ChangeEvent: &ChangeEventInfo{
						Timestamp: failureTime,
						EventType: "UPDATE",
						Data:      data,
					},
				},
			},
		},
		&RootCauseHypothesis{
			Resource: SymptomResource{Kind: "Pod"},
			ChangeEvent: ChangeEventInfo{
				Timestamp: failureTime,
				EventType: "UPDATE",
				Data:      data,
			},
		},
		ConfidenceScore{},
		nil,
		nil,
		ResultQuality{},
		&PerformanceMetrics{},
		failureTime,
		42,
	)

	if result.Incident.Graph.Nodes[0].ChangeEvent.Data == nil {
		t.Fatal("expected graph node change event legacy data to be preserved")
	}
	if result.Incident.RootCause.ChangeEvent.Data == nil {
		t.Fatal("expected root cause change event legacy data to be preserved")
	}
}
