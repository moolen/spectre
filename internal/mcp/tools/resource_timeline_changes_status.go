package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/models"
)

// summarizeConditions extracts and summarizes status conditions across segments.
func (t *ResourceTimelineChangesTool) summarizeConditions(segments []models.StatusSegment) map[string]string {
	result := make(map[string]string)
	conditionTimeline := buildConditionTimeline(segments)

	importantConditions := []string{"Ready", "Available", "Progressing", "Initialized", "ContainersReady", "PodScheduled"}
	for _, condType := range importantConditions {
		timeline := conditionTimeline[condType]
		if len(timeline) == 0 {
			continue
		}

		parts := make([]string, 0, len(timeline))
		for i, state := range timeline {
			duration := ""
			if i < len(timeline)-1 {
				durationSec := timeline[i+1].startTime - state.startTime
				duration = fmt.Sprintf("(%s)", formatDuration(durationSec))
			}
			parts = append(parts, fmt.Sprintf("%s%s", state.status, duration))
		}
		if len(parts) > 0 {
			result[condType] = strings.Join(parts, " -> ")
		}
	}

	return result
}

type conditionState struct {
	status    string
	startTime int64
}

func buildConditionTimeline(segments []models.StatusSegment) map[string][]conditionState {
	conditionTimeline := make(map[string][]conditionState)
	for _, segment := range segments {
		conditions := extractSegmentConditions(segment)
		for conditionType, status := range conditions {
			timeline := conditionTimeline[conditionType]
			if len(timeline) == 0 || timeline[len(timeline)-1].status != status {
				conditionTimeline[conditionType] = append(timeline, conditionState{
					status:    status,
					startTime: segment.StartTime,
				})
			}
		}
	}

	return conditionTimeline
}

func extractSegmentConditions(segment models.StatusSegment) map[string]string {
	if len(segment.ResourceData) == 0 {
		return nil
	}

	var resourceData map[string]any
	if err := json.Unmarshal(segment.ResourceData, &resourceData); err != nil {
		return nil
	}

	status, ok := resourceData["status"].(map[string]any)
	if !ok {
		return nil
	}

	conditions, ok := status["conditions"].([]any)
	if !ok {
		return nil
	}

	result := make(map[string]string)
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}

		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		if condType == "" {
			continue
		}

		result[condType] = condStatus
	}

	return result
}

// formatDuration is defined in cluster_health.go in the same package.
