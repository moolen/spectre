package graph

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildPropertiesString converts a map to Cypher property syntax.
func buildPropertiesString(props map[string]interface{}) string {
	if len(props) == 0 {
		return ""
	}

	parts := make([]string, 0, len(props))
	for key, value := range props {
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = fmt.Sprintf("'%s'", escapeCypherString(v))
		case bool:
			valueStr = fmt.Sprintf("%t", v)
		case int, int64, float64:
			valueStr = fmt.Sprintf("%v", v)
		case []string:
			escaped := make([]string, len(v))
			for i, s := range v {
				escaped[i] = fmt.Sprintf("'%s'", escapeCypherString(s))
			}
			valueStr = fmt.Sprintf("[%s]", strings.Join(escaped, ", "))
		default:
			jsonBytes, _ := json.Marshal(v)
			valueStr = fmt.Sprintf("'%s'", escapeCypherString(string(jsonBytes)))
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, valueStr))
	}

	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

// escapeCypherString escapes a string for safe inclusion in a Cypher query.
func escapeCypherString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

// replaceCypherParameters replaces $param placeholders with actual values.
func replaceCypherParameters(query string, params map[string]interface{}) string {
	result := query
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}

	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) || (len(keys[j]) == len(keys[i]) && keys[j] < keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, key := range keys {
		placeholder := "$" + key
		var replacement string

		switch v := params[key].(type) {
		case string:
			replacement = fmt.Sprintf("'%s'", escapeCypherString(v))
		case bool:
			replacement = fmt.Sprintf("%t", v)
		case int:
			replacement = fmt.Sprintf("%d", v)
		case int64:
			replacement = fmt.Sprintf("%d", v)
		case float64:
			replacement = fmt.Sprintf("%f", v)
		case []string:
			escaped := make([]string, len(v))
			for i, s := range v {
				escaped[i] = fmt.Sprintf("'%s'", escapeCypherString(s))
			}
			replacement = fmt.Sprintf("[%s]", strings.Join(escaped, ", "))
		default:
			jsonBytes, _ := json.Marshal(v)
			replacement = fmt.Sprintf("'%s'", escapeCypherString(string(jsonBytes)))
		}

		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result
}

// parseGraphQueryResult parses the result from a raw GRAPH.QUERY command.
func parseGraphQueryResult(result interface{}) (*QueryResult, error) {
	resultArray, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	queryResult := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
		Stats:   QueryStats{},
	}
	if len(resultArray) == 0 {
		return queryResult, nil
	}

	if columns, ok := resultArray[0].([]interface{}); ok {
		queryResult.Columns = make([]string, len(columns))
		for i, col := range columns {
			if colStr, ok := col.(string); ok {
				queryResult.Columns[i] = colStr
			}
		}
	}

	if len(resultArray) > 2 {
		for i := 1; i < len(resultArray)-1; i++ {
			if row, ok := resultArray[i].([]interface{}); ok && len(row) > 0 {
				queryResult.Rows = append(queryResult.Rows, row)
			}
		}
	}

	if statsArray, ok := resultArray[len(resultArray)-1].([]interface{}); ok {
		queryResult.Stats = parseQueryStats(statsArray)
	}

	return queryResult, nil
}

// parseQueryStats extracts statistics from the FalkorDB stats array.
func parseQueryStats(statsArray []interface{}) QueryStats {
	stats := QueryStats{}

	for _, stat := range statsArray {
		statStr, ok := stat.(string)
		if !ok {
			continue
		}

		if strings.Contains(statStr, "Nodes created:") {
			_, _ = fmt.Sscanf(statStr, "Nodes created: %d", &stats.NodesCreated)
		} else if strings.Contains(statStr, "Nodes deleted:") {
			_, _ = fmt.Sscanf(statStr, "Nodes deleted: %d", &stats.NodesDeleted)
		} else if strings.Contains(statStr, "Relationships created:") {
			_, _ = fmt.Sscanf(statStr, "Relationships created: %d", &stats.RelationshipsCreated)
		} else if strings.Contains(statStr, "Relationships deleted:") {
			_, _ = fmt.Sscanf(statStr, "Relationships deleted: %d", &stats.RelationshipsDeleted)
		} else if strings.Contains(statStr, "Properties set:") {
			_, _ = fmt.Sscanf(statStr, "Properties set: %d", &stats.PropertiesSet)
		} else if strings.Contains(statStr, "Labels added:") {
			_, _ = fmt.Sscanf(statStr, "Labels added: %d", &stats.LabelsAdded)
		}
	}

	return stats
}
