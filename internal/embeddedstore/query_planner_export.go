package embeddedstore

import (
	"context"
	"fmt"
	"sort"

	"github.com/moolen/spectre/internal/models"
)

func (p *QueryPlanner) ExportTimeRange(ctx context.Context, startTimeNs, endTimeNs int64, filters models.QueryFilters) ([]models.Event, error) {
	events, _, err := p.exportTimeRange(ctx, startTimeNs, endTimeNs, filters)
	return events, err
}

func (p *QueryPlanner) exportTimeRange(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
) ([]models.Event, queryPlanStats, error) {
	startTimeNs = p.clampToRetention(startTimeNs)
	if p == nil || endTimeNs < startTimeNs {
		return nil, queryPlanStats{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, queryPlanStats{}, err
	}

	stats := queryPlanStats{}
	var exported []models.Event
	relevant := p.relevantExportSegments(filters, startTimeNs, endTimeNs)
	stats.relevantSegments = len(relevant)
	if len(relevant) > 0 {
		stats.coldUsed = true
		stats.scannedSegments += len(relevant)
	}
	for _, reader := range relevant {
		if err := ctx.Err(); err != nil {
			return nil, stats, err
		}
		events, err := reader.scanTimeRangeLazy(ctx, startTimeNs, endTimeNs, func(event *models.Event) bool {
			return filters.Matches(event.Resource)
		}, true)
		if err != nil {
			return nil, stats, fmt.Errorf("scan segment %q by time: %w", reader.ID(), err)
		}
		for i := range events {
			exported = append(exported, events[i].event)
		}
	}

	if p.hot != nil {
		hotEvents := p.hot.ScanTimeRange(startTimeNs, endTimeNs)
		if len(hotEvents) > 0 {
			stats.hotUsed = true
			stats.hotScans++
		}
		for _, event := range hotEvents {
			if !filters.Matches(event.Resource) {
				continue
			}
			exported = append(exported, cloneEvent(event))
		}
	}

	if len(exported) == 0 {
		return nil, stats, nil
	}

	sort.SliceStable(exported, func(i, j int) bool {
		return compareEventOrder(exported[i], exported[j]) < 0
	})

	return dedupeEventsByID(exported), stats, nil
}
