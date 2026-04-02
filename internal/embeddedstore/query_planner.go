package embeddedstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type QueryPlanner struct {
	projection *Projection
	hot        *hotStore
	segments   []*segmentReader

	mu              sync.RWMutex
	segmentDimCache map[string]map[segmentDimensionEntry]struct{}
}

func NewQueryPlanner(projection *Projection, hot *hotStore, segments []*segmentReader) *QueryPlanner {
	return newQueryPlanner(projection, hot, segments)
}

func newQueryPlanner(projection *Projection, hot *hotStore, segments []*segmentReader) *QueryPlanner {
	return &QueryPlanner{
		projection:      projection,
		hot:             hot,
		segments:        append([]*segmentReader(nil), segments...),
		segmentDimCache: make(map[string]map[segmentDimensionEntry]struct{}),
	}
}

func (p *QueryPlanner) PlanResourceEvents(ctx context.Context, uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) ([]models.Event, error) {
	if p == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rawEvents, err := p.collectMergedResourceEvents(ctx, uid, meta, startTimeNs, endTimeNs)
	if err != nil {
		return nil, err
	}
	return resourceEventsInWindow(rawEvents, startTimeNs, endTimeNs), nil
}

func (p *QueryPlanner) ExportTimeRange(ctx context.Context, startTimeNs, endTimeNs int64, filters models.QueryFilters) ([]models.Event, error) {
	if p == nil || endTimeNs < startTimeNs {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var exported []models.Event
	for _, reader := range p.relevantExportSegments(filters, startTimeNs, endTimeNs) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events, err := reader.ScanTimeRange(ctx, startTimeNs, endTimeNs)
		if err != nil {
			return nil, fmt.Errorf("scan segment %q by time: %w", reader.meta.ID, err)
		}
		for i := range events {
			if !filters.Matches(events[i].Resource) {
				continue
			}
			exported = append(exported, cloneEvent(events[i]))
		}
	}

	if p.hot != nil {
		for _, event := range p.hot.ScanTimeRange(startTimeNs, endTimeNs) {
			if !filters.Matches(event.Resource) {
				continue
			}
			exported = append(exported, cloneEvent(event))
		}
	}

	if len(exported) == 0 {
		return nil, nil
	}
	sort.SliceStable(exported, func(i, j int) bool {
		return compareEventOrder(exported[i], exported[j]) < 0
	})
	return dedupeEventsByID(exported), nil
}

func (p *QueryPlanner) collectMergedResourceEvents(ctx context.Context, uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) ([]models.Event, error) {
	if uid == "" || endTimeNs < startTimeNs {
		return nil, nil
	}

	var merged []models.Event
	if p.hot != nil {
		merged = cloneEvents(p.hot.RecentEventsByUID(uid))
	}
	for _, reader := range p.relevantSegments(uid, meta, startTimeNs, endTimeNs) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events, err := reader.ScanUID(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("scan segment %q for uid %q: %w", reader.meta.ID, uid, err)
		}
		for i := range events {
			merged = append(merged, cloneEvent(events[i]))
		}
	}

	if len(merged) == 0 {
		return nil, nil
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return compareEventOrder(merged[i], merged[j]) < 0
	})
	return dedupeEventsByID(merged), nil
}

func (p *QueryPlanner) relevantSegments(uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || len(p.segments) == 0 {
		return nil
	}

	relevant := make([]*segmentReader, 0, len(p.segments))
	for _, reader := range p.segments {
		if reader == nil || reader.meta.EventCount == 0 {
			continue
		}
		if reader.meta.MinTimestamp > endTimeNs {
			continue
		}
		if reader.meta.MaxTimestamp < startTimeNs {
			continue
		}
		if !reader.MayContain(meta.Namespace, meta.Kind) {
			continue
		}
		if !p.segmentContainsDimension(reader, meta.Namespace, meta.Kind) {
			continue
		}
		if uid != "" {
			if offsets := reader.resourceIndex[uid]; len(offsets) == 0 {
				continue
			}
		}
		relevant = append(relevant, reader)
	}
	return relevant
}

func (p *QueryPlanner) relevantExportSegments(filters models.QueryFilters, startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || len(p.segments) == 0 {
		return nil
	}

	relevant := make([]*segmentReader, 0, len(p.segments))
	for _, reader := range p.segments {
		if reader == nil || reader.meta.EventCount == 0 {
			continue
		}
		if reader.meta.MinTimestamp > endTimeNs || reader.meta.MaxTimestamp < startTimeNs {
			continue
		}
		if !p.segmentMayMatchFilters(reader, filters) {
			continue
		}
		relevant = append(relevant, reader)
	}
	return relevant
}

func (p *QueryPlanner) segmentContainsDimension(reader *segmentReader, namespace, kind string) bool {
	if reader == nil {
		return false
	}
	if namespace == "" && kind == "" {
		return true
	}

	dimensions, ok := p.segmentDimensions(reader)
	if !ok {
		return false
	}
	_, exists := dimensions[segmentDimensionEntry{Namespace: namespace, Kind: kind}]
	return exists
}

func (p *QueryPlanner) segmentMayMatchFilters(reader *segmentReader, filters models.QueryFilters) bool {
	namespaces := filters.GetNamespaces()
	kinds := filters.GetKinds()
	if len(namespaces) == 0 && len(kinds) == 0 {
		return true
	}

	dimensions, ok := p.segmentDimensions(reader)
	if !ok {
		return true
	}
	for entry := range dimensions {
		if len(namespaces) > 0 && !containsString(namespaces, entry.Namespace) {
			continue
		}
		if len(kinds) > 0 && !containsString(kinds, entry.Kind) {
			continue
		}
		return true
	}
	return false
}

func (p *QueryPlanner) segmentDimensions(reader *segmentReader) (map[segmentDimensionEntry]struct{}, bool) {
	if reader == nil {
		return nil, false
	}
	p.mu.RLock()
	dimensions, ok := p.segmentDimCache[reader.meta.ID]
	p.mu.RUnlock()
	if ok {
		return dimensions, true
	}

	dimPath := filepath.Join(filepath.Dir(reader.eventsPath), segmentDimIndexFile)
	payload, err := os.ReadFile(dimPath)
	if err != nil {
		return nil, false
	}

	var dimIndex segmentDimensionIndex
	if err := json.Unmarshal(payload, &dimIndex); err != nil {
		return nil, false
	}

	dimensions = make(map[segmentDimensionEntry]struct{}, len(dimIndex.Entries))
	for i := range dimIndex.Entries {
		dimensions[dimIndex.Entries[i]] = struct{}{}
	}

	p.mu.Lock()
	p.segmentDimCache[reader.meta.ID] = dimensions
	p.mu.Unlock()
	return dimensions, true
}

func containsString(values []string, target string) bool {
	for i := range values {
		if values[i] == target {
			return true
		}
	}
	return false
}

func resourceEventsInWindow(events []models.Event, startTimeNs, endTimeNs int64) []models.Event {
	if len(events) == 0 {
		return nil
	}

	var inRange []models.Event
	var lastBefore models.Event
	var hasLastBefore bool

	for i := range events {
		event := events[i]
		if event.Timestamp < startTimeNs {
			lastBefore = cloneEvent(event)
			hasLastBefore = true
			continue
		}
		if event.Timestamp > endTimeNs {
			break
		}
		inRange = append(inRange, cloneEvent(event))
	}

	var result []models.Event
	if hasLastBefore && lastBefore.Type != models.EventTypeDelete {
		lastBefore.PreExisting = true
		result = append(result, lastBefore)
	}

	if len(inRange) == 0 && len(result) == 0 {
		return nil
	}

	result = append(result, inRange...)
	return result
}

func dedupeEventsByID(events []models.Event) []models.Event {
	if len(events) <= 1 {
		return events
	}

	deduped := make([]models.Event, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for i := range events {
		if _, ok := seen[events[i].ID]; ok {
			continue
		}
		seen[events[i].ID] = struct{}{}
		deduped = append(deduped, events[i])
	}
	return deduped
}
