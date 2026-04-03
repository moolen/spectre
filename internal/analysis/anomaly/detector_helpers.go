package anomaly

import (
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/analysis"
)

// IsPodFailureAnomaly checks if an anomaly type indicates pod failure
func IsPodFailureAnomaly(anomalyType string) bool {
	failureTypes := map[string]bool{
		"CrashLoopBackOff":           true,
		"ImagePullBackOff":           true,
		"ErrImagePull":               true,
		"OOMKilled":                  true,
		"ContainerCreateError":       true,
		"CreateContainerConfigError": true,
		"InvalidImageNameError":      true,
		"PodPending":                 true,
		"Evicted":                    true,
		"ErrorStatus":                true,
		"InitContainerFailed":        true,
	}
	return failureTypes[anomalyType]
}

// deduplicateAnomalies removes duplicate anomalies based on node+type+timestamp
func deduplicateAnomalies(anomalies []Anomaly) []Anomaly {
	seen := make(map[string]bool)
	result := make([]Anomaly, 0, len(anomalies))

	for _, anomaly := range anomalies {
		key := fmt.Sprintf("%s:%s:%s:%d",
			anomaly.Node.UID, anomaly.Category, anomaly.Type, anomaly.Timestamp.Unix())

		if seen[key] {
			continue
		}

		seen[key] = true
		result = append(result, anomaly)
	}

	return result
}

func latestSnapshotData(node *analysis.GraphNode) map[string]interface{} {
	if len(node.AllEvents) == 0 {
		return nil
	}

	latestEvent := node.AllEvents[len(node.AllEvents)-1]
	if latestEvent.FullSnapshot != nil {
		return latestEvent.FullSnapshot
	}

	if len(latestEvent.Data) == 0 {
		return nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(latestEvent.Data, &resourceData); err != nil {
		return nil
	}

	return resourceData
}
