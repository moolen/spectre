package causalpaths

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// detectAnomaliesForAllNodes runs anomaly detection for the causal subgraph.
func (d *PathDiscoverer) detectAnomaliesForAllNodes(
	ctx context.Context,
	graph analysis.CausalGraph,
	input CausalPathsInput,
) (map[string][]anomaly.Anomaly, error) {
	nodeAnomalies := make(map[string][]anomaly.Anomaly)

	startNs := input.FailureTimestamp - input.LookbackNs
	endNs := input.FailureTimestamp

	detectInput := anomaly.DetectInput{
		ResourceUID: input.ResourceUID,
		Start:       startNs / 1_000_000_000,
		End:         endNs / 1_000_000_000,
	}

	response, err := d.anomalyDetector.Detect(ctx, detectInput)
	if err != nil {
		return nil, err
	}

	uidToNodeID := make(map[string]string)
	for i := range graph.Nodes {
		uidToNodeID[graph.Nodes[i].Resource.UID] = graph.Nodes[i].ID
	}

	for _, detected := range response.Anomalies {
		if nodeID, ok := uidToNodeID[detected.Node.UID]; ok {
			nodeAnomalies[nodeID] = append(nodeAnomalies[nodeID], detected)
		}
	}

	return nodeAnomalies, nil
}

// identifyFirstFailure determines the first failure timestamp for the symptom.
func (d *PathDiscoverer) identifyFirstFailure(
	symptomAnomalies []anomaly.Anomaly,
	fallbackTimestamp int64,
) time.Time {
	var earliestNonChange time.Time

	for _, detected := range symptomAnomalies {
		if detected.Category == anomaly.CategoryChange {
			continue
		}
		if earliestNonChange.IsZero() || detected.Timestamp.Before(earliestNonChange) {
			earliestNonChange = detected.Timestamp
		}
	}

	if earliestNonChange.IsZero() {
		return time.Unix(0, fallbackTimestamp)
	}

	return earliestNonChange
}

// hasNoReadyEndpointsAnomaly checks if anomalies include NoReadyEndpoints.
func (d *PathDiscoverer) hasNoReadyEndpointsAnomaly(anomalies []anomaly.Anomaly) bool {
	for _, detected := range anomalies {
		if detected.Type == "NoReadyEndpoints" {
			return true
		}
	}
	return false
}
