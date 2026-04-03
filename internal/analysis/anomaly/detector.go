package anomaly

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/logging"
)

// AnomalyDetector orchestrates anomaly detection across the causal subgraph
type AnomalyDetector struct {
	analyzer *analysis.RootCauseAnalyzer
	logger   *logging.Logger

	// Sub-detectors
	eventDetector     *EventAnomalyDetector
	stateDetector     *StateAnomalyDetector
	changeDetector    *ChangeAnomalyDetector
	frequencyDetector *FrequencyAnomalyDetector
	configDetector    *ConfigAnomalyDetector
	networkDetector   *NetworkAnomalyDetector
}

// NewDetector creates a new anomaly detector
func NewDetector(store analysisstore.AnalysisStore) *AnomalyDetector {
	return &AnomalyDetector{
		analyzer:          analysis.NewRootCauseAnalyzer(store),
		logger:            logging.GetLogger("anomaly.detector"),
		eventDetector:     NewEventAnomalyDetector(),
		stateDetector:     NewStateAnomalyDetector(),
		changeDetector:    NewChangeAnomalyDetector(),
		frequencyDetector: NewFrequencyAnomalyDetector(),
		configDetector:    NewConfigAnomalyDetector(),
		networkDetector:   NewNetworkAnomalyDetector(),
	}
}

// DetectInput contains the parameters for anomaly detection
type DetectInput struct {
	ResourceUID string
	Start       int64 // Unix seconds
	End         int64 // Unix seconds
}

// Detect analyzes a resource's causal subgraph for anomalies
func (d *AnomalyDetector) Detect(ctx context.Context, input DetectInput) (*AnomalyResponse, error) {
	timeWindow := TimeWindow{
		Start: time.Unix(input.Start, 0),
		End:   time.Unix(input.End, 0),
	}

	// Convert to nanoseconds for analyzer
	failureTimestampNs := input.End * 1_000_000_000
	lookbackNs := (input.End - input.Start) * 1_000_000_000

	d.logger.Debug("Detecting anomalies for resource %s, time window: %v to %v",
		input.ResourceUID, timeWindow.Start, timeWindow.End)

	// Use the existing analyzer to fetch the causal subgraph
	analyzeInput := analysis.AnalyzeInput{
		ResourceUID:      input.ResourceUID,
		FailureTimestamp: failureTimestampNs,
		LookbackNs:       lookbackNs,
		MaxDepth:         5,
		MinConfidence:    0.5,
		Format:           analysis.FormatDiff,
	}

	result, err := d.analyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		// Check if this is a "no data in range" error with a hint
		var noDataErr *analysis.ErrNoChangeEventInRange
		if errors.As(err, &noDataErr) {
			// Return success with empty anomalies and a hint
			d.logger.Debug("No data in requested time range, returning hint: %s", noDataErr.Hint())
			return &AnomalyResponse{
				Anomalies: []Anomaly{},
				Metadata: ResponseMetadata{
					ResourceUID: input.ResourceUID,
					TimeWindow:  timeWindow,
					Hint:        noDataErr.Hint(),
				},
			}, nil
		}

		d.logger.Error("Failed to analyze causal graph: %v", err)
		return nil, fmt.Errorf("failed to analyze causal graph: %w", err)
	}

	d.logger.Debug("Causal graph has %d nodes", len(result.Incident.Graph.Nodes))

	// Collect all anomalies from all nodes
	var allAnomalies []Anomaly

	// Build a map of node anomalies for graph-level detection
	nodeAnomaliesMap := make(map[string][]Anomaly)

	for i := range result.Incident.Graph.Nodes {
		node := &result.Incident.Graph.Nodes[i]

		detectorInput := DetectorInput{
			Node:       node,
			TimeWindow: timeWindow,
			AllEvents:  node.AllEvents,
			K8sEvents:  node.K8sEvents,
		}

		var nodeAnomalies []Anomaly

		// Run all detectors
		eventAnomalies := d.eventDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d event anomalies",
			node.Resource.Name, node.Resource.Kind, len(eventAnomalies))
		nodeAnomalies = append(nodeAnomalies, eventAnomalies...)

		stateAnomalies := d.stateDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d state anomalies",
			node.Resource.Name, node.Resource.Kind, len(stateAnomalies))
		nodeAnomalies = append(nodeAnomalies, stateAnomalies...)

		changeAnomalies := d.changeDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d change anomalies",
			node.Resource.Name, node.Resource.Kind, len(changeAnomalies))
		nodeAnomalies = append(nodeAnomalies, changeAnomalies...)

		frequencyAnomalies := d.frequencyDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d frequency anomalies",
			node.Resource.Name, node.Resource.Kind, len(frequencyAnomalies))
		nodeAnomalies = append(nodeAnomalies, frequencyAnomalies...)

		configAnomalies := d.configDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d config anomalies",
			node.Resource.Name, node.Resource.Kind, len(configAnomalies))
		nodeAnomalies = append(nodeAnomalies, configAnomalies...)

		networkAnomalies := d.networkDetector.Detect(detectorInput)
		d.logger.Debug("Node %s (%s): %d network anomalies",
			node.Resource.Name, node.Resource.Kind, len(networkAnomalies))
		nodeAnomalies = append(nodeAnomalies, networkAnomalies...)

		// Store for graph-level detection
		nodeAnomaliesMap[node.ID] = nodeAnomalies
		allAnomalies = append(allAnomalies, nodeAnomalies...)
	}

	// Run graph-level detection (e.g., Service with no ready endpoints)
	graphAnomalies := d.detectGraphLevelAnomalies(result.Incident.Graph, timeWindow, nodeAnomaliesMap)
	d.logger.Debug("Graph-level anomalies: %d", len(graphAnomalies))
	allAnomalies = append(allAnomalies, graphAnomalies...)

	// Deduplicate anomalies (same node + type + timestamp)
	anomalies := deduplicateAnomalies(allAnomalies)

	d.logger.Debug("Total anomalies detected: %d (after deduplication)", len(anomalies))

	return &AnomalyResponse{
		Anomalies: anomalies,
		Metadata: ResponseMetadata{
			ResourceUID:   input.ResourceUID,
			TimeWindow:    timeWindow,
			NodesAnalyzed: len(result.Incident.Graph.Nodes),
		},
	}, nil
}
