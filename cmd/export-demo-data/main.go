package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"sort"

	"github.com/moolen/spectre/internal/demo"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

const (
	queryStartTimestamp = int64(0)                     // Seconds since epoch (0 for deterministic offsets)
	queryEndTimestamp   = queryStartTimestamp + 6*3600 // Cover six hours of demo activity
)

type demoDataset struct {
	Version   int            `json:"version"`
	TimeRange demoTimeRange  `json:"timeRange"`
	Resources []demoResource `json:"resources"`
	Metadata  demoMetadata   `json:"metadata"`
	MaxOffset int64          `json:"maxOffsetSeconds"`
}

type demoTimeRange struct {
	EarliestOffsetSec int64 `json:"earliestOffsetSec"`
	LatestOffsetSec   int64 `json:"latestOffsetSec"`
}

type demoMetadata struct {
	Namespaces     []string       `json:"namespaces"`
	Kinds          []string       `json:"kinds"`
	Groups         []string       `json:"groups"`
	ResourceCounts map[string]int `json:"resourceCounts"`
	TotalEvents    int            `json:"totalEvents"`
}

type demoResource struct {
	ID             string              `json:"id"`
	Group          string              `json:"group"`
	Version        string              `json:"version"`
	Kind           string              `json:"kind"`
	Namespace      string              `json:"namespace"`
	Name           string              `json:"name"`
	StatusSegments []demoStatusSegment `json:"statusSegments"`
	Events         []demoK8sEvent      `json:"events,omitempty"`
}

type demoStatusSegment struct {
	StartOffsetSec int64           `json:"startOffsetSec"`
	EndOffsetSec   int64           `json:"endOffsetSec"`
	Status         string          `json:"status"`
	Message        string          `json:"message,omitempty"`
	ResourceData   json.RawMessage `json:"resourceData,omitempty"`
}

type demoK8sEvent struct {
	ID                      string `json:"id"`
	TimestampOffsetSec      int64  `json:"timestampOffsetSec"`
	Reason                  string `json:"reason"`
	Message                 string `json:"message"`
	Type                    string `json:"type"`
	Count                   int32  `json:"count"`
	Source                  string `json:"source,omitempty"`
	FirstTimestampOffsetSec *int64 `json:"firstTimestampOffsetSec,omitempty"`
	LastTimestampOffsetSec  *int64 `json:"lastTimestampOffsetSec,omitempty"`
}

func main() {
	outputPath := flag.String("output", "ui/src/demo/demo-data.json", "output file for serialized demo dataset")
	flag.Parse()

	if err := exportDemoDataset(*outputPath); err != nil {
		log.Fatalf("failed to export demo dataset: %v", err)
	}
}

func exportDemoDataset(outputPath string) error {
	executor := demo.NewDemoQueryExecutor()

	query := &models.QueryRequest{
		StartTimestamp: queryStartTimestamp,
		EndTimestamp:   queryEndTimestamp,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := executor.Execute(query)
	if err != nil {
		return err
	}

	resourceBuilder := storage.NewResourceBuilder()
	resourceMap := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	// Attach Kubernetes events for richer timelines
	eventQuery := &models.QueryRequest{
		StartTimestamp: queryStartTimestamp,
		EndTimestamp:   queryEndTimestamp,
		Filters: models.QueryFilters{
			Kind:    "Event",
			Version: "v1",
		},
	}

	eventResult, err := executor.Execute(eventQuery)
	if err != nil {
		return err
	}

	resourceBuilder.AttachK8sEvents(resourceMap, eventResult.Events)

	resources := convertResources(resourceMap)
	metadata := buildMetadataSummary(queryResult.Events)
	timeRange := computeTimeRange(queryResult.Events)
	maxOffset := calculateMaxOffset(resources)

	dataset := demoDataset{
		Version:   1,
		TimeRange: timeRange,
		Resources: resources,
		Metadata:  metadata,
		MaxOffset: maxOffset,
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(dataset)
}

func convertResources(resourceMap map[string]*models.Resource) []demoResource {
	resources := make([]demoResource, 0, len(resourceMap))

	for _, res := range resourceMap {
		demoRes := demoResource{
			ID:        res.ID,
			Group:     res.Group,
			Version:   res.Version,
			Kind:      res.Kind,
			Namespace: res.Namespace,
			Name:      res.Name,
		}

		demoRes.StatusSegments = make([]demoStatusSegment, 0, len(res.StatusSegments))
		for _, segment := range res.StatusSegments {
			demoRes.StatusSegments = append(demoRes.StatusSegments, demoStatusSegment{
				StartOffsetSec: segment.StartTime - queryStartTimestamp,
				EndOffsetSec:   segment.EndTime - queryStartTimestamp,
				Status:         segment.Status,
				Message:        segment.Message,
				ResourceData:   segment.ResourceData,
			})
		}

		if len(res.Events) > 0 {
			demoRes.Events = make([]demoK8sEvent, 0, len(res.Events))
			for _, evt := range res.Events {
				demoRes.Events = append(demoRes.Events, convertK8sEvent(evt))
			}
		}

		resources = append(resources, demoRes)
	}

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Kind == resources[j].Kind {
			if resources[i].Namespace == resources[j].Namespace {
				return resources[i].Name < resources[j].Name
			}
			return resources[i].Namespace < resources[j].Namespace
		}
		return resources[i].Kind < resources[j].Kind
	})

	return resources
}

func convertK8sEvent(event models.K8sEvent) demoK8sEvent {
	demoEvt := demoK8sEvent{
		ID:                 event.ID,
		TimestampOffsetSec: event.Timestamp - queryStartTimestamp,
		Reason:             event.Reason,
		Message:            event.Message,
		Type:               event.Type,
		Count:              event.Count,
		Source:             event.Source,
	}

	if event.FirstTimestamp > 0 {
		offset := event.FirstTimestamp - queryStartTimestamp
		demoEvt.FirstTimestampOffsetSec = &offset
	}
	if event.LastTimestamp > 0 {
		offset := event.LastTimestamp - queryStartTimestamp
		demoEvt.LastTimestampOffsetSec = &offset
	}

	return demoEvt
}

func buildMetadataSummary(events []models.Event) demoMetadata {
	namespaces := make(map[string]struct{})
	kinds := make(map[string]struct{})
	groups := make(map[string]struct{})
	resourceCounts := make(map[string]int)

	for _, evt := range events {
		namespaces[evt.Resource.Namespace] = struct{}{}
		kinds[evt.Resource.Kind] = struct{}{}
		groups[evt.Resource.Group] = struct{}{}
		resourceCounts[evt.Resource.Kind]++
	}

	return demoMetadata{
		Namespaces:     toSortedSlice(namespaces),
		Kinds:          toSortedSlice(kinds),
		Groups:         toSortedSlice(groups),
		ResourceCounts: resourceCounts,
		TotalEvents:    len(events),
	}
}

func computeTimeRange(events []models.Event) demoTimeRange {
	var earliest, latest int64 = -1, -1
	for _, evt := range events {
		ts := evt.Timestamp / 1e9
		if earliest == -1 || ts < earliest {
			earliest = ts
		}
		if latest == -1 || ts > latest {
			latest = ts
		}
	}

	if earliest == -1 {
		earliest = 0
	}
	if latest == -1 {
		latest = 0
	}

	return demoTimeRange{
		EarliestOffsetSec: earliest - queryStartTimestamp,
		LatestOffsetSec:   latest - queryStartTimestamp,
	}
}

func calculateMaxOffset(resources []demoResource) int64 {
	var maxOffset int64

	for _, res := range resources {
		for _, seg := range res.StatusSegments {
			if seg.EndOffsetSec > maxOffset {
				maxOffset = seg.EndOffsetSec
			}
		}
		for _, evt := range res.Events {
			if evt.TimestampOffsetSec > maxOffset {
				maxOffset = evt.TimestampOffsetSec
			}
			if evt.LastTimestampOffsetSec != nil && *evt.LastTimestampOffsetSec > maxOffset {
				maxOffset = *evt.LastTimestampOffsetSec
			}
		}
	}

	return maxOffset
}

func toSortedSlice(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for val := range set {
		values = append(values, val)
	}
	sort.Strings(values)
	return values
}
