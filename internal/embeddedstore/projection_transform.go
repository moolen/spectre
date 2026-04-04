package embeddedstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

func buildResourceIdentity(event models.Event, object map[string]any, priorVersions []resourceVersion, previousData []byte) graph.ResourceIdentity {
	labels := extractLabels(object)
	if len(labels) == 0 && len(previousData) > 0 {
		labels = extractLabels(parseObject(previousData))
	}

	firstSeen := event.Timestamp
	if len(priorVersions) > 0 {
		firstSeen = priorVersions[0].timestamp
	}

	return graph.ResourceIdentity{
		UID:       event.Resource.UID,
		Kind:      event.Resource.Kind,
		APIGroup:  event.Resource.Group,
		Version:   event.Resource.Version,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Labels:    labels,
		FirstSeen: firstSeen,
		LastSeen:  event.Timestamp,
		Deleted:   event.Type == models.EventTypeDelete,
		DeletedAt: func() int64 {
			if event.Type == models.EventTypeDelete {
				return event.Timestamp
			}
			return 0
		}(),
	}
}

func buildChangeEventInfo(event models.Event, data, previousData []byte) analysisstore.ChangeEventInfo {
	var parsed *analyzerpkg.ResourceData
	if len(data) > 0 {
		parsed, _ = analyzerpkg.ParseResourceData(data)
	}

	status := analyzerpkg.InferStatusFromParsedData(event.Resource.Kind, parsed, string(event.Type))
	configChanged, statusChanged := detectChangeKinds(previousData, data)

	return analysisstore.ChangeEventInfo{
		EventID:       event.ID,
		Timestamp:     timeFromNs(event.Timestamp),
		EventType:     string(event.Type),
		Status:        status,
		ConfigChanged: configChanged,
		StatusChanged: statusChanged,
		Description:   fmt.Sprintf("%s event", event.Type),
		Data:          nil,
		FullSnapshot:  nil,
		Significance:  nil,
		Diff:          nil,
	}
}

func buildK8sEventInfo(event models.Event) analysisstore.K8sEventInfo {
	object := parseObject(event.Data)
	count := 1
	if value, ok := getInt(object, "count"); ok {
		count = int(value)
	}

	source := ""
	if sourceMap := getMap(object, "source"); sourceMap != nil {
		source = getString(sourceMap, "component")
		if source == "" {
			source = getString(sourceMap, "host")
		}
	}

	return analysisstore.K8sEventInfo{
		EventID:   event.ID,
		Timestamp: timeFromNs(event.Timestamp),
		Reason:    getString(object, "reason"),
		Message:   getString(object, "message"),
		Type:      getString(object, "type"),
		Count:     count,
		Source:    source,
	}
}

func detectChangeKinds(previousData, currentData []byte) (bool, bool) {
	if len(previousData) == 0 || len(currentData) == 0 {
		return false, false
	}

	previousObject := parseObject(previousData)
	currentObject := parseObject(currentData)
	kind := getString(currentObject, "kind")

	configChanged := detectKindSpecificConfigChange(kind, previousObject, currentObject)
	statusChanged := fieldChanged(previousObject, currentObject, "status")

	diffs, err := analysis.ComputeJSONDiff(previousData, currentData)
	if err != nil {
		return configChanged, statusChanged
	}
	for _, diff := range diffs {
		if bytes.HasPrefix([]byte(diff.Path), []byte("/spec")) || bytes.HasPrefix([]byte(diff.Path), []byte("spec")) {
			configChanged = true
		}
		if bytes.HasPrefix([]byte(diff.Path), []byte("/status")) || bytes.HasPrefix([]byte(diff.Path), []byte("status")) {
			statusChanged = true
		}
	}
	return configChanged, statusChanged
}

func detectKindSpecificConfigChange(kind string, previousObject, currentObject map[string]any) bool {
	switch kind {
	case "ConfigMap", "Secret":
		return fieldChanged(previousObject, currentObject, "data") ||
			fieldChanged(previousObject, currentObject, "binaryData") ||
			fieldChanged(previousObject, currentObject, "stringData")
	case "Role", "ClusterRole":
		return fieldChanged(previousObject, currentObject, "rules") ||
			fieldChanged(previousObject, currentObject, "aggregationRule")
	case "RoleBinding", "ClusterRoleBinding":
		return fieldChanged(previousObject, currentObject, "roleRef") ||
			fieldChanged(previousObject, currentObject, "subjects")
	default:
		return false
	}
}

func fieldChanged(previousObject, currentObject map[string]any, key string) bool {
	return !reflect.DeepEqual(previousObject[key], currentObject[key])
}

func extractLabels(object map[string]any) map[string]string {
	meta := getMap(object, "metadata")
	labelsMap := getMap(meta, "labels")
	if len(labelsMap) == 0 {
		return nil
	}
	labels := make(map[string]string, len(labelsMap))
	for key, value := range labelsMap {
		if stringValue, ok := value.(string); ok {
			labels[key] = stringValue
		}
	}
	return labels
}

func getMap(object map[string]any, key string) map[string]any {
	if object == nil {
		return nil
	}
	value, ok := object[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func getSlice(object map[string]any, key string) []any {
	if object == nil {
		return nil
	}
	value, ok := object[key]
	if !ok {
		return nil
	}
	typed, _ := value.([]any)
	return typed
}

func getString(object map[string]any, key string) string {
	if object == nil {
		return ""
	}
	value, ok := object[key]
	if !ok {
		return ""
	}
	typed, _ := value.(string)
	return typed
}

func getInt(object map[string]any, key string) (int64, bool) {
	if object == nil {
		return 0, false
	}
	value, ok := object[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}

func copyIdentity(version *resourceVersion) graph.ResourceIdentity {
	identity := version.identity
	identity.Labels = copyStringMap(identity.Labels)
	return identity
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func parseObject(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil
	}
	return object
}

func timeFromNs(value int64) time.Time {
	return time.Unix(0, value)
}
