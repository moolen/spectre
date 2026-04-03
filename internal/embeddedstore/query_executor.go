package embeddedstore

import (
	"context"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type filteredResource struct {
	orderedResourceKey
	events []models.Event
}

// QueryExecutor executes embedded timeline queries against a shared projection.
type QueryExecutor struct {
	logger     *logging.Logger
	projection *Projection
	planner    *QueryPlanner
	metrics    *Metrics
}

func NewQueryExecutor(projection *Projection) *QueryExecutor {
	if projection == nil {
		projection = NewProjection()
	}

	return &QueryExecutor{
		logger:     logging.GetLogger("embedded.query"),
		projection: projection,
	}
}

func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
	planner, ok := cache.(*QueryPlanner)
	if !ok {
		return
	}

	qe.planner = planner
}

func (qe *QueryExecutor) SetMetrics(metrics *Metrics) {
	qe.metrics = metrics
}
