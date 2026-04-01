package sync

import (
	"sync"
)

// LabelIndex provides fast lookup of resources by label selectors.
// This eliminates the need to query the graph database when processing
// Service/Deployment events that need to find matching Pods.
//
// The index maintains two maps:
// - byResource: namespace -> kind -> uid -> labels (for updates/removals)
// - byLabel: namespace -> kind -> labelKey -> labelValue -> set of UIDs (for queries)
type LabelIndex struct {
	// Forward index: namespace -> kind -> uid -> labels
	byResource map[string]map[string]map[string]map[string]string

	// Reverse index: namespace -> kind -> labelKey -> labelValue -> set of UIDs
	byLabel map[string]map[string]map[string]map[string]map[string]bool

	mu sync.RWMutex

	// Statistics
	hits   int64
	misses int64
}

// NewLabelIndex creates a new label index
func NewLabelIndex() *LabelIndex {
	return &LabelIndex{
		byResource: make(map[string]map[string]map[string]map[string]string),
		byLabel:    make(map[string]map[string]map[string]map[string]map[string]bool),
	}
}

// Update adds or updates a resource's labels in the index.
// This should be called when processing CREATE or UPDATE events for indexable resources.
func (idx *LabelIndex) Update(namespace, kind, uid string, labels map[string]string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove old labels if resource exists (handles label changes)
	idx.removeUnsafe(namespace, kind, uid)

	// Skip if no labels
	if len(labels) == 0 {
		return
	}

	// Initialize namespace map if needed
	if idx.byResource[namespace] == nil {
		idx.byResource[namespace] = make(map[string]map[string]map[string]string)
	}
	if idx.byResource[namespace][kind] == nil {
		idx.byResource[namespace][kind] = make(map[string]map[string]string)
	}

	// Store labels by resource (make a copy to avoid external mutations)
	labelsCopy := make(map[string]string, len(labels))
	for k, v := range labels {
		labelsCopy[k] = v
	}
	idx.byResource[namespace][kind][uid] = labelsCopy

	// Build reverse index (label -> UIDs)
	if idx.byLabel[namespace] == nil {
		idx.byLabel[namespace] = make(map[string]map[string]map[string]map[string]bool)
	}
	if idx.byLabel[namespace][kind] == nil {
		idx.byLabel[namespace][kind] = make(map[string]map[string]map[string]bool)
	}

	for key, value := range labels {
		if idx.byLabel[namespace][kind][key] == nil {
			idx.byLabel[namespace][kind][key] = make(map[string]map[string]bool)
		}
		if idx.byLabel[namespace][kind][key][value] == nil {
			idx.byLabel[namespace][kind][key][value] = make(map[string]bool)
		}
		idx.byLabel[namespace][kind][key][value][uid] = true
	}
}

// Remove removes a resource from the index.
// This should be called when processing DELETE events.
func (idx *LabelIndex) Remove(namespace, kind, uid string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeUnsafe(namespace, kind, uid)
}

// removeUnsafe removes a resource without locking (caller must hold lock)
func (idx *LabelIndex) removeUnsafe(namespace, kind, uid string) {
	if idx.byResource[namespace] == nil || idx.byResource[namespace][kind] == nil {
		return
	}

	oldLabels, exists := idx.byResource[namespace][kind][uid]
	if !exists {
		return
	}

	// Remove from reverse index
	for key, value := range oldLabels {
		if idx.byLabel[namespace] != nil &&
			idx.byLabel[namespace][kind] != nil &&
			idx.byLabel[namespace][kind][key] != nil &&
			idx.byLabel[namespace][kind][key][value] != nil {
			delete(idx.byLabel[namespace][kind][key][value], uid)

			// Clean up empty maps
			if len(idx.byLabel[namespace][kind][key][value]) == 0 {
				delete(idx.byLabel[namespace][kind][key], value)
			}
			if len(idx.byLabel[namespace][kind][key]) == 0 {
				delete(idx.byLabel[namespace][kind], key)
			}
		}
	}

	// Remove from forward index
	delete(idx.byResource[namespace][kind], uid)

	// Clean up empty maps
	if len(idx.byResource[namespace][kind]) == 0 {
		delete(idx.byResource[namespace], kind)
	}
	if len(idx.byResource[namespace]) == 0 {
		delete(idx.byResource, namespace)
	}
}

// FindBySelector returns UIDs of resources matching ALL selector labels.
// Uses set intersection for efficient multi-label matching.
// Returns nil if no matches found or if selector is empty.
func (idx *LabelIndex) FindBySelector(namespace, kind string, selector map[string]string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(selector) == 0 {
		idx.misses++
		return nil
	}

	if idx.byLabel[namespace] == nil || idx.byLabel[namespace][kind] == nil {
		idx.misses++
		return nil
	}

	// Start with candidates from first selector label
	var candidates map[string]bool
	first := true

	for key, value := range selector {
		if idx.byLabel[namespace][kind][key] == nil ||
			idx.byLabel[namespace][kind][key][value] == nil {
			idx.misses++
			return nil // No matches for this label
		}

		matchingUIDs := idx.byLabel[namespace][kind][key][value]

		if first {
			// Initialize candidates with first label matches
			candidates = make(map[string]bool, len(matchingUIDs))
			for uid := range matchingUIDs {
				candidates[uid] = true
			}
			first = false
		} else {
			// Intersect with existing candidates
			for uid := range candidates {
				if !matchingUIDs[uid] {
					delete(candidates, uid)
				}
			}
		}

		if len(candidates) == 0 {
			idx.misses++
			return nil
		}
	}

	idx.hits++

	result := make([]string, 0, len(candidates))
	for uid := range candidates {
		result = append(result, uid)
	}
	return result
}

// Contains checks if a resource is in the index
func (idx *LabelIndex) Contains(namespace, kind, uid string) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.byResource[namespace] == nil || idx.byResource[namespace][kind] == nil {
		return false
	}
	_, exists := idx.byResource[namespace][kind][uid]
	return exists
}

// GetLabels returns the labels for a specific resource
func (idx *LabelIndex) GetLabels(namespace, kind, uid string) map[string]string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.byResource[namespace] == nil || idx.byResource[namespace][kind] == nil {
		return nil
	}
	labels, exists := idx.byResource[namespace][kind][uid]
	if !exists {
		return nil
	}

	// Return a copy to prevent external mutations
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[k] = v
	}
	return result
}

// GetStats returns index statistics: hits, misses, and resource counts
func (idx *LabelIndex) GetStats() (hits, misses int64, namespaces, resources int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	namespaces = len(idx.byResource)
	for _, kinds := range idx.byResource {
		for _, uids := range kinds {
			resources += len(uids)
		}
	}
	return idx.hits, idx.misses, namespaces, resources
}

// ResetStats resets the hit/miss counters
func (idx *LabelIndex) ResetStats() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.hits = 0
	idx.misses = 0
}

// Clear empties the index and resets statistics
func (idx *LabelIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.byResource = make(map[string]map[string]map[string]map[string]string)
	idx.byLabel = make(map[string]map[string]map[string]map[string]map[string]bool)
	idx.hits = 0
	idx.misses = 0
}

// Len returns the total number of indexed resources
func (idx *LabelIndex) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count := 0
	for _, kinds := range idx.byResource {
		for _, uids := range kinds {
			count += len(uids)
		}
	}
	return count
}

// HitRate returns the hit rate as a percentage (0-100)
func (idx *LabelIndex) HitRate() float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	total := idx.hits + idx.misses
	if total == 0 {
		return 0
	}
	return float64(idx.hits) / float64(total) * 100
}
