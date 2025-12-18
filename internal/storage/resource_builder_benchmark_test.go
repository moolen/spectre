package storage

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// BenchmarkBuildResourcesFromEvents benchmarks the optimized BuildResourcesFromEvents function
func BenchmarkBuildResourcesFromEvents(b *testing.B) {
	scenarios := []struct {
		name          string
		numResources  int
		eventsPerResource int
	}{
		{"Small_10_resources_100_events", 10, 10},
		{"Medium_100_resources_1000_events", 100, 10},
		{"Large_439_resources_6000_events", 439, 13}, // Real-world scenario
		{"XLarge_1000_resources_10000_events", 1000, 10},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			events := generateBenchmarkEvents(scenario.numResources, scenario.eventsPerResource)
			builder := NewResourceBuilder()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = builder.BuildResourcesFromEvents(events)
			}
		})
	}
}

// BenchmarkBuildResourcesFromEvents_WithK8sEvents benchmarks with Kubernetes Events attached
func BenchmarkBuildResourcesFromEvents_WithK8sEvents(b *testing.B) {
	// Generate events similar to real-world scenario
	numResources := 439
	eventsPerResource := 13
	numK8sEvents := 1500

	baseEvents := generateBenchmarkEvents(numResources, eventsPerResource)
	k8sEvents := generateBenchmarkK8sEvents(numK8sEvents, numResources)
	allEvents := append(baseEvents, k8sEvents...)

	builder := NewResourceBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resources := builder.BuildResourcesFromEvents(baseEvents)
		builder.AttachK8sEvents(resources, allEvents)
	}
}

// BenchmarkBuildStatusSegments benchmarks the old non-optimized path
func BenchmarkBuildStatusSegments(b *testing.B) {
	numEvents := 6000
	resourceUID := "test-uid-1"
	events := generateBenchmarkEventsForUID(resourceUID, numEvents)
	builder := NewResourceBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.BuildStatusSegments(resourceUID, events)
	}
}

// BenchmarkBuildStatusSegmentsFromEvents benchmarks the new optimized path
func BenchmarkBuildStatusSegmentsFromEvents(b *testing.B) {
	numEvents := 6000
	resourceUID := "test-uid-1"
	events := generateBenchmarkEventsForUID(resourceUID, numEvents)
	builder := NewResourceBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.BuildStatusSegmentsFromEvents(events)
	}
}

// Helper functions for benchmark data generation

func generateBenchmarkEvents(numResources, eventsPerResource int) []models.Event {
	events := make([]models.Event, 0, numResources*eventsPerResource)
	baseTime := time.Now().Unix() * 1e9

	for i := 0; i < numResources; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		for j := 0; j < eventsPerResource; j++ {
			event := models.Event{
				ID:        fmt.Sprintf("event-%d-%d", i, j),
				Timestamp: baseTime + int64(j)*60*1e9, // 1 minute apart
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:       uid,
					Group:     "",
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      fmt.Sprintf("pod-%d", i),
				},
				Data: generateBenchmarkResourceData(i, j),
			}
			events = append(events, event)
		}
	}

	return events
}

func generateBenchmarkEventsForUID(uid string, numEvents int) []models.Event {
	events := make([]models.Event, 0, numEvents)
	baseTime := time.Now().Unix() * 1e9

	// Add mostly unrelated events (to simulate filtering overhead)
	for i := 0; i < numEvents-13; i++ {
		event := models.Event{
			ID:        fmt.Sprintf("other-event-%d", i),
			Timestamp: baseTime + int64(i)*60*1e9,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				UID:       fmt.Sprintf("other-uid-%d", i),
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      fmt.Sprintf("other-pod-%d", i),
			},
			Data: generateBenchmarkResourceData(i, 0),
		}
		events = append(events, event)
	}

	// Add target events
	for i := 0; i < 13; i++ {
		event := models.Event{
			ID:        fmt.Sprintf("target-event-%d", i),
			Timestamp: baseTime + int64(i)*60*1e9,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				UID:       uid,
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "target-pod",
			},
			Data: generateBenchmarkResourceData(0, i),
		}
		events = append(events, event)
	}

	return events
}

func generateBenchmarkK8sEvents(numEvents, numResources int) []models.Event {
	events := make([]models.Event, 0, numEvents)
	baseTime := time.Now().Unix() * 1e9

	for i := 0; i < numEvents; i++ {
		targetUID := fmt.Sprintf("uid-%d", i%numResources)
		event := models.Event{
			ID:        fmt.Sprintf("k8s-event-%d", i),
			Timestamp: baseTime + int64(i)*60*1e9,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				UID:              fmt.Sprintf("k8s-event-uid-%d", i),
				Group:            "",
				Version:          "v1",
				Kind:             "Event",
				Namespace:        "default",
				Name:             fmt.Sprintf("event-%d", i),
				InvolvedObjectUID: targetUID,
			},
			Data: generateBenchmarkK8sEventData(i),
		}
		events = append(events, event)
	}

	return events
}

func generateBenchmarkResourceData(resourceIdx, eventIdx int) json.RawMessage {
	data := map[string]interface{}{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("pod-%d", resourceIdx),
			"namespace": "default",
		},
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []map[string]interface{}{
				{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	bytes, _ := json.Marshal(data)
	return bytes
}

func generateBenchmarkK8sEventData(eventIdx int) json.RawMessage {
	data := map[string]interface{}{
		"kind":       "Event",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("event-%d", eventIdx),
			"namespace": "default",
		},
		"reason":  "Started",
		"message": fmt.Sprintf("Benchmark event %d", eventIdx),
		"type":    "Normal",
		"count":   1,
		"source": map[string]interface{}{
			"component": "kubelet",
		},
	}
	bytes, _ := json.Marshal(data)
	return bytes
}
