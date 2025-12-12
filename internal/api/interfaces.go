package api

import (
	"context"

	"github.com/moolen/spectre/internal/models"
)

// QueryExecutor defines the interface for executing queries against stored events
type QueryExecutor interface {
	Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error)
}
