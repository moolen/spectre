package api

import (
	"context"

	"github.com/moolen/spectre/internal/models"
)

// QueryExecutor defines the interface for executing queries against stored events
type QueryExecutor interface {
	Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error)
	SetSharedCache(cache interface{})
}

// EventIngestor defines the interface for ingesting single events.
type EventIngestor interface {
	ProcessEvent(ctx context.Context, event models.Event) error
}

// BatchIngestor defines the interface for ingesting event batches.
type BatchIngestor interface {
	ProcessBatch(ctx context.Context, events []models.Event) error
}

// TimelineQuerySource specifies which executor to use for queries
type TimelineQuerySource string

const (
	TimelineQuerySourceStorage TimelineQuerySource = "storage"
	TimelineQuerySourceGraph   TimelineQuerySource = "graph"
)
