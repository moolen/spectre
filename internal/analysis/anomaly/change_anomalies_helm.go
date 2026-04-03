package anomaly

import (
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

// detectHelmReleaseChanges detects specific HelmRelease change types:
// - HelmUpgrade: chart version or lastAppliedRevision increased
// - HelmRollback: version/revision decreased (rollback)
// - ValuesChanged: spec.values or spec.valuesFrom changed
func (d *ChangeAnomalyDetector) detectHelmReleaseChanges(input DetectorInput, event analysis.ChangeEventInfo) []Anomaly {
	var anomalies []Anomaly
	var hasValuesChange bool
	var hasVersionChange bool
	var oldVersion, newVersion string

	for _, diff := range event.Diff {
		if strings.Contains(diff.Path, "spec.values") || strings.Contains(diff.Path, "spec.valuesFrom") {
			hasValuesChange = true
		}

		if strings.Contains(diff.Path, "spec.chart.spec.version") {
			hasVersionChange = true
			if s, ok := diff.OldValue.(string); ok {
				oldVersion = s
			}
			if s, ok := diff.NewValue.(string); ok {
				newVersion = s
			}
		}

		if strings.Contains(diff.Path, "status.lastAppliedRevision") || strings.Contains(diff.Path, "status.lastAttemptedRevision") {
			if !hasVersionChange {
				hasVersionChange = true
				if s, ok := diff.OldValue.(string); ok {
					oldVersion = s
				}
				if s, ok := diff.NewValue.(string); ok {
					newVersion = s
				}
			}
		}
	}

	if hasValuesChange {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryChange,
			Type:      "ValuesChanged",
			Severity:  GetSeverity(CategoryChange, "ValuesChanged", kindHelmRelease),
			Timestamp: event.Timestamp,
			Summary:   "HelmRelease values configuration changed",
			Details: map[string]interface{}{
				"event_type": event.EventType,
			},
		})
	}

	if hasVersionChange && oldVersion != "" && newVersion != "" {
		if compareVersions(oldVersion, newVersion) > 0 {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "HelmRollback",
				Severity:  GetSeverity(CategoryChange, "HelmRollback", kindHelmRelease),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("HelmRelease rolled back from %s to %s", oldVersion, newVersion),
				Details: map[string]interface{}{
					"unified_diff": analysis.FormatValueDiff("version", oldVersion, newVersion),
				},
			})
		} else {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "HelmUpgrade",
				Severity:  GetSeverity(CategoryChange, "HelmUpgrade", kindHelmRelease),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("HelmRelease upgraded from %s to %s", oldVersion, newVersion),
				Details: map[string]interface{}{
					"unified_diff": analysis.FormatValueDiff("version", oldVersion, newVersion),
				},
			})
		}
	}

	return anomalies
}
