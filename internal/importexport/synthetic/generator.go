package synthetic

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/models"
)

const defaultSeed int64 = 42

type Config struct {
	Seed           int64 `json:"seed"`
	KindCount      int   `json:"kindCount"`
	ResourceCount  int   `json:"resourceCount"`
	NamespaceCount int   `json:"namespaceCount"`
}

type KindSummary struct {
	Kind          string  `json:"kind"`
	ResourceCount int     `json:"resourceCount"`
	EventCount    int     `json:"eventCount"`
	ResourceShare float64 `json:"resourceShare"`
	EventShare    float64 `json:"eventShare"`
}

type Summary struct {
	Seed                     int64         `json:"seed"`
	KindCount                int           `json:"kindCount"`
	TotalKinds               int           `json:"totalKinds"`
	TotalResources           int           `json:"totalResources"`
	TotalEvents              int           `json:"totalEvents"`
	NamespaceCount           int           `json:"namespaceCount"`
	TopKindsByEvents         []string      `json:"topKindsByEvents"`
	Top20PercentKindEventPct float64       `json:"top20PercentKindEventPct"`
	KindDistribution         []KindSummary `json:"kindDistribution"`
}

func GenerateDataset(outputDir string, config Config) (Summary, error) {
	cfg := normalizeConfig(config)

	if outputDir == "" {
		return Summary{}, fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Summary{}, fmt.Errorf("create output directory: %w", err)
	}

	eventsDir := filepath.Join(outputDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return Summary{}, fmt.Errorf("create events directory: %w", err)
	}

	rng := rand.New(rand.NewSource(cfg.Seed))

	kindNames := make([]string, cfg.KindCount)
	for i := range cfg.KindCount {
		kindNames[i] = fmt.Sprintf("Kind-%03d", i+1)
	}

	permutation := rng.Perm(cfg.KindCount)
	weights := make([]float64, cfg.KindCount)
	for rank, idx := range permutation {
		weights[idx] = 1.0 / math.Pow(float64(rank+1), 1.15)
	}

	resourceCounts := distributeByWeight(cfg.ResourceCount, weights, rng)
	eventCounts := make([]int, cfg.KindCount)
	totalEvents := 0
	resourceSequence := 0

	for kindIdx, resourceCount := range resourceCounts {
		if resourceCount == 0 {
			continue
		}
		kind := kindNames[kindIdx]
		kindDir := filepath.Join(eventsDir, strings.ToLower(kind))
		if err := os.MkdirAll(kindDir, 0o755); err != nil {
			return Summary{}, fmt.Errorf("create kind directory: %w", err)
		}

		kindWeight := weights[kindIdx]
		for resourceNumber := 0; resourceNumber < resourceCount; resourceNumber++ {
			resourceSequence++
			eventsForResource := eventsPerResource(kindWeight, rng)

			namespace := fmt.Sprintf("ns-%02d", resourceSequence%cfg.NamespaceCount+1)
			resourceName := fmt.Sprintf("%s-resource-%04d", strings.ToLower(kind), resourceNumber+1)
			resourceUID := fmt.Sprintf("%s-%06d", strings.ToLower(kind), resourceSequence)

			events := make([]models.Event, 0, eventsForResource)
			for eventNumber := 0; eventNumber < eventsForResource; eventNumber++ {
				eventType := models.EventTypeUpdate
				if eventNumber == 0 {
					eventType = models.EventTypeCreate
				}
				if eventNumber == eventsForResource-1 && rng.Intn(8) == 0 {
					eventType = models.EventTypeDelete
				}

				timestamp := int64(1_700_000_000_000_000_000) + int64(resourceSequence*1_000_000) + int64(eventNumber*1_000)
				eventID := fmt.Sprintf("%s-e-%03d", resourceUID, eventNumber+1)
				payload := json.RawMessage(fmt.Sprintf(`{"generation":%d,"kind":"%s","resource":"%s"}`, eventNumber+1, kind, resourceName))

				events = append(events, models.Event{
					ID:        eventID,
					Timestamp: timestamp,
					Type:      eventType,
					Resource: models.ResourceMetadata{
						Group:     "synthetic.spectre.io",
						Version:   "v1",
						Kind:      kind,
						Namespace: namespace,
						Name:      resourceName,
						UID:       resourceUID,
					},
					Data: payload,
				})
			}

			request := importexport.BatchEventImportRequest{Events: events}
			data, err := json.Marshal(request)
			if err != nil {
				return Summary{}, fmt.Errorf("marshal resource events: %w", err)
			}

			filePath := filepath.Join(kindDir, fmt.Sprintf("%s.json", resourceUID))
			if err := os.WriteFile(filePath, data, 0o644); err != nil {
				return Summary{}, fmt.Errorf("write resource file: %w", err)
			}

			eventCounts[kindIdx] += eventsForResource
			totalEvents += eventsForResource
		}
	}

	summary := buildSummary(cfg, kindNames, resourceCounts, eventCounts, totalEvents)
	summaryBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return Summary{}, fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "summary.json"), summaryBytes, 0o644); err != nil {
		return Summary{}, fmt.Errorf("write summary file: %w", err)
	}

	return summary, nil
}

func normalizeConfig(config Config) Config {
	cfg := config
	if cfg.Seed == 0 {
		cfg.Seed = defaultSeed
	}
	if cfg.KindCount <= 0 {
		cfg.KindCount = 55
	}
	if cfg.ResourceCount <= 0 {
		cfg.ResourceCount = 5000
	}
	if cfg.NamespaceCount <= 0 {
		cfg.NamespaceCount = 20
	}
	return cfg
}

func distributeByWeight(total int, weights []float64, rng *rand.Rand) []int {
	counts := make([]int, len(weights))
	weightSum := 0.0
	for _, w := range weights {
		weightSum += w
	}
	if total <= 0 || weightSum <= 0 {
		return counts
	}

	for i := 0; i < total; i++ {
		draw := rng.Float64() * weightSum
		running := 0.0
		chosen := len(weights) - 1
		for idx, weight := range weights {
			running += weight
			if draw <= running {
				chosen = idx
				break
			}
		}
		counts[chosen]++
	}
	return counts
}

func eventsPerResource(kindWeight float64, rng *rand.Rand) int {
	u := rng.Float64()
	if u > 0.9999 {
		u = 0.9999
	}
	pareto := 1.0 / math.Pow(1.0-u, 1.35)
	weightBoost := 1.0 + (kindWeight * 14.0)
	events := int(math.Round(pareto * weightBoost))
	if events < 1 {
		events = 1
	}
	if events > 250 {
		events = 250
	}
	return events
}

func buildSummary(config Config, kinds []string, resourceCounts []int, eventCounts []int, totalEvents int) Summary {
	distribution := make([]KindSummary, 0, len(kinds))
	totalResources := 0
	for _, count := range resourceCounts {
		totalResources += count
	}

	for idx, kind := range kinds {
		resourceShare := 0.0
		eventShare := 0.0
		if totalResources > 0 {
			resourceShare = float64(resourceCounts[idx]) / float64(totalResources)
		}
		if totalEvents > 0 {
			eventShare = float64(eventCounts[idx]) / float64(totalEvents)
		}
		distribution = append(distribution, KindSummary{
			Kind:          kind,
			ResourceCount: resourceCounts[idx],
			EventCount:    eventCounts[idx],
			ResourceShare: resourceShare,
			EventShare:    eventShare,
		})
	}

	sorted := make([]KindSummary, len(distribution))
	copy(sorted, distribution)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].EventCount == sorted[j].EventCount {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].EventCount > sorted[j].EventCount
	})

	topKinds := make([]string, 0, minInt(10, len(sorted)))
	for i := 0; i < len(sorted) && i < 10; i++ {
		topKinds = append(topKinds, sorted[i].Kind)
	}

	topCount := int(math.Ceil(float64(len(sorted)) * 0.2))
	if topCount < 1 {
		topCount = 1
	}
	topEvents := 0
	for i := 0; i < len(sorted) && i < topCount; i++ {
		topEvents += sorted[i].EventCount
	}
	topPct := 0.0
	if totalEvents > 0 {
		topPct = float64(topEvents) / float64(totalEvents)
	}

	return Summary{
		Seed:                     config.Seed,
		KindCount:                config.KindCount,
		TotalKinds:               len(kinds),
		TotalResources:           totalResources,
		TotalEvents:              totalEvents,
		NamespaceCount:           config.NamespaceCount,
		TopKindsByEvents:         topKinds,
		Top20PercentKindEventPct: topPct,
		KindDistribution:         sorted,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
