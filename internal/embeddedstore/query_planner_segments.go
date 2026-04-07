package embeddedstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/moolen/spectre/internal/models"
)

func (p *QueryPlanner) relevantSegments(uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || len(p.segments) == 0 {
		return nil
	}

	relevant := make([]*segmentReader, 0, len(p.segments))
	for _, reader := range p.segments {
		if reader == nil {
			continue
		}
		segmentMeta, metaKnown := queryPlannerSegmentMeta(reader)
		if metaKnown && segmentMeta.EventCount == 0 {
			continue
		}
		if metaKnown && segmentMeta.MinTimestamp > endTimeNs {
			continue
		}
		if metaKnown && segmentMeta.MaxTimestamp < startTimeNs {
			continue
		}
		if !reader.MayContain(meta.Namespace, meta.Kind) {
			continue
		}
		if !p.segmentContainsDimension(reader, meta.Namespace, meta.Kind) {
			continue
		}
		if uid != "" {
			if !reader.MayContainUID(uid) {
				continue
			}
		}
		relevant = append(relevant, reader)
	}

	return relevant
}

func (p *QueryPlanner) relevantResourceSegments(uid string, meta models.ResourceMetadata, startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || len(p.segments) == 0 {
		return nil
	}

	candidates := p.resourceSegmentCandidates(uid)
	relevant := make([]*segmentReader, 0, len(candidates))
	var latestPrior *segmentReader
	for _, reader := range candidates {
		if reader == nil {
			continue
		}
		segmentMeta, metaKnown := queryPlannerSegmentMeta(reader)
		if metaKnown && segmentMeta.EventCount == 0 {
			continue
		}
		if metaKnown && segmentMeta.MinTimestamp > endTimeNs {
			continue
		}
		if !reader.MayContain(meta.Namespace, meta.Kind) {
			continue
		}
		if !p.segmentContainsDimension(reader, meta.Namespace, meta.Kind) {
			continue
		}
		if uid != "" {
			if !reader.MayContainUID(uid) {
				continue
			}
		}

		if metaKnown && segmentMeta.MaxTimestamp < startTimeNs {
			latestPriorMeta, latestPriorKnown := queryPlannerSegmentMeta(latestPrior)
			if latestPrior == nil || !latestPriorKnown || segmentMeta.MaxTimestamp > latestPriorMeta.MaxTimestamp {
				latestPrior = reader
			}
			continue
		}

		if !metaKnown || (segmentMeta.MinTimestamp <= endTimeNs && segmentMeta.MaxTimestamp >= startTimeNs) {
			relevant = append(relevant, reader)
		}
	}

	if latestPrior != nil {
		relevant = append([]*segmentReader{latestPrior}, relevant...)
	}

	return relevant
}

func (p *QueryPlanner) resourceSegmentCandidates(uid string) []*segmentReader {
	if p == nil || len(p.segments) == 0 || uid == "" {
		return p.segments
	}

	p.mu.RLock()
	routed := append([]*segmentReader(nil), p.uidSegmentRoute[uid]...)
	hasFallback := len(p.uidRouteFallbackByID) > 0
	hasPending := len(p.uidRoutePendingByID) > 0
	if !hasFallback && !hasPending {
		p.mu.RUnlock()
		return routed
	}

	routedByID := make(map[string]struct{}, len(routed))
	for _, reader := range routed {
		if reader == nil {
			continue
		}
		routedByID[reader.ID()] = struct{}{}
	}
	fallbackByID := make(map[string]struct{}, len(p.uidRouteFallbackByID))
	for segmentID := range p.uidRouteFallbackByID {
		fallbackByID[segmentID] = struct{}{}
	}
	pendingByID := make(map[string]struct{}, len(p.uidRoutePendingByID))
	for segmentID := range p.uidRoutePendingByID {
		pendingByID[segmentID] = struct{}{}
	}
	p.mu.RUnlock()

	candidates := make([]*segmentReader, 0, len(routed)+len(fallbackByID)+len(pendingByID))
	for _, reader := range p.segments {
		if reader == nil {
			continue
		}
		readerID := reader.ID()
		if _, ok := routedByID[readerID]; ok {
			candidates = append(candidates, reader)
			continue
		}
		if _, ok := fallbackByID[readerID]; ok {
			candidates = append(candidates, reader)
			continue
		}
		if _, ok := pendingByID[readerID]; ok {
			candidates = append(candidates, reader)
		}
	}
	return candidates
}

func (p *QueryPlanner) relevantExportSegments(filters models.QueryFilters, startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || len(p.segments) == 0 {
		return nil
	}

	relevant := make([]*segmentReader, 0, len(p.segments))
	for _, reader := range p.segments {
		if reader == nil {
			continue
		}
		segmentMeta, metaKnown := queryPlannerSegmentMeta(reader)
		if metaKnown && segmentMeta.EventCount == 0 {
			continue
		}
		if metaKnown && (segmentMeta.MinTimestamp > endTimeNs || segmentMeta.MaxTimestamp < startTimeNs) {
			continue
		}
		if !p.segmentMayMatchFilters(reader, filters) {
			continue
		}
		relevant = append(relevant, reader)
	}

	return relevant
}

func (p *QueryPlanner) recentAssociatedEventSegments(startTimeNs, endTimeNs int64) []*segmentReader {
	if p == nil || endTimeNs < startTimeNs {
		return nil
	}

	relevant := p.relevantExportSegments(models.QueryFilters{Kinds: []string{"Event"}}, startTimeNs, endTimeNs)
	sort.SliceStable(relevant, func(i, j int) bool {
		leftMeta, leftKnown := queryPlannerSegmentMeta(relevant[i])
		rightMeta, rightKnown := queryPlannerSegmentMeta(relevant[j])
		switch {
		case leftKnown && rightKnown:
			if leftMeta.MaxTimestamp != rightMeta.MaxTimestamp {
				return leftMeta.MaxTimestamp > rightMeta.MaxTimestamp
			}
			if leftMeta.MinTimestamp != rightMeta.MinTimestamp {
				return leftMeta.MinTimestamp > rightMeta.MinTimestamp
			}
		case leftKnown:
			return true
		case rightKnown:
			return false
		}

		return relevant[i].ID() > relevant[j].ID()
	})
	return relevant
}

func queryPlannerSegmentMeta(reader *segmentReader) (segmentBundleMeta, bool) {
	if reader == nil {
		return segmentBundleMeta{}, false
	}

	meta, loaded := reader.bundleMeta()
	if loaded {
		return meta, true
	}

	meta, err := reader.EnsureBundleMeta()
	if err != nil {
		return meta, false
	}
	return meta, true
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
	dimensions, ok := p.segmentDimCache[reader.ID()]
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
	p.segmentDimCache[reader.ID()] = dimensions
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
