package mcp

import "github.com/moolen/spectre/internal/mcp/tools"

func (s *SpectreServer) registerTools() {
	s.registerTool(
		"cluster_health",
		"Get cluster health overview with resource status breakdown and top issues",
		tools.NewClusterHealthTool(s.timelineService),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by Kubernetes namespace",
				},
				"max_resources": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to list per status (default 100, max 500)",
				},
			},
			"required": []string{"start_time", "end_time"},
		},
	)

	s.registerTool(
		"resource_timeline_changes",
		"Get semantic field-level changes for resources by UID with noise filtering and status condition summarization",
		tools.NewResourceTimelineChangesTool(s.timelineService),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_uids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "List of resource UIDs to query (required, max 10)",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: end timestamp (Unix seconds or milliseconds). Default: now",
				},
				"include_full_snapshot": map[string]interface{}{
					"type":        "boolean",
					"description": "Optional: include first segment's full resource JSON (default: false)",
				},
				"max_changes_per_resource": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max changes per resource (default 50, max 200)",
				},
			},
			"required": []string{"resource_uids"},
		},
	)

	s.registerTool(
		"resource_timeline",
		"Get resource timeline with status segments, events, and transitions for root cause analysis",
		tools.NewResourceTimelineTool(s.timelineService),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_kind": map[string]interface{}{
					"type":        "string",
					"description": "Resource kind to get timeline for (e.g., 'Pod', 'Deployment')",
				},
				"resource_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional: specific resource name, or '*' for all",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Kubernetes namespace to filter by",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to return when using '*' (default 20, max 100)",
				},
			},
			"required": []string{"resource_kind", "start_time", "end_time"},
		},
	)
}
