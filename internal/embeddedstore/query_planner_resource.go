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

	relevant := p.relevantResourceSegments(uid, meta, startTimeNs, endTimeNs)
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
			return nil, stats, fmt.Errorf("scan segment %q for uid %q: %w", reader.ID(), uid, err)
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

func (p *QueryPlanner) collectAssociatedEvents(
	ctx context.Context,
	involvedUIDs []string,
	namespaces []string,
	startTimeNs, endTimeNs int64,
) (map[string][]models.Event, queryPlanStats, error) {
	if p == nil || len(involvedUIDs) == 0 || endTimeNs < startTimeNs {
		return nil, queryPlanStats{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, queryPlanStats{}, err
	}

	targetUIDs := make(map[string]struct{}, len(involvedUIDs))
	for i := range involvedUIDs {
		if involvedUIDs[i] == "" {
			continue
		}
		targetUIDs[involvedUIDs[i]] = struct{}{}
	}
	if len(targetUIDs) == 0 {
		return nil, queryPlanStats{}, nil
	}

	filters := models.QueryFilters{
		Kinds:      []string{"Event"},
		Namespaces: namespaces,
	}
	stats := queryPlanStats{}
	eventsByUID := make(map[string][]models.Event, len(targetUIDs))

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
		events, err := reader.ScanTimeRange(ctx, startTimeNs, endTimeNs)
		if err != nil {
			return nil, stats, fmt.Errorf("scan segment %q for associated events: %w", reader.ID(), err)
		}
		for i := range events {
			event := events[i]
			if event.Resource.Kind != "Event" || !filters.Matches(event.Resource) {
				continue
			}
			if _, ok := targetUIDs[event.Resource.InvolvedObjectUID]; !ok {
				continue
			}
			eventsByUID[event.Resource.InvolvedObjectUID] = append(eventsByUID[event.Resource.InvolvedObjectUID], cloneEvent(event))
		}
	}

	if p.hot != nil {
		hotUsed := false
		for uid := range targetUIDs {
			events := p.hot.RecentAssociatedEventsByUID(uid)
			if len(events) == 0 {
				continue
			}
			hotUsed = true
			for i := range events {
				event := events[i]
				if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
					continue
				}
				if event.Resource.Kind != "Event" || !filters.Matches(event.Resource) {
					continue
				}
				eventsByUID[uid] = append(eventsByUID[uid], cloneEvent(event))
			}
		}
		if hotUsed {
			stats.hotUsed = true
			stats.hotScans++
		}
	}

	if len(eventsByUID) == 0 {
		return nil, stats, nil
	}

	for uid := range eventsByUID {
		sort.SliceStable(eventsByUID[uid], func(i, j int) bool {
			return compareEventOrder(eventsByUID[uid][i], eventsByUID[uid][j]) < 0
		})
		eventsByUID[uid] = dedupeEventsByID(eventsByUID[uid])
	}

	return eventsByUID, stats, nil
}
