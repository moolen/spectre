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

	s.registerTool(
		"detect_anomalies",
		"Detect anomalies in a resource's causal subgraph including crash loops, config errors, state transitions, and networking issues",
		tools.NewDetectAnomaliesTool(s.graphService, s.timelineService),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_uid": map[string]interface{}{
					"type":        "string",
					"description": "The UID of the resource to analyze for anomalies",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
			},
			"required": []string{"resource_uid", "start_time", "end_time"},
		},
	)

	s.registerTool(
		"causal_paths",
		"Discover causal paths from root causes to a failing resource using graph-based causality analysis. Returns ranked paths with confidence scores.",
		tools.NewCausalPathsTool(s.graphService),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resourceUID": map[string]interface{}{
					"type":        "string",
					"description": "The UID of the resource that failed (symptom)",
				},
				"failureTimestamp": map[string]interface{}{
					"type":        "integer",
					"description": "Unix timestamp (seconds or nanoseconds) when the failure occurred",
				},
				"lookbackMinutes": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: how far back to search for causes in minutes (default: 10)",
				},
				"maxDepth": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: maximum depth to traverse causality chain (default: 5, max: 10)",
				},
				"maxPaths": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: maximum number of causal paths to return (default: 5, max: 20)",
				},
			},
			"required": []string{"resourceUID", "failureTimestamp"},
		},
	)
}
