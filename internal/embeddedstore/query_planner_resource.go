package embeddedstore

import (
	"context"
	"fmt"
	"sort"

	"github.com/moolen/spectre/internal/models"
)

func (p *QueryPlanner) PlanResourceEvents(ctx context.Context, uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) ([]models.Event, error) {
	events, _, err := p.planResourceEvents(ctx, uid, meta, startTimeNs, endTimeNs)
	return events, err
}

func (p *QueryPlanner) planResourceEvents(
	ctx context.Context,
	uid string,
	meta models.ResourceMetadata,
	startTimeNs, endTimeNs int64,
) ([]models.Event, queryPlanStats, error) {
	if p == nil {
		return nil, queryPlanStats{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, queryPlanStats{}, err
	}

	rawEvents, stats, err := p.collectMergedResourceEvents(ctx, uid, meta, startTimeNs, endTimeNs)
	if err != nil {
		return nil, queryPlanStats{}, err
	}

	return resourceEventsInWindow(rawEvents, startTimeNs, endTimeNs), stats, nil
}

func (p *QueryPlanner) collectMergedResourceEvents(
	ctx context.Context,
	uid string,
	meta models.ResourceMetadata,
	startTimeNs, endTimeNs int64,
) ([]models.Event, queryPlanStats, error) {
	if uid == "" || endTimeNs < startTimeNs {
		return nil, queryPlanStats{}, nil
	}

	stats := queryPlanStats{}
	var merged []models.Event
	if p.hot != nil {
		hotEvents := p.hot.RecentEventsByUID(uid)
		if len(hotEvents) > 0 {
			stats.hotUsed = true
			stats.hotScans++
			merged = cloneEvents(hotEvents)
		}
	}

	relevant := p.relevantSegments(uid, meta, startTimeNs, endTimeNs)
	stats.relevantSegments = len(relevant)
	if len(relevant) > 0 {
		stats.coldUsed = true
		stats.scannedSegments += len(relevant)
		stats.uidDiskLookups += len(relevant)
	}
	for _, reader := range relevant {
		if err := ctx.Err(); err != nil {
			return nil, stats, err
		}
		events, err := reader.ScanUID(ctx, uid)
		if err != nil {
			return nil, stats, fmt.Errorf("scan segment %q for uid %q: %w", reader.meta.ID, uid, err)
		}
		for i := range events {
			merged = append(merged, cloneEvent(events[i]))
		}
	}

	if len(merged) == 0 {
		return nil, stats, nil
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return compareEventOrder(merged[i], merged[j]) < 0
	})

	return dedupeEventsByID(merged), stats, nil
}
