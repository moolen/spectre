package storage

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/models"
)

// BenchmarkBuildResourcesWithStatus benchmarks BuildResourcesFromEvents including status inference
// This benchmark specifically measures the impact of the status inference caching optimization
func BenchmarkBuildResourcesWithStatus(b *testing.B) {
	// Generate realistic test data with full resource JSON (including status fields)
	events := generateEventsWithFullResourceData(439, 13)
	builder := NewResourceBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resources := builder.BuildResourcesFromEvents(events)
		// Verify status segments were built (ensures status inference ran)
		if len(resources) == 0 {
			b.Fatal("No resources built")
		}
		for _, resource := range resources {
			if len(resource.StatusSegments) == 0 {
				b.Fatal("No status segments built")
			}
		}
	}
}

// BenchmarkStatusInferenceOnly benchmarks just the status inference part
func BenchmarkStatusInferenceOnly(b *testing.B) {
	events := generateEventsWithFullResourceData(439, 13)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Extract all resource data for status inference
		for _, event := range events {
			if len(event.Data) > 0 {
				// This simulates what happens in BuildStatusSegmentsFromEvents
				_ = analyzer.InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type))
			}
		}
	}
}

// generateEventsWithFullResourceData creates events with realistic full resource JSON
// This includes status fields, conditions, metadata, etc. to simulate real Kubernetes resources
func generateEventsWithFullResourceData(numResources, eventsPerResource int) []models.Event {
	events := make([]models.Event, 0, numResources*eventsPerResource)
	baseTime := time.Now().Unix() * 1e9

	for i := 0; i < numResources; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		for j := 0; j < eventsPerResource; j++ {
			event := models.Event{
				ID:        fmt.Sprintf("event-%d-%d", i, j),
				Timestamp: baseTime + int64(j)*60*1e9,
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:       uid,
					Group:     "",
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      fmt.Sprintf("pod-%d", i),
				},
				Data: generateFullPodResourceData(i, j),
			}
			events = append(events, event)
		}
	}

	return events
}

// generateFullPodResourceData creates a realistic Pod resource JSON
// This includes all fields that status inference checks (conditions, deletion timestamp, etc.)
func generateFullPodResourceData(resourceIdx, eventIdx int) json.RawMessage {
	// Create a realistic Pod resource with conditions and status
	phase := "Running"
	if eventIdx%5 == 0 {
		phase = "Pending"
	}

	readyCondition := "True"
	if eventIdx%7 == 0 {
		readyCondition = "False"
	}

	data := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("pod-%d", resourceIdx),
			"namespace": "default",
			"uid":       fmt.Sprintf("uid-%d", resourceIdx),
		},
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{
					"name":  "app",
					"image": "nginx:latest",
				},
			},
		},
		"status": map[string]interface{}{
			"phase": phase,
			"conditions": []map[string]interface{}{
				{
					"type":   "Ready",
					"status": readyCondition,
					"reason": "ContainersReady",
				},
				{
					"type":   "Initialized",
					"status": "True",
				},
				{
					"type":   "PodScheduled",
					"status": "True",
				},
			},
			"containerStatuses": []map[string]interface{}{
				{
					"name":  "app",
					"ready": readyCondition == "True",
					"state": map[string]interface{}{
						"running": map[string]interface{}{
							"startedAt": "2025-12-17T00:00:00Z",
						},
					},
				},
			},
		},
	}

	bytes, _ := json.Marshal(data)
	return bytes
}
