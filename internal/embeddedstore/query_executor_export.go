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
	if qe.planner != nil {
		events, planStats, err := qe.planner.exportTimeRange(ctx, startTimeNs, endTimeNs, query.Filters)
		qe.recordQueryMetrics(queryFamilyExportTimeRange, planStats, start, err)
		return events, err
	}

	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()
	stats.projectionUsed = true

	exported := make([]models.Event, 0)
	for i := range qe.projection.events {
		event := qe.projection.events[i]
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
	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()

	namespacesSet := make(map[string]struct{})
	kindsSet := make(map[string]struct{})
	minTime = -1
	maxTime = -1

	for _, key := range qe.projection.orderedResources {
		meta, ok := qe.projection.resourceMetaByUID[key.uid]
		if !ok {
			continue
		}

		resourceEvents := qe.collectResourceEvents(qe.projection.eventsByResourceUID[key.uid], startTimeNs, endTimeNs)
		if len(resourceEvents) == 0 {
			continue
		}

		namespacesSet[meta.Namespace] = struct{}{}
		kindsSet[meta.Kind] = struct{}{}
		for _, event := range resourceEvents {
			if minTime < 0 || event.Timestamp < minTime {
				minTime = event.Timestamp
			}
			if maxTime < 0 || event.Timestamp > maxTime {
				maxTime = event.Timestamp
			}
		}
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

	qe.recordQueryMetrics(queryFamilyDistinctMeta, queryPlanStats{projectionUsed: true}, start, nil)
	return namespaces, kinds, minTime, maxTime, nil
}
