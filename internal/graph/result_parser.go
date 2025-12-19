package graph

import (
	"encoding/json"
	"fmt"

	"github.com/FalkorDB/falkordb-go/v2"
)

// ParseNodeFromResult extracts node properties from a FalkorDB query result value
// With the FalkorDB Go client, nodes are returned as falkordb.Node objects
func ParseNodeFromResult(nodeValue interface{}) (map[string]interface{}, error) {
	// Handle nil values (from OPTIONAL MATCH)
	if nodeValue == nil {
		return make(map[string]interface{}), nil
	}

	// Try to parse as FalkorDB Node
	if node, ok := nodeValue.(falkordb.Node); ok {
		// FalkorDB Node has a Properties field that contains the properties map
		return node.Properties, nil
	}

	// Also handle pointer to Node
	if node, ok := nodeValue.(*falkordb.Node); ok {
		return node.Properties, nil
	}

	// Fallback: if it's already a map, return it
	if propsMap, ok := nodeValue.(map[string]interface{}); ok {
		return propsMap, nil
	}

	return nil, fmt.Errorf("unexpected node type: %T", nodeValue)
}

// ParseEdgeFromResult extracts edge properties from a FalkorDB query result value
// With the FalkorDB Go client, edges are returned as falkordb.Edge objects
func ParseEdgeFromResult(edgeValue interface{}) (edgeType string, properties map[string]interface{}, err error) {
	// Try to parse as FalkorDB Edge
	if edge, ok := edgeValue.(falkordb.Edge); ok {
		// FalkorDB Edge has Relation and Properties fields
		return edge.Relation, edge.Properties, nil
	}

	// Also handle pointer to Edge
	if edge, ok := edgeValue.(*falkordb.Edge); ok {
		return edge.Relation, edge.Properties, nil
	}

	return "", nil, fmt.Errorf("unexpected edge type: %T", edgeValue)
}

// ParseResourceIdentityFromNode extracts a ResourceIdentity from node properties
func ParseResourceIdentityFromNode(props map[string]interface{}) ResourceIdentity {
	resource := ResourceIdentity{}

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
	if firstSeen, ok := props["firstSeen"].(int64); ok {
		resource.FirstSeen = firstSeen
	} else if firstSeen, ok := props["firstSeen"].(float64); ok {
		resource.FirstSeen = int64(firstSeen)
	}
	if lastSeen, ok := props["lastSeen"].(int64); ok {
		resource.LastSeen = lastSeen
	} else if lastSeen, ok := props["lastSeen"].(float64); ok {
		resource.LastSeen = int64(lastSeen)
	}
	if deleted, ok := props["deleted"].(bool); ok {
		resource.Deleted = deleted
	}
	if deletedAt, ok := props["deletedAt"].(int64); ok {
		resource.DeletedAt = deletedAt
	} else if deletedAt, ok := props["deletedAt"].(float64); ok {
		resource.DeletedAt = int64(deletedAt)
	}

	// Parse labels (stored as JSON string in the database)
	if labelsJSON, ok := props["labels"].(string); ok && labelsJSON != "" && labelsJSON != "{}" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err == nil {
			resource.Labels = labels
		}
	}

	return resource
}

// ParseChangeEventFromNode extracts a ChangeEvent from node properties
func ParseChangeEventFromNode(props map[string]interface{}) ChangeEvent {
	event := ChangeEvent{}

	if id, ok := props["id"].(string); ok {
		event.ID = id
	}
	if timestamp, ok := props["timestamp"].(int64); ok {
		event.Timestamp = timestamp
	} else if timestamp, ok := props["timestamp"].(float64); ok {
		event.Timestamp = int64(timestamp)
	}
	if eventType, ok := props["eventType"].(string); ok {
		event.EventType = eventType
	}
	if status, ok := props["status"].(string); ok {
		event.Status = status
	}
	if errorMessage, ok := props["errorMessage"].(string); ok {
		event.ErrorMessage = errorMessage
	}
	if impactScore, ok := props["impactScore"].(float64); ok {
		event.ImpactScore = impactScore
	}
	if configChanged, ok := props["configChanged"].(bool); ok {
		event.ConfigChanged = configChanged
	}
	if statusChanged, ok := props["statusChanged"].(bool); ok {
		event.StatusChanged = statusChanged
	}
	if replicasChanged, ok := props["replicasChanged"].(bool); ok {
		event.ReplicasChanged = replicasChanged
	}

	// Parse container issues array
	if issues, ok := props["containerIssues"].([]interface{}); ok {
		event.ContainerIssues = make([]string, 0, len(issues))
		for _, issue := range issues {
			if issueStr, ok := issue.(string); ok {
				event.ContainerIssues = append(event.ContainerIssues, issueStr)
			}
		}
	}

	// Parse data field (full resource JSON)
	if data, ok := props["data"].(string); ok {
		event.Data = data
	}

	return event
}

// ParseTriggeredByEdge extracts TRIGGERED_BY edge properties
func ParseTriggeredByEdge(props map[string]interface{}) TriggeredByEdge {
	edge := TriggeredByEdge{}

	if confidence, ok := props["confidence"].(float64); ok {
		edge.Confidence = confidence
	}
	if lagMs, ok := props["lagMs"].(int64); ok {
		edge.LagMs = lagMs
	} else if lagMs, ok := props["lagMs"].(float64); ok {
		edge.LagMs = int64(lagMs)
	}
	if reason, ok := props["reason"].(string); ok {
		edge.Reason = reason
	}

	return edge
}

// ParseManagesEdge extracts MANAGES edge properties
func ParseManagesEdge(props map[string]interface{}) ManagesEdge {
	edge := ManagesEdge{}

	// Handle confidence as float64 or int64
	if confidence, ok := props["confidence"].(float64); ok {
		edge.Confidence = confidence
	} else if confidence, ok := props["confidence"].(int64); ok {
		edge.Confidence = float64(confidence)
	} else if confidence, ok := props["confidence"].(int); ok {
		edge.Confidence = float64(confidence)
	}
	
	if firstObserved, ok := props["firstObserved"].(int64); ok {
		edge.FirstObserved = firstObserved
	} else if firstObserved, ok := props["firstObserved"].(float64); ok {
		edge.FirstObserved = int64(firstObserved)
	}
	if lastValidated, ok := props["lastValidated"].(int64); ok {
		edge.LastValidated = lastValidated
	} else if lastValidated, ok := props["lastValidated"].(float64); ok {
		edge.LastValidated = int64(lastValidated)
	}
	if validationState, ok := props["validationState"].(string); ok {
		edge.ValidationState = ValidationState(validationState)
	}

	// Parse evidence array
	if evidenceRaw, ok := props["evidence"].(string); ok {
		var evidence []EvidenceItem
		if err := json.Unmarshal([]byte(evidenceRaw), &evidence); err == nil {
			edge.Evidence = evidence
		}
	}

	return edge
}

// ParseK8sEventFromNode extracts K8sEvent from node properties
func ParseK8sEventFromNode(props map[string]interface{}) K8sEvent {
	event := K8sEvent{}

	if id, ok := props["id"].(string); ok {
		event.ID = id
	}
	if timestamp, ok := props["timestamp"].(int64); ok {
		event.Timestamp = timestamp
	} else if timestamp, ok := props["timestamp"].(float64); ok {
		event.Timestamp = int64(timestamp)
	}
	if reason, ok := props["reason"].(string); ok {
		event.Reason = reason
	}
	if message, ok := props["message"].(string); ok {
		event.Message = message
	}
	if eventType, ok := props["type"].(string); ok {
		event.Type = eventType
	}
	if count, ok := props["count"].(int64); ok {
		event.Count = int(count)
	} else if count, ok := props["count"].(float64); ok {
		event.Count = int(count)
	}
	if source, ok := props["source"].(string); ok {
		event.Source = source
	}

	return event
}

// GetStringProperty safely extracts a string property
func GetStringProperty(props map[string]interface{}, key string) string {
	if val, ok := props[key].(string); ok {
		return val
	}
	return ""
}

// GetInt64Property safely extracts an int64 property
func GetInt64Property(props map[string]interface{}, key string) int64 {
	if val, ok := props[key].(int64); ok {
		return val
	}
	if val, ok := props[key].(float64); ok {
		return int64(val)
	}
	if val, ok := props[key].(int); ok {
		return int64(val)
	}
	return 0
}

// GetFloat64Property safely extracts a float64 property
func GetFloat64Property(props map[string]interface{}, key string) float64 {
	if val, ok := props[key].(float64); ok {
		return val
	}
	if val, ok := props[key].(int64); ok {
		return float64(val)
	}
	if val, ok := props[key].(int); ok {
		return float64(val)
	}
	return 0.0
}

// DebugPrintResult prints the structure of a query result for debugging
func DebugPrintResult(result *QueryResult) {
	fmt.Printf("Columns: %v\n", result.Columns)
	fmt.Printf("Rows: %d\n", len(result.Rows))
	for i, row := range result.Rows {
		fmt.Printf("Row %d:\n", i)
		for j, val := range row {
			valJSON, _ := json.MarshalIndent(val, "  ", "  ")
			fmt.Printf("  [%d] (%T): %s\n", j, val, string(valJSON))
		}
	}
}
