package extractors

import (
	"encoding/json"
	"strings"

	"github.com/moolen/spectre/internal/graph"
)

// ExtractUIDFromRow extracts UID from a graph query result row (map-based results)
// Handles both ResourceIdentity nodes and direct UID fields
func ExtractUIDFromRow(row map[string]interface{}) string {
	// Try p.uid first (common pattern)
	if uid, ok := row["p.uid"].(string); ok {
		return uid
	}

	// Try r.uid
	if uid, ok := row["r.uid"].(string); ok {
		return uid
	}

	// Try uid field directly
	if uid, ok := row["uid"].(string); ok {
		return uid
	}

	return ""
}

// LabelsMatchSelector checks if resource labels match a selector
// Returns true if ALL selector labels are present with matching values
func LabelsMatchSelector(resourceLabels map[string]string, selectorLabels map[string]string) bool {
	if len(selectorLabels) == 0 {
		return true // Empty selector matches everything
	}

	for key, value := range selectorLabels {
		resourceValue, exists := resourceLabels[key]
		if !exists || resourceValue != value {
			return false
		}
	}

	return true
}

// HasLabelMatch checks if a resource has a specific label key-value pair
func HasLabelMatch(labels map[string]string, key, value string) bool {
	if labels == nil {
		return false
	}
	labelValue, exists := labels[key]
	return exists && labelValue == value
}

// ParseJSONField safely parses a JSON field from event data
func ParseJSONField(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetNestedField safely retrieves a nested field from a map
// Example: GetNestedField(obj, "spec", "template", "spec")
func GetNestedField(obj map[string]interface{}, fields ...string) (interface{}, bool) {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			// Last field, return its value
			val, ok := current[field]
			return val, ok
		}

		// Intermediate field, must be a map
		next, ok := current[field].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

// GetNestedString safely retrieves a nested string field
func GetNestedString(obj map[string]interface{}, fields ...string) (string, bool) {
	val, ok := GetNestedField(obj, fields...)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetNestedMap safely retrieves a nested map field
func GetNestedMap(obj map[string]interface{}, fields ...string) (map[string]interface{}, bool) {
	val, ok := GetNestedField(obj, fields...)
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]interface{})
	return m, ok
}

// GetNestedArray safely retrieves a nested array field
func GetNestedArray(obj map[string]interface{}, fields ...string) ([]interface{}, bool) {
	val, ok := GetNestedField(obj, fields...)
	if !ok {
		return nil, false
	}
	arr, ok := val.([]interface{})
	return arr, ok
}

// ParseLabelsFromMap converts interface{} labels to map[string]string
func ParseLabelsFromMap(labelsInterface interface{}) map[string]string {
	if labelsInterface == nil {
		return nil
	}

	labelsMap, ok := labelsInterface.(map[string]interface{})
	if !ok {
		return nil
	}

	result := make(map[string]string, len(labelsMap))
	for k, v := range labelsMap {
		if strVal, ok := v.(string); ok {
			result[k] = strVal
		}
	}
	return result
}

// BuildLabelQuery creates a Cypher query fragment for label matching
// Returns a WHERE clause fragment
// nodeAlias: the node variable name to use (e.g., "p", "r")
func BuildLabelQuery(labels map[string]string, nodeAlias string) string {
	if len(labels) == 0 {
		return ""
	}

	if nodeAlias == "" {
		nodeAlias = "r"
	}

	var conditions []string
	for key, value := range labels {
		// JSON substring matching for labels stored as JSON
		conditions = append(conditions, nodeAlias+`.labels CONTAINS '"`+key+`":"`+value+`"'`)
	}

	return strings.Join(conditions, " AND ")
}

// CalculateTemporalProximityScore calculates a score based on time difference
// Returns 1.0 for immediate proximity, 0.0 if outside window
func CalculateTemporalProximityScore(lagMs int64, maxWindowMs int64) float64 {
	if lagMs < 0 || lagMs > maxWindowMs {
		return 0.0
	}
	return 1.0 - (float64(lagMs) / float64(maxWindowMs))
}

// CreateEvidenceItem creates an evidence item for relationship scoring
func CreateEvidenceItem(evidenceType graph.EvidenceType, value string, weight float64) graph.EvidenceItem {
	return graph.EvidenceItem{
		Type:      evidenceType,
		Value:     value,
		Weight:    weight,
		Timestamp: 0, // Will be set by caller if needed
	}
}
