// Package enrichment provides pluggable strategies for enriching imported events.
//
// This package defines the Enricher interface and implementations for transforming
// event data after parsing but before consumption. The primary use case is enriching
// Kubernetes Event resources with metadata extracted from the event payload.
package enrichment

import (
	"encoding/json"
	"strings"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// Enricher defines the interface for event enrichment strategies
type Enricher interface {
	// Enrich modifies events in-place, adding or transforming metadata
	Enrich(events []models.Event, logger *logging.Logger)

	// Name returns the enricher's identifier for logging and metrics
	Name() string
}

// Chain applies multiple enrichers in sequence
type Chain struct {
	enrichers []Enricher
}

// NewChain creates a new enrichment chain
func NewChain(enrichers ...Enricher) *Chain {
	return &Chain{enrichers: enrichers}
}

// Enrich applies all enrichers in the chain
func (c *Chain) Enrich(events []models.Event, logger *logging.Logger) {
	for _, enricher := range c.enrichers {
		logger.Debug("Applying enricher: %s", enricher.Name())
		enricher.Enrich(events, logger)
	}
}

// Name returns the chain identifier
func (c *Chain) Name() string {
	return "enrichment-chain"
}

// InvolvedObjectUIDEnricher extracts involvedObject.uid from Kubernetes Event resources
// and populates the InvolvedObjectUID field in resource metadata.
//
// This matches the behavior of the live watcher and is critical for proper event
// correlation in the causal graph.
type InvolvedObjectUIDEnricher struct {
	eventKind string
}

// NewInvolvedObjectUIDEnricher creates the default enricher for Kubernetes Events
func NewInvolvedObjectUIDEnricher() *InvolvedObjectUIDEnricher {
	return &InvolvedObjectUIDEnricher{
		eventKind: "Event",
	}
}

// Name returns the enricher identifier
func (e *InvolvedObjectUIDEnricher) Name() string {
	return "involved-object-uid"
}

// Enrich processes events and extracts involvedObject UIDs
func (e *InvolvedObjectUIDEnricher) Enrich(events []models.Event, logger *logging.Logger) {
	enrichedCount := 0
	skippedCount := 0
	errorCount := 0

	for i := range events {
		event := &events[i]

		// Only process Kubernetes Event resources
		if !strings.EqualFold(event.Resource.Kind, e.eventKind) {
			continue
		}

		// Skip if data is empty
		if len(event.Data) == 0 {
			skippedCount++
			logger.Debug("Skipping event %s: empty data", event.ID)
			continue
		}

		// Skip if already populated
		if event.Resource.InvolvedObjectUID != "" {
			skippedCount++
			logger.Debug("Skipping event %s: InvolvedObjectUID already populated", event.ID)
			continue
		}

		// Extract involvedObject.uid from the JSON data
		var eventData map[string]any
		if err := json.Unmarshal(event.Data, &eventData); err != nil {
			errorCount++
			logger.Warn("Failed to parse event %s data for enrichment: %v", event.ID, err)
			continue
		}

		// Navigate to involvedObject.uid
		involvedObject, ok := eventData["involvedObject"].(map[string]any)
		if !ok {
			skippedCount++
			logger.Debug("Event %s missing involvedObject field", event.ID)
			continue
		}

		uid, ok := involvedObject["uid"].(string)
		if !ok || uid == "" {
			skippedCount++
			logger.Debug("Event %s involvedObject missing uid field", event.ID)
			continue
		}

		// Populate the InvolvedObjectUID field
		event.Resource.InvolvedObjectUID = uid
		enrichedCount++
		logger.Debug("Enriched event %s with involvedObjectUID: %s", event.ID, uid)
	}

	logger.InfoWithFields("Event enrichment completed",
		logging.Field("enricher", e.Name()),
		logging.Field("enriched", enrichedCount),
		logging.Field("skipped", skippedCount),
		logging.Field("errors", errorCount))
}

// Default returns the standard enrichment chain for imports
func Default() *Chain {
	return NewChain(NewInvolvedObjectUIDEnricher())
}
