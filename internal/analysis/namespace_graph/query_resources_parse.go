package namespacegraph

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// parseResourceResults parses the query result into resourceResult structs
func parseResourceResults(result *graph.QueryResult) []resourceResult {
	resources := make([]resourceResult, 0, len(result.Rows))

	for _, row := range result.Rows {
		resource := resourceResult{
			UID:       parseStringCell(row, 0),
			Kind:      parseStringCell(row, 1),
			APIGroup:  parseStringCell(row, 2),
			Namespace: parseStringCell(row, 3),
			Name:      parseStringCell(row, 4),
			Labels:    parseLabelsCell(row, 5),
			FirstSeen: parseInt64Cell(row, 6),
			LastSeen:  parseInt64Cell(row, 7),
			Deleted:   parseBoolCell(row, 8),
			DeletedAt: parseInt64Cell(row, 9),
		}

		if resource.UID != "" {
			resources = append(resources, resource)
		}
	}

	return resources
}

func parseStringCell(row []interface{}, index int) string {
	if index >= len(row) {
		return ""
	}
	value, _ := row[index].(string)
	return value
}

func parseInt64Cell(row []interface{}, index int) int64 {
	if index >= len(row) {
		return 0
	}

	switch value := row[index].(type) {
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func parseFloat64Cell(row []interface{}, index int) float64 {
	if index >= len(row) {
		return 0
	}
	value, _ := row[index].(float64)
	return value
}

func parseBoolCell(row []interface{}, index int) bool {
	if index >= len(row) {
		return false
	}
	value, _ := row[index].(bool)
	return value
}

func parseLabelsCell(row []interface{}, index int) map[string]string {
	if index >= len(row) {
		return nil
	}

	labels, ok := row[index].(map[string]interface{})
	if !ok {
		return nil
	}

	parsed := make(map[string]string)
	for key, value := range labels {
		if stringValue, ok := value.(string); ok {
			parsed[key] = stringValue
		}
	}

	return parsed
}

// decodeCursor decodes a base64-encoded pagination cursor
func decodeCursor(cursor string) (*PaginationCursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var pc PaginationCursor
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return &pc, nil
}

// encodeCursor encodes a pagination cursor to base64
func encodeCursor(pc PaginationCursor) string {
	data, _ := json.Marshal(pc)
	return base64.StdEncoding.EncodeToString(data)
}
