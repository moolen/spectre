package extractors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// graphClientLookup implements ResourceLookup using graph.Client
type graphClientLookup struct {
	client graph.Client
}

// NewGraphClientLookup creates a ResourceLookup adapter for graph.Client
func NewGraphClientLookup(client graph.Client) ResourceLookup {
	return &graphClientLookup{client: client}
}

func (l *graphClientLookup) FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	query := graph.FindResourceByUIDQuery(uid)
	result, err := l.client.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("resource not found: %s", uid)
	}

	// Parse node from result
	// FalkorDB returns nodes as arrays: [0] = properties map
	if len(result.Rows[0]) == 0 {
		return nil, fmt.Errorf("empty result row")
	}

	// Pass the node directly - parseResourceIdentity handles FalkorDB nodes
	return parseResourceIdentity(result.Rows[0][0])
}

func (l *graphClientLookup) FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE r.namespace = $namespace
			  AND r.kind = $kind
			  AND r.name = $name
			  AND r.deleted = false
			RETURN r
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"namespace": namespace,
			"kind":      kind,
			"name":      name,
		},
	}

	result, err := l.client.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("resource not found: %s/%s/%s", namespace, kind, name)
	}

	if len(result.Rows[0]) == 0 {
		return nil, fmt.Errorf("empty result row")
	}

	// Pass the node directly - parseResourceIdentity handles FalkorDB nodes
	return parseResourceIdentity(result.Rows[0][0])
}

func (l *graphClientLookup) FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error) {
	// Calculate time window
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $uid})-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp >= $startTime
			RETURN e
			ORDER BY e.timestamp DESC
			LIMIT 100
		`,
		Parameters: map[string]interface{}{
			"uid":       uid,
			"startTime": windowNs,
		},
	}

	result, err := l.client.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	events := make([]graph.ChangeEvent, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) == 0 {
			continue
		}

		eventData, ok := row[0].(map[string]interface{})
		if !ok {
			continue
		}

		event, err := parseChangeEvent(eventData)
		if err != nil {
			continue // Skip malformed events
		}

		events = append(events, *event)
	}

	return events, nil
}

func (l *graphClientLookup) QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	return l.client.ExecuteQuery(ctx, query)
}

// Helper functions to parse graph results

func parseResourceIdentity(data interface{}) (*graph.ResourceIdentity, error) {
	// Handle FalkorDB Node objects
	props, err := graph.ParseNodeFromResult(data)
	if err != nil {
		// If it's not a Node, try to use it as a map directly
		if dataMap, ok := data.(map[string]interface{}); ok {
			props = dataMap
		} else {
			return nil, fmt.Errorf("failed to parse resource identity: %w", err)
		}
	}

	resource := &graph.ResourceIdentity{}

	if uid, ok := props["uid"].(string); ok {
		resource.UID = uid
	}
	if kind, ok := props["kind"].(string); ok {
		resource.Kind = kind
	}
	if apiGroup, ok := props["apiGroup"].(string); ok {
		resource.APIGroup = apiGroup
	}
	if version, ok := props["version"].(string); ok {
		resource.Version = version
	}
	if namespace, ok := props["namespace"].(string); ok {
		resource.Namespace = namespace
	}
	if name, ok := props["name"].(string); ok {
		resource.Name = name
	}

	// Handle numeric fields (may be float64 or int64)
	if firstSeen, ok := props["firstSeen"].(float64); ok {
		resource.FirstSeen = int64(firstSeen)
	} else if firstSeen, ok := props["firstSeen"].(int64); ok {
		resource.FirstSeen = firstSeen
	}

	if lastSeen, ok := props["lastSeen"].(float64); ok {
		resource.LastSeen = int64(lastSeen)
	} else if lastSeen, ok := props["lastSeen"].(int64); ok {
		resource.LastSeen = lastSeen
	}

	if deleted, ok := props["deleted"].(bool); ok {
		resource.Deleted = deleted
	}

	if deletedAt, ok := props["deletedAt"].(float64); ok {
		resource.DeletedAt = int64(deletedAt)
	} else if deletedAt, ok := props["deletedAt"].(int64); ok {
		resource.DeletedAt = deletedAt
	}

	// Parse labels (stored as JSON string in graph)
	if labelsJSON, ok := props["labels"].(string); ok && labelsJSON != "" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err == nil {
			resource.Labels = labels
		}
	}

	return resource, nil
}

func parseChangeEvent(data map[string]interface{}) (*graph.ChangeEvent, error) {
	event := &graph.ChangeEvent{}

	if id, ok := data["id"].(string); ok {
		event.ID = id
	}

	if timestamp, ok := data["timestamp"].(float64); ok {
		event.Timestamp = int64(timestamp)
	} else if timestamp, ok := data["timestamp"].(int64); ok {
		event.Timestamp = timestamp
	}

	if eventType, ok := data["eventType"].(string); ok {
		event.EventType = eventType
	}
	if status, ok := data["status"].(string); ok {
		event.Status = status
	}
	if errorMessage, ok := data["errorMessage"].(string); ok {
		event.ErrorMessage = errorMessage
	}

	// Handle array fields
	if containerIssuesRaw, ok := data["containerIssues"].([]interface{}); ok {
		containerIssues := make([]string, 0, len(containerIssuesRaw))
		for _, issue := range containerIssuesRaw {
			if issueStr, ok := issue.(string); ok {
				containerIssues = append(containerIssues, issueStr)
			}
		}
		event.ContainerIssues = containerIssues
	}

	// Handle boolean fields
	if configChanged, ok := data["configChanged"].(bool); ok {
		event.ConfigChanged = configChanged
	}
	if statusChanged, ok := data["statusChanged"].(bool); ok {
		event.StatusChanged = statusChanged
	}
	if replicasChanged, ok := data["replicasChanged"].(bool); ok {
		event.ReplicasChanged = replicasChanged
	}

	if impactScore, ok := data["impactScore"].(float64); ok {
		event.ImpactScore = impactScore
	}

	return event, nil
}

// ExtractUID extracts UID from a query result row
func ExtractUID(row []interface{}) string {
	if len(row) == 0 {
		return ""
	}

	// Check if it's a direct UID string
	if uid, ok := row[0].(string); ok {
		return uid
	}

	// Use the result parser to handle FalkorDB Node objects
	nodeProps, err := graph.ParseNodeFromResult(row[0])
	if err == nil && nodeProps != nil {
		if uid, ok := nodeProps["uid"].(string); ok {
			return uid
		}
	}

	// Fallback: Check if it's a plain map
	if nodeData, ok := row[0].(map[string]interface{}); ok {
		if uid, ok := nodeData["uid"].(string); ok {
			return uid
		}
	}

	return ""
}

// ParseManagesEdge parses a MANAGES edge from query result
func ParseManagesEdge(edgeData map[string]interface{}) (*graph.ManagesEdge, error) {
	edge := &graph.ManagesEdge{}

	if confidence, ok := edgeData["confidence"].(float64); ok {
		edge.Confidence = confidence
	}

	if evidenceJSON, ok := edgeData["evidence"].(string); ok {
		var evidence []graph.EvidenceItem
		if err := json.Unmarshal([]byte(evidenceJSON), &evidence); err == nil {
			edge.Evidence = evidence
		}
	}

	if firstObserved, ok := edgeData["firstObserved"].(float64); ok {
		edge.FirstObserved = int64(firstObserved)
	} else if firstObserved, ok := edgeData["firstObserved"].(int64); ok {
		edge.FirstObserved = firstObserved
	}

	if lastValidated, ok := edgeData["lastValidated"].(float64); ok {
		edge.LastValidated = int64(lastValidated)
	} else if lastValidated, ok := edgeData["lastValidated"].(int64); ok {
		edge.LastValidated = lastValidated
	}

	if validationState, ok := edgeData["validationState"].(string); ok {
		edge.ValidationState = graph.ValidationState(validationState)
	}

	return edge, nil
}
