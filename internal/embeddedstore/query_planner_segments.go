package embeddedstore

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/moolen/spectre/internal/models"
)

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
