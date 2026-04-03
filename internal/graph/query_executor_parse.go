package graph

import (
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// parseTimelineResults converts graph query results into Event objects and K8sEvents.
func (qe *QueryExecutor) parseTimelineResults(result *QueryResult) ([]models.Event, map[string][]models.K8sEvent) {
	var events []models.Event
	k8sEventsByResource := make(map[string][]models.K8sEvent)

	for _, row := range result.Rows {
		if len(row) < 4 {
			qe.logger.Warn("Unexpected row length: %d, expected 4 (r, events, k8sEvents, hasPreExisting)", len(row))
			continue
		}

		resourceProps, err := ParseNodeFromResult(row[0])
		if err != nil {
			qe.logger.Warn("Failed to parse resource node: %v", err)
			continue
		}
		resourceMeta := qe.parseResourceMetadata(resourceProps)

		changeEvents, ok := row[1].([]interface{})
		if !ok {
			qe.logger.Debug("No change events for resource %s", resourceMeta.UID)
			continue
		}

		hasPreExisting := false
		if preExisting, ok := row[3].(bool); ok {
			hasPreExisting = preExisting
		}
		if hasPreExisting && len(changeEvents) > 0 {
			qe.logger.DebugWithFields("Resource has pre-existing event",
				logging.Field("resource", resourceMeta.UID),
				logging.Field("total_events", len(changeEvents)))
		}

		for i, eventData := range changeEvents {
			eventProps, err := ParseNodeFromResult(eventData)
			if err != nil {
				qe.logger.Debug("Failed to parse change event node: %v", err)
				continue
			}

			event := qe.parseChangeEvent(eventProps, resourceMeta)
			if event == nil {
				continue
			}
			if i == 0 && hasPreExisting {
				event.PreExisting = true
			}
			if resourceMeta.Kind == "Pod" && len(event.Data) == 0 {
				qe.logger.Debug("Pod ChangeEvent missing data field: resource=%s/%s, eventID=%s",
					resourceMeta.Namespace, resourceMeta.Name, event.ID)
			}
			events = append(events, *event)
		}

		if k8sEventsData, ok := row[2].([]interface{}); ok && len(k8sEventsData) > 0 {
			for _, k8sEventData := range k8sEventsData {
				k8sEventProps, err := ParseNodeFromResult(k8sEventData)
				if err != nil {
					qe.logger.Debug("Failed to parse K8sEvent node: %v", err)
					continue
				}

				k8sEvent := qe.parseK8sEvent(k8sEventProps)
				if k8sEvent != nil {
					k8sEventsByResource[resourceMeta.UID] = append(k8sEventsByResource[resourceMeta.UID], *k8sEvent)
				}
			}
		}
	}

	qe.logger.Info("Parsed %d events and %d K8sEvents from graph query", len(events), len(k8sEventsByResource))
	return events, k8sEventsByResource
}

// parseK8sEvent converts a K8sEvent graph node to a models.K8sEvent.
func (qe *QueryExecutor) parseK8sEvent(node map[string]interface{}) *models.K8sEvent {
	eventID := getStringField(node, "id")
	if eventID == "" {
		return nil
	}

	return &models.K8sEvent{
		ID:        eventID,
		Timestamp: getInt64Field(node, "timestamp"),
		Reason:    getStringField(node, "reason"),
		Message:   getStringField(node, "message"),
		Type:      getStringField(node, "type"),
		Count:     int32(getInt64Field(node, "count")), // #nosec G115
		Source:    getStringField(node, "source"),
	}
}

// parseResourceMetadata extracts ResourceMetadata from a ResourceIdentity node.
func (qe *QueryExecutor) parseResourceMetadata(node map[string]interface{}) models.ResourceMetadata {
	return models.ResourceMetadata{
		UID:       getStringField(node, "uid"),
		Kind:      getStringField(node, "kind"),
		Group:     getStringField(node, "apiGroup"),
		Version:   getStringField(node, "version"),
		Namespace: getStringField(node, "namespace"),
		Name:      getStringField(node, "name"),
	}
}

// parseChangeEvent converts a ChangeEvent node to a models.Event.
func (qe *QueryExecutor) parseChangeEvent(node map[string]interface{}, resourceMeta models.ResourceMetadata) *models.Event {
	eventID := getStringField(node, "id")
	if eventID == "" {
		return nil
	}

	timestamp := getInt64Field(node, "timestamp")
	eventType := getStringField(node, "eventType")

	var evtType models.EventType
	switch eventType {
	case "CREATE":
		evtType = models.EventTypeCreate
	case "UPDATE":
		evtType = models.EventTypeUpdate
	case "DELETE":
		evtType = models.EventTypeDelete
	default:
		evtType = models.EventTypeUpdate
	}

	dataStr := getStringField(node, "data")
	var data []byte
	if dataStr != "" {
		data = []byte(dataStr)
	}

	return &models.Event{
		ID:        eventID,
		Timestamp: timestamp,
		Type:      evtType,
		Resource:  resourceMeta,
		Data:      data,
	}
}

func getStringField(node map[string]interface{}, key string) string {
	if val, ok := node[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64Field(node map[string]interface{}, key string) int64 {
	if val, ok := node[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}
