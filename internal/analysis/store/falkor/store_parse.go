package falkor

import "github.com/moolen/spectre/internal/graph"

func parseResourceResults(result *graph.QueryResult) []resourceResult {
	resources := make([]resourceResult, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 10 {
			continue
		}

		resource := resourceResult{}
		if uid, ok := row[0].(string); ok {
			resource.UID = uid
		}
		if kind, ok := row[1].(string); ok {
			resource.Kind = kind
		}
		if apiGroup, ok := row[2].(string); ok {
			resource.APIGroup = apiGroup
		}
		if namespace, ok := row[3].(string); ok {
			resource.Namespace = namespace
		}
		if name, ok := row[4].(string); ok {
			resource.Name = name
		}
		if labels, ok := row[5].(map[string]interface{}); ok {
			resource.Labels = make(map[string]string)
			for key, value := range labels {
				if labelValue, ok := value.(string); ok {
					resource.Labels[key] = labelValue
				}
			}
		} else if labels, ok := row[5].(map[string]string); ok {
			resource.Labels = labels
		}
		switch firstSeen := row[6].(type) {
		case int64:
			resource.FirstSeen = firstSeen
		case float64:
			resource.FirstSeen = int64(firstSeen)
		}
		switch lastSeen := row[7].(type) {
		case int64:
			resource.LastSeen = lastSeen
		case float64:
			resource.LastSeen = int64(lastSeen)
		}
		if deleted, ok := row[8].(bool); ok {
			resource.Deleted = deleted
		}
		switch deletedAt := row[9].(type) {
		case int64:
			resource.DeletedAt = deletedAt
		case float64:
			resource.DeletedAt = int64(deletedAt)
		}

		if resource.UID != "" {
			resources = append(resources, resource)
		}
	}
	return resources
}
