package graph

import "encoding/json"

// UpsertResourceIdentityQuery creates a query to insert or update a ResourceIdentity node.
func UpsertResourceIdentityQuery(resource ResourceIdentity) GraphQuery {
	labelsJSON := "{}"
	if resource.Labels != nil && len(resource.Labels) > 0 {
		labelsBytes, _ := json.Marshal(resource.Labels)
		labelsJSON = string(labelsBytes)
	}

	query := `
		MERGE (r:ResourceIdentity {uid: $uid})
		ON CREATE SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.labels = $labels,
			r.firstSeen = $firstSeen,
			r.lastSeen = $lastSeen,
			r.deleted = $deleted,
			r.deletedAt = $deletedAt
	`

	if resource.Deleted {
		query += `
		ON MATCH SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.firstSeen = CASE WHEN r.firstSeen IS NULL THEN $firstSeen ELSE r.firstSeen END,
			r.labels = $labels,
			r.lastSeen = $lastSeen,
			r.deleted = true,
			r.deletedAt = $deletedAt
		`
	} else {
		query += `
		ON MATCH SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.deleted = CASE WHEN r.deleted IS NULL THEN false ELSE r.deleted END,
			r.firstSeen = CASE WHEN r.firstSeen IS NULL THEN $firstSeen ELSE r.firstSeen END,
			r.labels = CASE WHEN NOT COALESCE(r.deleted, false) THEN $labels ELSE r.labels END,
			r.lastSeen = CASE WHEN NOT COALESCE(r.deleted, false) THEN $lastSeen ELSE r.lastSeen END
		`
	}

	return GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
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
		},
	}
}

// CreateChangeEventQuery creates a query to insert a ChangeEvent node.
func CreateChangeEventQuery(event ChangeEvent) GraphQuery {
	containerIssuesJSON := "[]"
	if len(event.ContainerIssues) > 0 {
		issuesBytes, _ := json.Marshal(event.ContainerIssues)
		containerIssuesJSON = string(issuesBytes)
	}

	return GraphQuery{
		Query: `
			MERGE (e:ChangeEvent {id: $id})
			ON CREATE SET
				e.timestamp = $timestamp,
				e.eventType = $eventType,
				e.status = $status,
				e.errorMessage = $errorMessage,
				e.containerIssues = $containerIssues,
				e.configChanged = $configChanged,
				e.statusChanged = $statusChanged,
				e.replicasChanged = $replicasChanged,
				e.impactScore = $impactScore,
				e.data = $data
		`,
		Parameters: map[string]interface{}{
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
		},
	}
}

// CreateK8sEventQuery creates a query to insert a K8sEvent node.
func CreateK8sEventQuery(event K8sEvent) GraphQuery {
	return GraphQuery{
		Query: `
			MERGE (e:K8sEvent {id: $id})
			ON CREATE SET
				e.timestamp = $timestamp,
				e.reason = $reason,
				e.message = $message,
				e.type = $type,
				e.count = $count,
				e.source = $source
		`,
		Parameters: map[string]interface{}{
			"id":        event.ID,
			"timestamp": event.Timestamp,
			"reason":    event.Reason,
			"message":   event.Message,
			"type":      event.Type,
			"count":     event.Count,
			"source":    event.Source,
		},
	}
}

// UpsertDashboardNode creates a query to insert or update a Dashboard node.
func UpsertDashboardNode(dashboard DashboardNode) GraphQuery {
	tagsJSON := "[]"
	if dashboard.Tags != nil && len(dashboard.Tags) > 0 {
		tagsBytes, _ := json.Marshal(dashboard.Tags)
		tagsJSON = string(tagsBytes)
	}

	return GraphQuery{
		Query: `
			MERGE (d:Dashboard {uid: $uid})
			ON CREATE SET
				d.title = $title,
				d.version = $version,
				d.tags = $tags,
				d.folder = $folder,
				d.url = $url,
				d.firstSeen = $firstSeen,
				d.lastSeen = $lastSeen
			ON MATCH SET
				d.title = $title,
				d.version = $version,
				d.tags = $tags,
				d.folder = $folder,
				d.url = $url,
				d.lastSeen = $lastSeen
		`,
		Parameters: map[string]interface{}{
			"uid":       dashboard.UID,
			"title":     dashboard.Title,
			"version":   dashboard.Version,
			"tags":      tagsJSON,
			"folder":    dashboard.Folder,
			"url":       dashboard.URL,
			"firstSeen": dashboard.FirstSeen,
			"lastSeen":  dashboard.LastSeen,
		},
	}
}
