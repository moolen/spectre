package api

import "github.com/moolen/spectre/internal/models"

// QueryExecutor defines the interface for executing queries against stored events
type QueryExecutor interface {
	Execute(query *models.QueryRequest) (*models.QueryResult, error)
}
