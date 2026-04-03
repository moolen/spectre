package graph

import (
	"encoding/json"
	"fmt"
)

// BatchUpsertResourceIdentitiesQuery creates a single query to upsert multiple ResourceIdentity nodes.
func BatchUpsertResourceIdentitiesQuery(resources []ResourceIdentity) GraphQuery {
	resourceParams := make([]map[string]interface{}, len(resources))
	for i, resource := range resources {
		labelsJSON := "{}"
		if len(resource.Labels) > 0 {
			labelsBytes, _ := json.Marshal(resource.Labels)
			labelsJSON = string(labelsBytes)
		}
		resourceParams[i] = map[string]interface{}{
			"uid":       resource.UID,
			"kind":      resource.Kind,
			"apiGroup":  resource.APIGroup,
			"version":   resource.Version,
			"namespace": resource.Namespace,
			"name":      resource.Name,
			"labels":    labelsJSON,
			"firstSeen": resource.FirstSeen,
			"lastSeen":  resource.LastSeen,
			"deleted":   resource.Deleted,
			"deletedAt": resource.DeletedAt,
		}
	}

	resourcesLiteral := buildCypherListLiteral(resourceParams)
	query := fmt.Sprintf(`
		UNWIND %s AS r
		MERGE (n:ResourceIdentity {uid: r.uid})
		ON CREATE SET
			n.kind = r.kind,
			n.apiGroup = r.apiGroup,
			n.version = r.version,
			n.namespace = r.namespace,
			n.name = r.name,
			n.labels = r.labels,
			n.firstSeen = r.firstSeen,
			n.lastSeen = r.lastSeen,
			n.deleted = r.deleted,
			n.deletedAt = r.deletedAt
		ON MATCH SET
			n.kind = CASE WHEN n.kind IS NULL THEN r.kind ELSE n.kind END,
			n.apiGroup = CASE WHEN n.apiGroup IS NULL THEN r.apiGroup ELSE n.apiGroup END,
			n.version = CASE WHEN n.version IS NULL THEN r.version ELSE n.version END,
			n.namespace = CASE WHEN n.namespace IS NULL THEN r.namespace ELSE n.namespace END,
			n.name = CASE WHEN n.name IS NULL THEN r.name ELSE n.name END,
			n.firstSeen = CASE WHEN n.firstSeen IS NULL THEN r.firstSeen ELSE n.firstSeen END,
			n.labels = CASE WHEN NOT n.deleted THEN r.labels ELSE n.labels END,
			n.lastSeen = CASE WHEN NOT n.deleted THEN r.lastSeen ELSE n.lastSeen END
		RETURN count(n) as upsertedCount
	`, resourcesLiteral)

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateChangeEventsQuery creates a single query to insert multiple ChangeEvent nodes.
func BatchCreateChangeEventsQuery(events []ChangeEvent) GraphQuery {
	eventParams := make([]map[string]interface{}, len(events))
	for i, event := range events {
		containerIssuesJSON := "[]"
		if len(event.ContainerIssues) > 0 {
			issuesBytes, _ := json.Marshal(event.ContainerIssues)
			containerIssuesJSON = string(issuesBytes)
		}

		eventParams[i] = map[string]interface{}{
			"id":              event.ID,
			"timestamp":       event.Timestamp,
			"eventType":       event.EventType,
			"status":          event.Status,
			"errorMessage":    event.ErrorMessage,
			"containerIssues": containerIssuesJSON,
			"configChanged":   event.ConfigChanged,
			"statusChanged":   event.StatusChanged,
			"replicasChanged": event.ReplicasChanged,
			"impactScore":     event.ImpactScore,
			"data":            event.Data,
		}
	}

	eventsLiteral := buildCypherListLiteral(eventParams)
	query := fmt.Sprintf(`
		UNWIND %s AS e
		MERGE (n:ChangeEvent {id: e.id})
		ON CREATE SET
			n.timestamp = e.timestamp,
			n.eventType = e.eventType,
			n.status = e.status,
			n.errorMessage = e.errorMessage,
			n.containerIssues = e.containerIssues,
			n.configChanged = e.configChanged,
			n.statusChanged = e.statusChanged,
			n.replicasChanged = e.replicasChanged,
			n.impactScore = e.impactScore,
			n.data = e.data
		RETURN count(n) as createdCount
	`, eventsLiteral)

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateK8sEventsQuery creates a single query to insert multiple K8sEvent nodes.
func BatchCreateK8sEventsQuery(events []K8sEvent) GraphQuery {
	eventParams := make([]map[string]interface{}, len(events))
	for i, event := range events {
		eventParams[i] = map[string]interface{}{
			"id":        event.ID,
			"timestamp": event.Timestamp,
			"reason":    event.Reason,
			"message":   event.Message,
			"type":      event.Type,
			"count":     event.Count,
			"source":    event.Source,
		}
	}

	eventsLiteral := buildCypherListLiteral(eventParams)
	query := fmt.Sprintf(`
		UNWIND %s AS e
		MERGE (n:K8sEvent {id: e.id})
		ON CREATE SET
			n.timestamp = e.timestamp,
			n.reason = e.reason,
			n.message = e.message,
			n.type = e.type,
			n.count = e.count,
			n.source = e.source
		RETURN count(n) as createdCount
	`, eventsLiteral)

	return GraphQuery{Query: query, Parameters: nil}
}
