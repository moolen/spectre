package anomaly

import (
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

func (d *ChangeAnomalyDetector) detectSpecificChanges(input DetectorInput, event analysis.ChangeEventInfo) []Anomaly {
	var anomalies []Anomaly
	kind := input.Node.Resource.Kind

	if kind == "Node" {
		anomalies = append(anomalies, d.detectNodeTaintChanges(input, event)...)
	}

	for _, diff := range event.Diff {
		if isImageChangeDiff(diff) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ImageChanged",
				Severity:  GetSeverity(CategoryChange, "ImageChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container image changed",
				Details: map[string]interface{}{
					"unified_diff": analysis.FormatValueDiff(diff.Path, diff.OldValue, diff.NewValue),
				},
			})
		}

		if strings.Contains(diff.Path, "spec.containers") && strings.Contains(diff.Path, ".env") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "EnvironmentChanged",
				Severity:  GetSeverity(CategoryChange, "EnvironmentChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container environment variables changed",
				Details: map[string]interface{}{
					"path":      diff.Path,
					"operation": diff.Op,
				},
			})
		}

		if strings.Contains(diff.Path, "spec.containers") &&
			(strings.Contains(diff.Path, ".resources.limits") || strings.Contains(diff.Path, ".resources.requests")) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ResourceLimitsChanged",
				Severity:  GetSeverity(CategoryChange, "ResourceLimitsChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container resource limits/requests changed",
				Details: map[string]interface{}{
					"unified_diff": analysis.FormatValueDiff(diff.Path, diff.OldValue, diff.NewValue),
				},
			})
		}
	}

	return anomalies
}

func (d *ChangeAnomalyDetector) detectNodeTaintChanges(input DetectorInput, event analysis.ChangeEventInfo) []Anomaly {
	var anomalies []Anomaly

	for _, diff := range event.Diff {
		if strings.Contains(diff.Path, "spec.taints") && (diff.Op == "add" || diff.Op == "replace") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "TaintAdded",
				Severity:  SeverityMedium,
				Timestamp: event.Timestamp,
				Summary:   "Node taint added",
				Details: map[string]interface{}{
					"unified_diff": analysis.FormatValueDiff(diff.Path, diff.OldValue, diff.NewValue),
				},
			})
		}
	}

	if len(event.Diff) > 0 || !event.ConfigChanged {
		return anomalies
	}

	resourceData := eventSnapshotData(event)
	if resourceData == nil {
		return anomalies
	}

	spec, ok := resourceData["spec"].(map[string]interface{})
	if !ok {
		return anomalies
	}

	taints, ok := spec["taints"].([]interface{})
	if !ok || len(taints) == 0 {
		return anomalies
	}

	anomalies = append(anomalies, Anomaly{
		Node:      NodeFromGraphNode(input.Node),
		Category:  CategoryChange,
		Type:      "TaintAdded",
		Severity:  SeverityMedium,
		Timestamp: event.Timestamp,
		Summary:   "Node taint added",
		Details: map[string]interface{}{
			"taint_count": len(taints),
		},
	})

	return anomalies
}

func isImageChangeDiff(diff analysis.EventDiff) bool {
	isImageFieldChange := strings.Contains(diff.Path, "spec.containers") && strings.HasSuffix(diff.Path, ".image")
	isContainersArrayChange := (diff.Path == "spec.containers" || diff.Path == "spec.template.spec.containers") && diff.Op == "replace"
	return isImageFieldChange || isContainersArrayChange
}
