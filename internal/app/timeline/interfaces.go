package timeline

import (
	"context"

	"github.com/moolen/spectre/internal/models"
)

type QueryExecutor interface {
	Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error)
	SetSharedCache(cache interface{})
}

type EventIngestor interface {
	ProcessEvent(ctx context.Context, event models.Event) error
}

type BatchIngestor interface {
	ProcessBatch(ctx context.Context, events []models.Event) error
}

type QuerySource string

const (
	QuerySourceStorage QuerySource = "storage"
	QuerySourceGraph   QuerySource = "graph"
)
