package embeddedstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (qe *QueryExecutor) ExportTimeRange(ctx context.Context, query *models.QueryRequest) ([]models.Event, error) {
	start := time.Now()
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	stats := queryPlanStats{}
	planner := qe.sharedPlanner()
	if planner != nil {
		events, planStats, err := planner.exportTimeRange(ctx, startTimeNs, endTimeNs, query.Filters)
		qe.recordQueryMetrics(queryFamilyExportTimeRange, planStats, start, err)
		return events, err
	}
	if !qe.projectionHistoryFallbackEnabled {
		err := fmt.Errorf("projection history fallback disabled")
		qe.recordQueryMetrics(queryFamilyExportTimeRange, stats, start, err)
		return nil, err
	}

	stats.projectionUsed = true

	projectionEvents := qe.snapshotProjectionEvents()
	exported := make([]models.Event, 0)
	for i := range projectionEvents {
		event := projectionEvents[i]
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			continue
		}
		if !query.Filters.Matches(event.Resource) {
			continue
		}
		exported = append(exported, cloneEvent(event))
	}

	qe.recordQueryMetrics(queryFamilyExportTimeRange, stats, start, nil)
	return exported, nil
}

func (qe *QueryExecutor) QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error) {
	start := time.Now()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		qe.recordQueryMetrics(queryFamilyDistinctMeta, queryPlanStats{}, start, err)
		return nil, nil, 0, 0, err
	}

	namespacesSet := make(map[string]struct{})
	kindsSet := make(map[string]struct{})
	minTime = -1
	maxTime = -1
	stats := queryPlanStats{projectionUsed: true}
	planner := qe.sharedPlanner()

	projection := qe.projection
	projection.mu.RLock()
	projectionMinTime, projectionMaxTime := projection.minTimestampNs, projection.maxTimestampNs
	// Full-range metadata warmups only need projection state: scanning per-resource
	// history is unnecessary when the caller asks for the entire persisted range.
	fullRange := projectionMinTime >= 0 && startTimeNs <= projectionMinTime && endTimeNs >= projectionMaxTime
	if !fullRange && planner == nil && !qe.projectionHistoryFallbackEnabled {
		projection.mu.RUnlock()
		err := fmt.Errorf("projection history fallback disabled")
		qe.recordQueryMetrics(queryFamilyDistinctMeta, stats, start, err)
		return nil, nil, 0, 0, err
	}
	for uid, meta := range projection.resourceMetaByUID {
		if !fullRange {
			first, last, ok := resourceRecordWindowBounds(projection.resourcesByUID[uid], startTimeNs, endTimeNs)
			if !ok {
				continue
			}
			if minTime < 0 || first < minTime {
				minTime = first
			}
			if maxTime < 0 || last > maxTime {
				maxTime = last
			}
		}
		namespacesSet[meta.Namespace] = struct{}{}
		kindsSet[meta.Kind] = struct{}{}
	}
	projection.mu.RUnlock()
	if fullRange {
		minTime = projectionMinTime
		maxTime = projectionMaxTime
	}

	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	for ns := range namespacesSet {
		namespaces = append(namespaces, ns)
	}
	for kind := range kindsSet {
		kinds = append(kinds, kind)
	}

	sort.Strings(namespaces)
	sort.Strings(kinds)

	qe.recordQueryMetrics(queryFamilyDistinctMeta, stats, start, nil)
	return namespaces, kinds, minTime, maxTime, nil
}

