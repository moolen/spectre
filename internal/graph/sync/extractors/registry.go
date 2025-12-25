package extractors

import (
	"context"
	"sort"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ExtractorRegistry manages relationship extractors
type ExtractorRegistry struct {
	extractors []RelationshipExtractor
	lookup     ResourceLookup
	logger     *logging.Logger
}

// NewExtractorRegistry creates a new registry
func NewExtractorRegistry(lookup ResourceLookup) *ExtractorRegistry {
	return &ExtractorRegistry{
		extractors: []RelationshipExtractor{},
		lookup:     lookup,
		logger:     logging.GetLogger("extractors.registry"),
	}
}

// Register adds an extractor to the registry
func (r *ExtractorRegistry) Register(extractor RelationshipExtractor) {
	r.extractors = append(r.extractors, extractor)

	// Sort by priority (lower = earlier execution)
	sort.Slice(r.extractors, func(i, j int) bool {
		return r.extractors[i].Priority() < r.extractors[j].Priority()
	})

	r.logger.Debug("Registered extractor: %s (priority: %d)", extractor.Name(), extractor.Priority())
}

// Extract applies all matching extractors to an event
func (r *ExtractorRegistry) Extract(ctx context.Context, event models.Event) ([]graph.Edge, error) {
	var allEdges []graph.Edge

	for _, extractor := range r.extractors {
		if !extractor.Matches(event) {
			continue
		}

		r.logger.Debug("Applying extractor %s to %s/%s", extractor.Name(), event.Resource.Kind, event.Resource.Name)

		edges, err := extractor.ExtractRelationships(ctx, event, r.lookup)
		if err != nil {
			// Log but continue - partial extraction is acceptable
			r.logger.Warn("Extractor %s failed for event %s: %v", extractor.Name(), event.ID, err)
			continue
		}

		r.logger.Debug("Extractor %s produced %d edges", extractor.Name(), len(edges))
		allEdges = append(allEdges, edges...)
	}

	return allEdges, nil
}

// Count returns the number of registered extractors
func (r *ExtractorRegistry) Count() int {
	return len(r.extractors)
}

// ListExtractors returns the names of all registered extractors
func (r *ExtractorRegistry) ListExtractors() []string {
	names := make([]string, len(r.extractors))
	for i, ext := range r.extractors {
		names[i] = ext.Name()
	}
	return names
}
