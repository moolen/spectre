package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

const (
	defaultOutputDir   = "./testdata"
	defaultSegmentSize = 10 * 1024 * 1024 // 10MB
	defaultEventCount  = 10000
	defaultDuration    = 1
	defaultNamespaces  = 10
)

var (
	resourceKinds = []string{
		"Pod", "Deployment", "Service", "ConfigMap", "StatefulSet", "DaemonSet", "Secret",
	}
	eventTypes = []models.EventType{
		models.EventTypeCreate,
		models.EventTypeUpdate,
		models.EventTypeDelete,
	}
)

func main() {
	outputDir := flag.String("output-dir", defaultOutputDir, "Output directory for generated files")
	segmentSize := flag.Int64("segment-size", defaultSegmentSize, "Target segment size in bytes")
	eventCount := flag.Int("event-count", defaultEventCount, "Total number of events to generate")
	durationHours := flag.Int("duration-hours", defaultDuration, "Hours of data (1 = single file, >1 = multiple files)")
	namespaces := flag.Int("namespaces", defaultNamespaces, "Number of namespaces to distribute across")
	distribution := flag.String("distribution", "uniform", "Distribution pattern: 'uniform' or 'skewed'")
	seed := flag.Int64("seed", 0, "Random seed (0 = use current time)")

	flag.Parse()

	// Initialize random seed
	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(*seed))

	fmt.Printf("Generating test data with:\n")
	fmt.Printf("  Output directory: %s\n", *outputDir)
	fmt.Printf("  Segment size: %d bytes (%.2f MB)\n", *segmentSize, float64(*segmentSize)/(1024*1024))
	fmt.Printf("  Event count: %d\n", *eventCount)
	fmt.Printf("  Duration: %d hours\n", *durationHours)
	fmt.Printf("  Namespaces: %d\n", *namespaces)
	fmt.Printf("  Distribution: %s\n", *distribution)
	fmt.Printf("  Seed: %d\n", *seed)
	fmt.Println()

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate namespace names
	namespaceNames := generateNamespaceNames(*namespaces)

	// Calculate events per hour
	eventsPerHour := *eventCount / *durationHours
	if eventsPerHour == 0 {
		eventsPerHour = 1
	}

	// Start time (current time, rounded down to the hour)
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	totalEvents := 0
	for hour := 0; hour < *durationHours; hour++ {
		hourTime := startTime.Add(time.Duration(hour) * time.Hour)
		hourTimestamp := hourTime.Unix()

		// Calculate events for this hour
		eventsThisHour := eventsPerHour
		if hour == *durationHours-1 {
			// Last hour gets remaining events
			eventsThisHour = *eventCount - totalEvents
		}

		// Generate filename
		filename := fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
			hourTime.Year(), hourTime.Month(), hourTime.Day(), hourTime.Hour())
		filePath := filepath.Join(*outputDir, filename)

		fmt.Printf("Generating hour %d/%d: %s (%d events)\n",
			hour+1, *durationHours, filename, eventsThisHour)

		// Create block storage file
		bsf, err := storage.NewBlockStorageFile(filePath, hourTimestamp, *segmentSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create storage file %s: %v\n", filePath, err)
			os.Exit(1)
		}

		// Generate events for this hour
		hourStartNs := hourTime.UnixNano()
		hourDurationNs := int64(time.Hour)

		for i := 0; i < eventsThisHour; i++ {
			// Distribute events evenly across the hour
			eventOffsetNs := int64(float64(i) / float64(eventsThisHour) * float64(hourDurationNs))
			eventTimestamp := hourStartNs + eventOffsetNs

			// Select namespace based on distribution
			namespace := selectNamespace(namespaceNames, *distribution, rng)

			// Select random kind
			kind := resourceKinds[rng.Intn(len(resourceKinds))]

			// Select random event type
			eventType := eventTypes[rng.Intn(len(eventTypes))]

			// Create event
			event := createEvent(
				eventTimestamp,
				kind,
				namespace,
				eventType,
				rng,
			)

			if err := bsf.WriteEvent(event); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write event: %v\n", err)
				os.Exit(1)
			}
		}

		// Close file
		if err := bsf.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close storage file %s: %v\n", filePath, err)
			os.Exit(1)
		}

		totalEvents += eventsThisHour
		fmt.Printf("  ✓ Generated %d events in %s\n", eventsThisHour, filename)
	}

	fmt.Printf("\n✓ Successfully generated %d events across %d hour(s)\n", totalEvents, *durationHours)
	fmt.Printf("  Output directory: %s\n", *outputDir)
}

// generateNamespaceNames creates a list of namespace names
func generateNamespaceNames(count int) []string {
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = fmt.Sprintf("namespace-%d", i+1)
	}
	return names
}

// selectNamespace selects a namespace based on the distribution pattern
func selectNamespace(namespaces []string, distribution string, rng *rand.Rand) string {
	if distribution == "skewed" {
		// 80% of events in 20% of namespaces (Pareto distribution)
		hotNamespaceCount := max(1, len(namespaces)/5) // 20% of namespaces
		if rng.Float64() < 0.8 {
			// 80% chance: select from hot namespaces
			return namespaces[rng.Intn(hotNamespaceCount)]
		}
		// 20% chance: select from all namespaces
		return namespaces[rng.Intn(len(namespaces))]
	}
	// Uniform distribution
	return namespaces[rng.Intn(len(namespaces))]
}

// createEvent creates a test event with the specified parameters
func createEvent(timestamp int64, kind, namespace string, eventType models.EventType, rng *rand.Rand) *models.Event {
	// Generate resource name
	resourceName := fmt.Sprintf("%s-%d", kind, rng.Intn(1000000))

	// Generate UID
	uid := uuid.New().String()

	// Determine group based on kind
	group := getGroupForKind(kind)

	// Create resource metadata
	resource := models.ResourceMetadata{
		Group:     group,
		Version:   "v1",
		Kind:      kind,
		Namespace: namespace,
		Name:      resourceName,
		UID:       uid,
	}

	// Generate event data (JSON payload)
	var data json.RawMessage
	if eventType != models.EventTypeDelete {
		// For CREATE and UPDATE, include resource data
		eventData := map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/v1", group),
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": namespace,
				"uid":       uid,
				"labels": map[string]string{
					"app":     resourceName,
					"version": "v1",
				},
			},
			"spec": map[string]interface{}{
				"replicas": rng.Intn(10) + 1,
				"image":    "nginx:latest",
			},
			"generated": true,
			"timestamp": timestamp,
		}
		dataBytes, _ := json.Marshal(eventData)
		data = json.RawMessage(dataBytes)
	}

	// Create event
	event := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: timestamp,
		Type:      eventType,
		Resource:  resource,
		Data:      data,
	}

	return event
}

// getGroupForKind returns the API group for a given Kubernetes resource kind
func getGroupForKind(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps"
	case "Pod", "Service", "ConfigMap", "Secret":
		return ""
	default:
		return ""
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
