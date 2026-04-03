package graph

import (
	"encoding/json"
	"strconv"
	"strings"
)

// buildCypherMapLiteral builds a Cypher map literal from a Go map.
func buildCypherMapLiteral(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(m))
	for key, value := range m {
		var valueString string
		switch typed := value.(type) {
		case string:
			valueString = "'" + escapeCypherString(typed) + "'"
		case bool:
			valueString = strconv.FormatBool(typed)
		case int:
			valueString = strconv.Itoa(typed)
		case int64:
			valueString = strconv.FormatInt(typed, 10)
		case float64:
			valueString = strconv.FormatFloat(typed, 'f', -1, 64)
		case nil:
			valueString = "null"
		default:
			jsonBytes, _ := json.Marshal(typed)
			valueString = "'" + escapeCypherString(string(jsonBytes)) + "'"
		}
		parts = append(parts, key+": "+valueString)
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

// buildCypherListLiteral builds a Cypher list literal from a slice of maps.
func buildCypherListLiteral(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "[]"
	}

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = buildCypherMapLiteral(item)
	}

	return "[" + strings.Join(parts, ", ") + "]"
}
