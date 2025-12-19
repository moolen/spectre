package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// StorageQuerier defines the interface for querying Spectre storage
type StorageQuerier interface {
	// Query retrieves events matching the given filters within a time range
	Query(ctx context.Context, request models.QueryRequest) (*models.QueryResult, error)
}

// Rebuilder handles rebuilding the graph from Spectre storage
type Rebuilder struct {
	storage  StorageQuerier
	pipeline Pipeline
	logger   *logging.Logger
}

// NewRebuilder creates a new graph rebuilder
func NewRebuilder(storage StorageQuerier, pipeline Pipeline) *Rebuilder {
	return &Rebuilder{
		storage:  storage,
		pipeline: pipeline,
		logger:   logging.GetLogger("graph.sync.rebuild"),
	}
}

// RebuildOptions configures graph rebuild behavior
type RebuildOptions struct {
	// TimeWindow is how far back to rebuild (default: 24h)
	TimeWindow time.Duration

	// BatchSize is how many events to process at once
	BatchSize int

	// IncludeK8sEvents determines if K8s Event objects should be included
	IncludeK8sEvents bool
}

// DefaultRebuildOptions returns default rebuild options
func DefaultRebuildOptions() RebuildOptions {
	return RebuildOptions{
		TimeWindow:       24 * time.Hour,
		BatchSize:        1000,
		IncludeK8sEvents: true,
	}
}

// Rebuild populates the graph from Spectre storage
func (r *Rebuilder) Rebuild(ctx context.Context, opts RebuildOptions) error {
	r.logger.Info("Starting graph rebuild (window: %v, batchSize: %d)", opts.TimeWindow, opts.BatchSize)

	startTime := time.Now().Add(-opts.TimeWindow)
	endTime := time.Now()

	// Query all events in the time window
	r.logger.Info("Querying events from %v to %v", startTime, endTime)

	request := models.QueryRequest{
		StartTimestamp: startTime.Unix(),
		EndTimestamp:   endTime.Unix(),
		Filters:        models.QueryFilters{}, // No filters - get all events
	}

	result, err := r.storage.Query(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to query storage: %w", err)
	}

	r.logger.Info("Retrieved %d events for rebuild", result.Count)

	if result.Count == 0 {
		r.logger.Info("No events to rebuild, graph will be empty")
		return nil
	}

	// Process events in batches
	totalEvents := len(result.Events)
	for i := 0; i < totalEvents; i += opts.BatchSize {
		end := i + opts.BatchSize
		if end > totalEvents {
			end = totalEvents
		}

		batch := result.Events[i:end]

		// Filter out K8s Events if not included
		if !opts.IncludeK8sEvents {
			filtered := make([]models.Event, 0, len(batch))
			for _, event := range batch {
				if event.Resource.Kind != "Event" {
					filtered = append(filtered, event)
				}
			}
			batch = filtered
		}

		if len(batch) == 0 {
			continue
		}

		r.logger.Info("Processing rebuild batch %d-%d of %d", i+1, end, totalEvents)

		// Process batch through pipeline
		if err := r.pipeline.ProcessBatch(ctx, batch); err != nil {
			r.logger.Warn("Failed to process batch %d-%d: %v", i+1, end, err)
			// Continue processing remaining batches
			continue
		}
	}

	r.logger.Info("Graph rebuild complete: processed %d events", totalEvents)

	// Get pipeline stats
	stats := r.pipeline.GetStats()
	r.logger.Info("Rebuild stats: %d nodes created, %d edges created, %d causality links",
		stats.NodesCreated, stats.EdgesCreated, stats.CausalityLinksFound)

	return nil
}

// RebuildIfEmpty rebuilds the graph only if it's empty
func (r *Rebuilder) RebuildIfEmpty(ctx context.Context, opts RebuildOptions, checkEmpty func(context.Context) (bool, error)) error {
	r.logger.Info("Checking if graph is empty...")
	isEmpty, err := checkEmpty(ctx)
	if err != nil {
		r.logger.Error("Failed to check if graph is empty: %v", err)
		return fmt.Errorf("failed to check if graph is empty: %w", err)
	}

	r.logger.Info("Graph empty check result: isEmpty=%v", isEmpty)
	
	if !isEmpty {
		r.logger.Info("Graph is not empty, skipping rebuild")
		return nil
	}

	r.logger.Info("Graph is empty, starting rebuild")
	return r.Rebuild(ctx, opts)
}

// PartialRebuild rebuilds only specific resource kinds
func (r *Rebuilder) PartialRebuild(ctx context.Context, kinds []string, opts RebuildOptions) error {
	r.logger.Info("Starting partial graph rebuild for kinds: %v", kinds)

	startTime := time.Now().Add(-opts.TimeWindow)
	endTime := time.Now()

	totalProcessed := 0

	// Query each kind separately
	for _, kind := range kinds {
		r.logger.Info("Querying events for kind: %s", kind)

		request := models.QueryRequest{
			StartTimestamp: startTime.Unix(),
			EndTimestamp:   endTime.Unix(),
			Filters: models.QueryFilters{
				Kind: kind,
			},
		}

		result, err := r.storage.Query(ctx, request)
		if err != nil {
			r.logger.Warn("Failed to query kind %s: %v", kind, err)
			continue
		}

		r.logger.Info("Retrieved %d events for kind %s", result.Count, kind)

		if result.Count == 0 {
			continue
		}

		// Process in batches
		for i := 0; i < len(result.Events); i += opts.BatchSize {
			end := i + opts.BatchSize
			if end > len(result.Events) {
				end = len(result.Events)
			}

			batch := result.Events[i:end]

			if err := r.pipeline.ProcessBatch(ctx, batch); err != nil {
				r.logger.Warn("Failed to process batch for kind %s: %v", kind, err)
				continue
			}

			totalProcessed += len(batch)
		}
	}

	r.logger.Info("Partial rebuild complete: processed %d events across %d kinds",
		totalProcessed, len(kinds))

	return nil
}
