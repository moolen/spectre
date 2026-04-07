package embeddedstore

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

var (
	recentEventTimelineCacheSeedFnMu sync.RWMutex
	recentEventTimelineCacheSeedFn   = func(qe *QueryExecutor, ctx context.Context, endTimeNs int64, horizon time.Duration) error {
		return qe.SeedRecentEventTimelineCache(ctx, endTimeNs, horizon)
	}
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
	recentEventCacheMu               sync.RWMutex
	recentEventCache                 []models.Event
	recentEventCacheHorizon          time.Duration
	recentEventCacheCoverageStartNs  int64
	recentEventCacheCoverageEndNs    int64
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

func (qe *QueryExecutor) ConfigureRecentEventTimelineCache(horizon time.Duration) {
	if qe == nil {
		return
	}

	qe.recentEventCacheMu.Lock()
	defer qe.recentEventCacheMu.Unlock()
	qe.recentEventCacheHorizon = horizon
	if horizon <= 0 {
		qe.recentEventCache = nil
		qe.recentEventCacheCoverageStartNs = 0
		qe.recentEventCacheCoverageEndNs = 0
	}
}

func (qe *QueryExecutor) SeedRecentEventTimelineCache(ctx context.Context, endTimeNs int64, horizon time.Duration) error {
	if qe == nil || endTimeNs <= 0 || horizon <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	planner := qe.sharedPlanner()
	if planner == nil {
		return nil
	}

	startTimeNs := endTimeNs - horizon.Nanoseconds()
	events, _, err := planner.exportTimeRange(ctx, startTimeNs, endTimeNs, models.QueryFilters{
		Kinds:   []string{"Event"},
		Version: "v1",
	})
	if err != nil {
		return fmt.Errorf("seed recent event timeline cache: %w", err)
	}

	qe.recentEventCacheMu.Lock()
	defer qe.recentEventCacheMu.Unlock()
	qe.recentEventCacheHorizon = horizon
	qe.recentEventCache = cloneEvents(events)
	qe.recentEventCacheCoverageStartNs = startTimeNs
	qe.recentEventCacheCoverageEndNs = endTimeNs
	qe.pruneRecentEventTimelineCacheLocked(startTimeNs)
	return nil
}

func (qe *QueryExecutor) AppendRecentEventTimelineCache(events []models.Event) {
	if qe == nil || len(events) == 0 {
		return
	}

	qe.recentEventCacheMu.Lock()
	defer qe.recentEventCacheMu.Unlock()
	if qe.recentEventCacheHorizon <= 0 {
		return
	}

	var newestTimestamp int64
	for i := range events {
		if events[i].Resource.Kind != "Event" {
			continue
		}
		qe.recentEventCache = appendOrderedEvent(qe.recentEventCache, cloneEvent(events[i]))
		if events[i].Timestamp > newestTimestamp {
			newestTimestamp = events[i].Timestamp
		}
	}

	if newestTimestamp == 0 {
		return
	}
	coverageEndNs := qe.recentEventCacheCoverageEndNs
	nowNs := time.Now().UnixNano()
	if nowNs > coverageEndNs {
		coverageEndNs = nowNs
	}
	if newestTimestamp > coverageEndNs {
		coverageEndNs = newestTimestamp
	}
	qe.recentEventCacheCoverageEndNs = coverageEndNs
	qe.recentEventCacheCoverageStartNs = coverageEndNs - qe.recentEventCacheHorizon.Nanoseconds()
	qe.pruneRecentEventTimelineCacheLocked(qe.recentEventCacheCoverageStartNs)
}

func (qe *QueryExecutor) recentEventTimelineEvents(
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
) ([]models.Event, queryPlanStats, bool) {
	if qe == nil || endTimeNs < startTimeNs {
		return nil, queryPlanStats{}, false
	}

	kinds := filters.GetKinds()
	if len(kinds) != 1 || kinds[0] != "Event" {
		return nil, queryPlanStats{}, false
	}

	qe.recentEventCacheMu.RLock()
	horizon := qe.recentEventCacheHorizon
	coverageStartNs := qe.recentEventCacheCoverageStartNs
	coverageEndNs := qe.recentEventCacheCoverageEndNs
	if horizon <= 0 || endTimeNs-startTimeNs > horizon.Nanoseconds() ||
		coverageEndNs <= 0 || startTimeNs < coverageStartNs || endTimeNs > coverageEndNs {
		qe.recentEventCacheMu.RUnlock()
		return nil, queryPlanStats{}, false
	}
	cached := cloneEvents(qe.recentEventCache)
	qe.recentEventCacheMu.RUnlock()

	if len(cached) == 0 {
		return nil, queryPlanStats{hotUsed: true}, true
	}

	filtered := make([]models.Event, 0, len(cached))
	for i := range cached {
		event := cached[i]
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			continue
		}
		if !filters.Matches(event.Resource) {
			continue
		}
		filtered = append(filtered, event)
	}
	if len(filtered) == 0 {
		return nil, queryPlanStats{hotUsed: true}, true
	}
	return dedupeEventsByID(filtered), queryPlanStats{hotUsed: true}, true
}

func (qe *QueryExecutor) pruneRecentEventTimelineCacheLocked(cutoffTimestamp int64) {
	if len(qe.recentEventCache) == 0 {
		return
	}

	cutoff := sort.Search(len(qe.recentEventCache), func(i int) bool {
		return qe.recentEventCache[i].Timestamp >= cutoffTimestamp
	})
	if cutoff <= 0 {
		return
	}
	if cutoff >= len(qe.recentEventCache) {
		qe.recentEventCache = nil
		return
	}
	qe.recentEventCache = append([]models.Event(nil), qe.recentEventCache[cutoff:]...)
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

func seedRecentEventTimelineCache(qe *QueryExecutor, ctx context.Context, endTimeNs int64, horizon time.Duration) error {
	recentEventTimelineCacheSeedFnMu.RLock()
	fn := recentEventTimelineCacheSeedFn
	recentEventTimelineCacheSeedFnMu.RUnlock()
	return fn(qe, ctx, endTimeNs, horizon)
}

func setRecentEventTimelineCacheSeedFnForTest(
	fn func(qe *QueryExecutor, ctx context.Context, endTimeNs int64, horizon time.Duration) error,
) func() {
	recentEventTimelineCacheSeedFnMu.Lock()
	previous := recentEventTimelineCacheSeedFn
	recentEventTimelineCacheSeedFn = fn
	recentEventTimelineCacheSeedFnMu.Unlock()

	return func() {
		recentEventTimelineCacheSeedFnMu.Lock()
		recentEventTimelineCacheSeedFn = previous
		recentEventTimelineCacheSeedFnMu.Unlock()
	}
}
