package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

type DemoData struct {
	Version   int                    `json:"version"`
	TimeRange TimeRange              `json:"timeRange"`
	Resources []DemoResource         `json:"resources"`
	Metadata  MetadataSection        `json:"metadata"`
	MaxOffset int64                  `json:"maxOffsetSeconds"`
}

type TimeRange struct {
	EarliestOffsetSec int64 `json:"earliestOffsetSec"`
	LatestOffsetSec   int64 `json:"latestOffsetSec"`
}

type DemoResource struct {
	ID              string        `json:"id"`
	Group           string        `json:"group"`
	Version         string        `json:"version"`
	Kind            string        `json:"kind"`
	Namespace       string        `json:"namespace"`
	Name            string        `json:"name"`
	StatusSegments  []DemoSegment `json:"statusSegments"`
}

type DemoSegment struct {
	StartOffsetSec int64           `json:"startOffsetSec"`
	EndOffsetSec   int64           `json:"endOffsetSec"`
	Status         string          `json:"status"`
	Message        string          `json:"message"`
	ResourceData   json.RawMessage `json:"resourceData,omitempty"`
}

type MetadataSection struct {
	Namespaces     []string       `json:"namespaces"`
	Kinds          []string       `json:"kinds"`
	Groups         []string       `json:"groups"`
	ResourceCounts map[string]int `json:"resourceCounts"`
	TotalEvents    int            `json:"totalEvents"`
}

func main() {
	inputDir := flag.String("input", "./demo-data", "Directory containing .bin files")
	outputFile := flag.String("output", "./ui/src/demo/demo-data.json", "Output JSON file")
	flag.Parse()

	// Read all .bin files
	files, err := filepath.Glob(filepath.Join(*inputDir, "*.bin"))
	if err != nil {
		log.Fatalf("failed to list .bin files: %v", err)
	}

	if len(files) == 0 {
		log.Fatalf("no .bin files found in %s", *inputDir)
	}

	// Sort files chronologically
	sort.Strings(files)

	var allEvents []*models.Event
	var earliestTime int64
	var latestTime int64

	// Read all events from all .bin files
	for _, filePath := range files {
		events, err := readBinFile(filePath)
		if err != nil {
			log.Printf("warning: failed to read %s: %v", filePath, err)
			continue
		}

		for _, event := range events {
			allEvents = append(allEvents, event)
			if earliestTime == 0 || event.Timestamp < earliestTime {
				earliestTime = event.Timestamp
			}
			if event.Timestamp > latestTime {
				latestTime = event.Timestamp
			}
		}
	}

	if len(allEvents) == 0 {
		log.Fatal("no events found in .bin files")
	}

	log.Printf("loaded %d total events", len(allEvents))

	// Convert nanoseconds to seconds
	earliestSec := earliestTime / 1e9
	latestSec := latestTime / 1e9

	// Build resources with status segments
	demoData := buildDemoData(allEvents, earliestSec, latestSec)

	// Write to JSON
	output, err := json.MarshalIndent(demoData, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(*outputFile, output, 0644); err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}

	log.Printf("wrote demo data to %s", *outputFile)
	log.Printf("time range: %d - %d seconds (%d seconds total)", earliestSec, latestSec, latestSec-earliestSec)
	log.Printf("resources: %d", len(demoData.Resources))
}

func readBinFile(filePath string) ([]*models.Event, error) {
	reader, err := storage.NewBlockReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	_, err = reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	footer, err := reader.ReadFileFooter()
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	index, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	var events []*models.Event

	for i := range index.BlockMetadata {
		blockMeta := index.BlockMetadata[i]
		blockEvents, err := reader.ReadBlockEvents(blockMeta)
		if err != nil {
			log.Printf("warning: failed to read block %d: %v", blockMeta.ID, err)
			continue
		}

		events = append(events, blockEvents...)
	}

	return events, nil
}

func buildDemoData(allEvents []*models.Event, earliestSec, latestSec int64) *DemoData {
	// Filter out Event resources, group by UID
	resourceMap := make(map[string]*DemoResource)
	var namespaceSet, kindSet, groupSet map[string]bool
	namespaceSet = make(map[string]bool)
	kindSet = make(map[string]bool)
	groupSet = make(map[string]bool)
	resourceCounts := make(map[string]int)

	// Filter and process events
	var baseEvents []*models.Event
	for _, event := range allEvents {
		if strings.EqualFold(event.Resource.Kind, "Event") {
			continue
		}
		baseEvents = append(baseEvents, event)
	}

	// Create resource map
	for _, event := range baseEvents {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}

		if _, exists := resourceMap[uid]; !exists {
			resourceMap[uid] = &DemoResource{
				ID:        uid,
				Group:     event.Resource.Group,
				Version:   event.Resource.Version,
				Kind:      event.Resource.Kind,
				Namespace: event.Resource.Namespace,
				Name:      event.Resource.Name,
			}
		}

		namespaceSet[event.Resource.Namespace] = true
		kindSet[event.Resource.Kind] = true
		groupSet[event.Resource.Group] = true
		resourceCounts[event.Resource.Kind]++
	}

	// Build status segments for each resource
	for uid, resource := range resourceMap {
		segments := buildStatusSegments(uid, baseEvents, earliestSec, latestSec)
		resource.StatusSegments = segments
	}

	// Convert sets to sorted slices
	namespaces := setToSlice(namespaceSet)
	kinds := setToSlice(kindSet)
	groups := setToSlice(groupSet)

	// Sort resources by name
	var resources []DemoResource
	for _, r := range resourceMap {
		resources = append(resources, *r)
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Namespace != resources[j].Namespace {
			return resources[i].Namespace < resources[j].Namespace
		}
		return resources[i].Name < resources[j].Name
	})

	return &DemoData{
		Version: 1,
		TimeRange: TimeRange{
			EarliestOffsetSec: 0,
			LatestOffsetSec:   latestSec - earliestSec,
		},
		Resources: resources,
		Metadata: MetadataSection{
			Namespaces:     namespaces,
			Kinds:          kinds,
			Groups:         groups,
			ResourceCounts: resourceCounts,
			TotalEvents:    len(baseEvents),
		},
		MaxOffset: latestSec - earliestSec,
	}
}

func buildStatusSegments(resourceUID string, allEvents []*models.Event, earliestSec, latestSec int64) []DemoSegment {
	// Filter events for this resource
	var resourceEvents []*models.Event
	for _, event := range allEvents {
		if event.Resource.UID == resourceUID {
			resourceEvents = append(resourceEvents, event)
		}
	}

	// Sort by timestamp
	sort.Slice(resourceEvents, func(i, j int) bool {
		return resourceEvents[i].Timestamp < resourceEvents[j].Timestamp
	})

	var segments []DemoSegment

	for i, event := range resourceEvents {
		eventSecStart := event.Timestamp / 1e9
		eventSecEnd := eventSecStart
		if i+1 < len(resourceEvents) {
			eventSecEnd = resourceEvents[i+1].Timestamp / 1e9
		} else {
			// For last event, use latestTime
			eventSecEnd = latestSec
		}

		// Convert to offsets from earliest
		startOffset := eventSecStart - earliestSec
		endOffset := eventSecEnd - earliestSec

		status := inferStatus(event.Resource.Kind, event.Data, string(event.Type))
		message := generateMessage(string(event.Type))

		segment := DemoSegment{
			StartOffsetSec: startOffset,
			EndOffsetSec:   endOffset,
			Status:         status,
			Message:        message,
			ResourceData:   event.Data,
		}

		segments = append(segments, segment)
	}

	return segments
}

func inferStatus(kind string, data json.RawMessage, eventType string) string {
	if strings.EqualFold(eventType, "DELETE") {
		return "Terminating"
	}

	if len(data) == 0 {
		if strings.EqualFold(eventType, "CREATE") || strings.EqualFold(eventType, "UPDATE") {
			return "Ready"
		}
		return "Unknown"
	}

	// Simple status inference based on kind
	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		return inferReplicaStatus(data)
	case "pod":
		return inferPodStatus(data)
	case "node":
		return inferNodeStatus(data)
	case "service", "configmap", "secret":
		return "Ready"
	default:
		return "Ready"
	}
}

func inferReplicaStatus(data json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "Ready"
	}

	status, ok := obj["status"].(map[string]any)
	if !ok {
		return "Ready"
	}

	// Check for unavailable replicas
	if unavailable, ok := status["unavailableReplicas"].(float64); ok && unavailable > 0 {
		return "Warning"
	}

	// Check for conditions
	if conditions, ok := status["conditions"].([]any); ok {
		for _, cond := range conditions {
			if condMap, ok := cond.(map[string]any); ok {
				if condType, ok := condMap["type"].(string); ok {
					if strings.EqualFold(condType, "Progressing") {
						if condStatus, ok := condMap["status"].(string); ok && strings.EqualFold(condStatus, "False") {
							return "Error"
						}
					}
				}
			}
		}
	}

	// Check ready replicas
	if readyReplicas, ok := status["readyReplicas"].(float64); ok && readyReplicas > 0 {
		return "Ready"
	}

	return "Warning"
}

func inferPodStatus(data json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "Ready"
	}

	status, ok := obj["status"].(map[string]any)
	if !ok {
		return "Ready"
	}

	if phase, ok := status["phase"].(string); ok {
		switch strings.ToLower(phase) {
		case "running":
			return "Ready"
		case "pending":
			return "Warning"
		case "failed":
			return "Error"
		case "succeeded":
			return "Ready"
		case "unknown":
			return "Warning"
		}
	}

	return "Ready"
}

func inferNodeStatus(data json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "Ready"
	}

	status, ok := obj["status"].(map[string]any)
	if !ok {
		return "Ready"
	}

	// Check for disk pressure or other pressure conditions
	if conditions, ok := status["conditions"].([]any); ok {
		for _, cond := range conditions {
			if condMap, ok := cond.(map[string]any); ok {
				if condType, ok := condMap["type"].(string); ok {
					if strings.Contains(strings.ToLower(condType), "pressure") {
						if condStatus, ok := condMap["status"].(string); ok && strings.EqualFold(condStatus, "True") {
							return "Warning"
						}
					}
				}
			}
		}
	}

	return "Ready"
}

func generateMessage(eventType string) string {
	switch strings.ToUpper(eventType) {
	case "CREATE":
		return "Resource created"
	case "UPDATE":
		return "Resource updated"
	case "DELETE":
		return "Resource deleted"
	default:
		return "Resource modified"
	}
}

func setToSlice(m map[string]bool) []string {
	var result []string
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
