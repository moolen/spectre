package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// detectChanges compares current event with previous event to detect what changed
func (b *graphBuilder) detectChanges(event models.Event, currentData *analyzer.ResourceData) (configChanged, statusChanged, replicasChanged bool) {
	if b.client == nil {
		if currentData != nil {
			statusChanged = true
		}
		return configChanged, statusChanged, replicasChanged
	}

	var previousEventData []byte

	if b.stateCache != nil {
		if cached := b.stateCache.Get(event.Resource.UID); cached != nil {
			if cached.Timestamp < event.Timestamp && (cached.EventType == "CREATE" || cached.EventType == "UPDATE") {
				previousEventData = cached.Data
				b.logger.Debug("State cache hit for resource %s (cached timestamp=%d, event timestamp=%d)",
					event.Resource.UID, cached.Timestamp, event.Timestamp)
			}
		}
	}

	if previousEventData == nil {
		if cachedEvents, exists := b.batchCache[event.Resource.UID]; exists && len(cachedEvents) > 0 {
			for i := len(cachedEvents) - 1; i >= 0; i-- {
				cached := cachedEvents[i]
				if cached.Timestamp < event.Timestamp && (cached.Type == models.EventTypeCreate || cached.Type == models.EventTypeUpdate) {
					previousEventData = cached.Data
					b.logger.Debug("Found previous event in batch cache: resourceUID=%s, cachedTimestamp=%d, currentTimestamp=%d",
						event.Resource.UID, cached.Timestamp, event.Timestamp)
					break
				}
			}
		}
	}

	if previousEventData == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		b.logger.Debug("Querying database for previous event: resourceUID=%s, timestamp=%d", event.Resource.UID, event.Timestamp)

		query := graph.GraphQuery{
			Query: `
				MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(ce:ChangeEvent)
				WHERE ce.timestamp < $currentTimestamp
				  AND ce.eventType IN ["CREATE", "UPDATE"]
				RETURN ce.data, ce.timestamp
				ORDER BY ce.timestamp DESC
				LIMIT 1
			`,
			Parameters: map[string]interface{}{
				"resourceUID":      event.Resource.UID,
				"currentTimestamp": event.Timestamp,
			},
		}

		result, err := b.client.ExecuteQuery(ctx, query)
		if err != nil {
			b.logger.Debug("Failed to query previous event for resource %s: %v", event.Resource.UID, err)
			if currentData != nil {
				statusChanged = true
			}
			return configChanged, statusChanged, replicasChanged
		}

		if len(result.Rows) == 0 {
			b.logger.Debug("No previous event found in database for resource %s (this is likely the first event)", event.Resource.UID)
			if currentData != nil {
				statusChanged = true
			}
			return configChanged, statusChanged, replicasChanged
		}

		if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
			if dataValue := result.Rows[0][0]; dataValue != nil {
				if dataStr, ok := dataValue.(string); ok && dataStr != "" {
					previousEventData = []byte(dataStr)
					b.logger.Debug("Previous event found in database for resource %s", event.Resource.UID)
				}
			}
		}
	}

	if previousEventData == nil {
		b.logger.Debug("No previous event data available for resource %s", event.Resource.UID)
		if currentData != nil {
			statusChanged = true
		}
		return configChanged, statusChanged, replicasChanged
	}

	if len(event.Data) == 0 {
		b.logger.Debug("Current event has no data for resource %s, skipping change detection", event.Resource.UID)
		return configChanged, statusChanged, replicasChanged
	}

	var currentResource map[string]interface{}
	if err := json.Unmarshal(event.Data, &currentResource); err != nil {
		b.logger.Debug("Failed to parse current event data for change detection: %v", err)
		statusChanged = true
		return configChanged, statusChanged, replicasChanged
	}

	var previousResource map[string]interface{}
	if err := json.Unmarshal(previousEventData, &previousResource); err != nil {
		b.logger.Debug("Failed to parse previous event data for resource %s: %v", event.Resource.UID, err)
	} else {
		b.logger.Debug("Successfully parsed previous event data for resource %s", event.Resource.UID)
	}

	var currentGeneration float64
	var previousGeneration float64

	if metadata, ok := currentResource["metadata"].(map[string]interface{}); ok {
		if gen, ok := metadata["generation"].(float64); ok {
			currentGeneration = gen
		}
	}

	if len(previousResource) > 0 {
		if metadata, ok := previousResource["metadata"].(map[string]interface{}); ok {
			if gen, ok := metadata["generation"].(float64); ok {
				previousGeneration = gen
			}
		}
	}

	b.logger.Debug("Generation comparison for resource %s: current=%v, previous=%v", event.Resource.UID, currentGeneration, previousGeneration)

	isConfigMapOrSecret := event.Resource.Kind == kindConfigMap || event.Resource.Kind == "Secret"
	isRBACResource := event.Resource.Kind == "Role" || event.Resource.Kind == kindClusterRole ||
		event.Resource.Kind == "RoleBinding" || event.Resource.Kind == "ClusterRoleBinding"

	if isConfigMapOrSecret {
		currentDataField, currentHasData := currentResource["data"]
		previousDataField, previousHasData := previousResource["data"]

		if currentHasData != previousHasData {
			configChanged = true
			b.logger.Debug("Config change detected for %s %s: data added/removed", event.Resource.Kind, event.Resource.UID)
		} else if currentHasData && previousHasData {
			if !deepEqual(currentDataField, previousDataField) {
				configChanged = true
				b.logger.Debug("Config change detected for %s %s: data differs", event.Resource.Kind, event.Resource.UID)
			}
		}

		if event.Resource.Kind == kindConfigMap {
			currentBinaryData, currentHasBinaryData := currentResource["binaryData"]
			previousBinaryData, previousHasBinaryData := previousResource["binaryData"]

			if currentHasBinaryData != previousHasBinaryData {
				configChanged = true
				b.logger.Debug("Config change detected for ConfigMap %s: binaryData added/removed", event.Resource.UID)
			} else if currentHasBinaryData && previousHasBinaryData {
				if !deepEqual(currentBinaryData, previousBinaryData) {
					configChanged = true
					b.logger.Debug("Config change detected for ConfigMap %s: binaryData differs", event.Resource.UID)
				}
			}
		}
	} else if isRBACResource {
		if event.Resource.Kind == "Role" || event.Resource.Kind == kindClusterRole {
			currentRules, currentHasRules := currentResource["rules"]
			previousRules, previousHasRules := previousResource["rules"]

			if currentHasRules != previousHasRules {
				configChanged = true
				b.logger.Debug("Config change detected for %s %s: rules added/removed", event.Resource.Kind, event.Resource.UID)
			} else if currentHasRules && previousHasRules {
				if !deepEqual(currentRules, previousRules) {
					configChanged = true
					b.logger.Debug("Config change detected for %s %s: rules differs", event.Resource.Kind, event.Resource.UID)
				}
			}
		} else {
			currentRoleRef, currentHasRoleRef := currentResource["roleRef"]
			previousRoleRef, previousHasRoleRef := previousResource["roleRef"]
			currentSubjects, currentHasSubjects := currentResource["subjects"]
			previousSubjects, previousHasSubjects := previousResource["subjects"]

			if currentHasRoleRef != previousHasRoleRef {
				configChanged = true
				b.logger.Debug("Config change detected for %s %s: roleRef added/removed", event.Resource.Kind, event.Resource.UID)
			} else if currentHasRoleRef && previousHasRoleRef {
				if !deepEqual(currentRoleRef, previousRoleRef) {
					configChanged = true
					b.logger.Debug("Config change detected for %s %s: roleRef differs", event.Resource.Kind, event.Resource.UID)
				}
			}

			if !configChanged {
				if currentHasSubjects != previousHasSubjects {
					configChanged = true
					b.logger.Debug("Config change detected for %s %s: subjects added/removed", event.Resource.Kind, event.Resource.UID)
				} else if currentHasSubjects && previousHasSubjects {
					if !deepEqual(currentSubjects, previousSubjects) {
						configChanged = true
						b.logger.Debug("Config change detected for %s %s: subjects differs", event.Resource.Kind, event.Resource.UID)
					}
				}
			}
		}
	} else if currentGeneration > previousGeneration {
		currentSpec, currentHasSpec := currentResource["spec"]
		previousSpec, previousHasSpec := previousResource["spec"]

		if currentHasSpec != previousHasSpec {
			configChanged = true
			b.logger.Debug("Config change detected for resource %s: spec added/removed", event.Resource.UID)
		} else if currentHasSpec && previousHasSpec {
			if !deepEqual(currentSpec, previousSpec) {
				configChanged = true
				b.logger.Debug("Config change detected for resource %s: spec differs (generation increased from %v to %v)", event.Resource.UID, previousGeneration, currentGeneration)
			} else {
				b.logger.Debug("No config change for resource %s: generation increased but spec is identical (only metadata changed)", event.Resource.UID)
			}
		}
	} else {
		b.logger.Debug("No config change detected for resource %s: current generation %v is not greater than previous %v", event.Resource.UID, currentGeneration, previousGeneration)
	}

	if currentResource["status"] != nil {
		statusChanged = true
	}

	if currentSpec, hasCurrentSpec := currentResource["spec"]; hasCurrentSpec {
		if specMap, ok := currentSpec.(map[string]interface{}); ok {
			if replicas, ok := specMap["replicas"].(float64); ok && replicas >= 0 {
				replicasChanged = false
			}
		}
	}

	return configChanged, statusChanged, replicasChanged
}

// deepEqual performs a deep comparison of two interface{} values.
func deepEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	switch aVal := a.(type) {
	case map[string]interface{}:
		bMap, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bMap) {
			return false
		}
		for key, aValue := range aVal {
			bValue, exists := bMap[key]
			if !exists {
				return false
			}
			if !deepEqual(aValue, bValue) {
				return false
			}
		}
		return true
	case []interface{}:
		bSlice, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bSlice) {
			return false
		}
		for i, aValue := range aVal {
			if !deepEqual(aValue, bSlice[i]) {
				return false
			}
		}
		return true
	case string:
		bStr, ok := b.(string)
		return ok && aVal == bStr
	case float64:
		bNum, ok := b.(float64)
		return ok && aVal == bNum
	case bool:
		bBool, ok := b.(bool)
		return ok && aVal == bBool
	default:
		return a == b
	}
}
