package embeddedstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type filteredResource struct {
	orderedResourceKey
	events []models.Event
}

// QueryExecutor executes embedded timeline queries against a shared projection.
type QueryExecutor struct {
	logger                           *logging.Logger
	projection                       *Projection
	plannerMu                        sync.RWMutex
	planner                          *QueryPlanner
	metrics                          *Metrics
	projectionHistoryFallbackEnabled bool
}

func NewQueryExecutor(projection *Projection) *QueryExecutor {
	if projection == nil {
		projection = NewProjection()
	}

	return &QueryExecutor{
		logger:                           logging.GetLogger("embedded.query"),
		projection:                       projection,
		projectionHistoryFallbackEnabled: true,
	}
}

// AttachesK8sEventsToResources reports that resource queries return each resource's
// core/v1 Events in QueryResult.K8sEventsByResource.
func (qe *QueryExecutor) AttachesK8sEventsToResources() bool {
	return true
}

func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
	var planner *QueryPlanner
	if typedPlanner, ok := cache.(*QueryPlanner); ok {
		planner = typedPlanner
	}
	qe.plannerMu.Lock()
	defer qe.plannerMu.Unlock()
	qe.planner = planner
}

func (qe *QueryExecutor) SetMetrics(metrics *Metrics) {
	qe.metrics = metrics
}

func (qe *QueryExecutor) DisableProjectionHistoryFallback() {
	qe.projectionHistoryFallbackEnabled = false
}

func (qe *QueryExecutor) sharedPlanner() *QueryPlanner {
	if qe == nil {
		return nil
	}
	qe.plannerMu.RLock()
	defer qe.plannerMu.RUnlock()
	return qe.planner
}

func (qe *QueryExecutor) warmTimelineCaches(ctx context.Context, endTimeNs int64, windows []time.Duration, pageSize int) error {
	if qe == nil || endTimeNs <= 0 || len(windows) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if pageSize <= 0 {
		pageSize = models.DefaultPageSize
	}

	for _, window := range windows {
		if window <= 0 {
			continue
		}

		startTimeNs := endTimeNs - window.Nanoseconds()
		resources, _, _, err := qe.collectPaginatedResources(ctx, startTimeNs, endTimeNs, models.QueryFilters{}, nil, pageSize)
		if err != nil {
			return err
		}
		if _, _, err := qe.collectK8sEventsForResources(ctx, resources, startTimeNs, endTimeNs); err != nil {
			return err
		}
		if _, _, err := qe.eventTimelineEvents(ctx, startTimeNs, endTimeNs, models.QueryFilters{
			Kinds:   []string{"Event"},
			Version: "v1",
		}); err != nil {
			return err
		}
	}

	return nil
}

func (qe *QueryExecutor) warmRecentAssociatedEventIndexes(
	ctx context.Context,
	endTimeNs int64,
	horizon time.Duration,
	maxSegments int,
	maxBytes int64,
) (int, error) {
	if qe == nil || endTimeNs <= 0 || horizon <= 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	planner := qe.sharedPlanner()
	if planner == nil {
		return 0, nil
	}

	startTimeNs := endTimeNs - horizon.Nanoseconds()
	candidates := planner.recentAssociatedEventSegments(startTimeNs, endTimeNs)
	if len(candidates) == 0 {
		return 0, nil
	}
	if maxSegments > 0 && len(candidates) > maxSegments {
		candidates = candidates[:maxSegments]
	}

	loaded := 0
	var warmedBytes int64
	for _, reader := range candidates {
		if reader == nil || reader.AssociatedIndexLoaded() {
			continue
		}
		indexBytes := reader.associatedIndexSizeBytes()
		if maxBytes > 0 && loaded > 0 && warmedBytes+indexBytes > maxBytes {
			break
		}
		if err := reader.ensureAssociatedIndexLoaded(); err != nil {
			return loaded, fmt.Errorf("warm recent associated index for segment %q: %w", reader.ID(), err)
		}
		loaded++
		warmedBytes += indexBytes
	}

	return loaded, nil
}
