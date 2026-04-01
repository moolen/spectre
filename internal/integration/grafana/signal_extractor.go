package grafana

import (
	"fmt"
)

// ExtractSignalsFromPanel transforms a single panel's queries into SignalAnchors.
// Each panel target (query) is parsed, classified, and linked to K8s workloads.
//
// Key behaviors:
// - Parses each panel target's PromQL expression
// - Classifies each metric using ClassifyMetric
// - Filters out low-confidence (< 0.5) classifications
// - Infers workload from label selectors using InferWorkloadFromLabels
// - Inherits quality score from source dashboard
// - Generates unique QueryID for graph linking
//
// Returns:
// - []SignalAnchor: One anchor per classified metric (may be multiple per panel)
// - error: Parse failures or other errors
func ExtractSignalsFromPanel(
	dashboard *GrafanaDashboard,
	panel GrafanaPanel,
	qualityScore float64,
	integrationName string,
	now int64,
) ([]SignalAnchor, error) {
	var signals []SignalAnchor

	// Process each target (query) in the panel
	for _, target := range panel.Targets {
		// Skip empty queries
		if target.Expr == "" {
			continue
		}

		// Parse PromQL to extract semantic information
		extraction, err := ExtractFromPromQL(target.Expr)
		if err != nil {
			// Graceful degradation: log warning and skip unparseable queries
			// Don't fail entire panel extraction due to one bad query
			continue
		}

		// Skip queries with no concrete metric names (variables or parse failures)
		if len(extraction.MetricNames) == 0 {
			continue
		}

		// Classify each metric in the query
		for _, metricName := range extraction.MetricNames {
			// Classify the metric using 5-layer classifier
			classification := ClassifyMetric(metricName, extraction, panel.Title)

			// Filter out low-confidence classifications (< 0.5 threshold)
			if classification.Confidence < 0.5 {
				continue
			}

			// Infer workload from label selectors
			workloadInference := InferWorkloadFromLabels(extraction.LabelSelectors)

			// Extract namespace and workload name (may be empty for unlinked signals)
			namespace := ""
			workloadName := ""
			if workloadInference != nil {
				namespace = workloadInference.Namespace
				workloadName = workloadInference.WorkloadName
			}

			// Generate unique QueryID for graph linking
			queryID := fmt.Sprintf("%s-%d-%s", dashboard.UID, panel.ID, target.RefID)

			// Calculate TTL: 7 days from now
			expiresAt := now + (7 * 24 * 60 * 60 * 1_000_000_000) // 7 days in nanoseconds

			// Create SignalAnchor
			signal := SignalAnchor{
				MetricName:        metricName,
				Role:              classification.Role,
				Confidence:        classification.Confidence,
				QualityScore:      qualityScore,
				WorkloadNamespace: namespace,
				WorkloadName:      workloadName,
				DashboardUID:      dashboard.UID,
				PanelID:           panel.ID,
				QueryID:           queryID,
				SourceGrafana:     integrationName,
				FirstSeen:         now,
				LastSeen:          now,
				ExpiresAt:         expiresAt,
			}

			signals = append(signals, signal)
		}
	}

	return signals, nil
}

// ExtractSignalsFromDashboard transforms all panels in a dashboard into SignalAnchors.
// Applies deduplication by composite key (metric_name + namespace + workload_name).
// When duplicates exist, highest quality score wins.
//
// Key behaviors:
// - Iterates through all panels calling ExtractSignalsFromPanel
// - Deduplicates by composite key: metric_name + namespace + workload_name
// - Selects highest quality signal when duplicates found
// - Updates LastSeen timestamp on duplicates
//
// Returns:
// - []SignalAnchor: Deduplicated signals across all panels
// - error: Fatal errors during extraction
func ExtractSignalsFromDashboard(
	dashboard *GrafanaDashboard,
	qualityScore float64,
	integrationName string,
	now int64,
) ([]SignalAnchor, error) {
	// Map for deduplication: key = metric_name + namespace + workload_name
	signalMap := make(map[string]SignalAnchor)

	// Extract signals from each panel
	for _, panel := range dashboard.Panels {
		panelSignals, err := ExtractSignalsFromPanel(dashboard, panel, qualityScore, integrationName, now)
		if err != nil {
			// Graceful degradation: continue with other panels
			continue
		}

		// Deduplicate signals
		for _, signal := range panelSignals {
			// Generate composite key
			key := fmt.Sprintf("%s|%s|%s", signal.MetricName, signal.WorkloadNamespace, signal.WorkloadName)

			// Check if signal already exists
			if existing, exists := signalMap[key]; exists {
				// Keep signal with higher quality score
				if signal.QualityScore > existing.QualityScore {
					// Update LastSeen from existing signal (preserve earliest FirstSeen)
					signal.FirstSeen = existing.FirstSeen
					signal.LastSeen = now
					signalMap[key] = signal
				} else {
					// Keep existing, update LastSeen
					existing.LastSeen = now
					signalMap[key] = existing
				}
			} else {
				// New signal, add to map
				signalMap[key] = signal
			}
		}
	}

	// Convert map to slice
	signals := make([]SignalAnchor, 0, len(signalMap))
	for _, signal := range signalMap {
		signals = append(signals, signal)
	}

	return signals, nil
}
