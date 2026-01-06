package analysis

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// ComputeJSONDiff calculates the differences between two JSON byte slices.
// Returns a slice of EventDiff representing the changes from old to new.
func ComputeJSONDiff(oldData, newData []byte) ([]EventDiff, error) {
	var oldObj, newObj map[string]any

	if len(oldData) > 0 {
		if err := json.Unmarshal(oldData, &oldObj); err != nil {
			return nil, err
		}
	}
	if len(newData) > 0 {
		if err := json.Unmarshal(newData, &newObj); err != nil {
			return nil, err
		}
	}

	diffs := diffMaps("", oldObj, newObj)

	// Sort diffs by path for consistent ordering
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	return diffs, nil
}

// ParseJSONToMap parses JSON bytes into a map for use in FullSnapshot field.
func ParseJSONToMap(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// diffMaps recursively computes differences between two maps.
func diffMaps(prefix string, old, new map[string]any) []EventDiff {
	var diffs []EventDiff

	// Track keys we've seen
	seen := make(map[string]bool)

	// Check for removed/changed keys in old
	for k, oldVal := range old {
		seen[k] = true
		path := joinPath(prefix, k)
		newVal, exists := new[k]

		if !exists {
			// Key removed
			diffs = append(diffs, EventDiff{
				Path:     path,
				OldValue: simplifyValue(oldVal),
				Op:       "remove",
			})
		} else if !deepEqual(oldVal, newVal) {
			// Value changed - check if we should recurse
			oldMap, oldIsMap := oldVal.(map[string]any)
			newMap, newIsMap := newVal.(map[string]any)

			if oldIsMap && newIsMap {
				// Recurse into nested objects
				diffs = append(diffs, diffMaps(path, oldMap, newMap)...)
			} else {
				// Different types or non-objects
				diffs = append(diffs, EventDiff{
					Path:     path,
					OldValue: simplifyValue(oldVal),
					NewValue: simplifyValue(newVal),
					Op:       "replace",
				})
			}
		}
	}

	// Check for added keys in new
	for k, newVal := range new {
		if !seen[k] {
			path := joinPath(prefix, k)
			diffs = append(diffs, EventDiff{
				Path:     path,
				NewValue: simplifyValue(newVal),
				Op:       "add",
			})
		}
	}

	return diffs
}

// joinPath concatenates path segments with dot notation.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// deepEqual checks if two values are deeply equal.
func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// simplifyValue converts complex values for diff output.
// For large arrays/objects, it may return a summary to keep diffs readable.
func simplifyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		// For large maps, summarize
		if len(val) > 10 {
			return map[string]any{
				"_type":  "object",
				"_keys":  len(val),
				"_value": val,
			}
		}
		return val
	case []any:
		// For large arrays, summarize
		if len(val) > 10 {
			return map[string]any{
				"_type":   "array",
				"_length": len(val),
				"_value":  val,
			}
		}
		return val
	default:
		return v
	}
}

// FilterNoisyPaths removes paths that are typically noisy and not useful for LLM analysis.
// This includes managed fields, resourceVersion, and other auto-generated fields.
func FilterNoisyPaths(diffs []EventDiff) []EventDiff {
	noisyPrefixes := []string{
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.generation",
		"metadata.uid",
		"metadata.creationTimestamp",
		"status.observedGeneration",
	}

	var filtered []EventDiff
	for _, d := range diffs {
		isNoisy := false
		for _, prefix := range noisyPrefixes {
			if strings.HasPrefix(d.Path, prefix) {
				isNoisy = true
				break
			}
		}
		if !isNoisy {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// ConvertEventsToDiffFormat converts a slice of events from legacy format (Data field)
// to diff format (Diff/FullSnapshot fields).
// The first event (chronologically oldest) gets FullSnapshot, subsequent events get Diff from previous.
// IMPORTANT: Input events are assumed to be in REVERSE chronological order (newest first),
// so we reverse them before processing.
func ConvertEventsToDiffFormat(events []ChangeEventInfo, filterNoisy bool) []ChangeEventInfo {
	if len(events) == 0 {
		return events
	}

	// Reverse events to get chronological order (oldest first)
	reversed := make([]ChangeEventInfo, len(events))
	for i, event := range events {
		reversed[len(events)-1-i] = event
	}

	result := make([]ChangeEventInfo, len(events))
	var prevData []byte

	for i, event := range reversed {
		result[i] = event
		result[i].Data = nil // Clear legacy field

		if i == 0 {
			// First event (chronologically oldest) gets full snapshot
			snapshot, err := ParseJSONToMap(event.Data)
			if err == nil && snapshot != nil {
				result[i].FullSnapshot = snapshot
			}
			prevData = event.Data
		} else {
			// Subsequent events get diff from previous
			diffs, err := ComputeJSONDiff(prevData, event.Data)
			if err == nil {
				if filterNoisy {
					diffs = FilterNoisyPaths(diffs)
				}
				result[i].Diff = diffs
			}
			prevData = event.Data
		}
	}

	// Reverse back to original order (newest first)
	final := make([]ChangeEventInfo, len(events))
	for i := range result {
		final[len(result)-1-i] = result[i]
	}

	return final
}

// ConvertSingleEventToDiff converts a single event to diff format given previous data.
// Returns the modified event with Diff field populated.
func ConvertSingleEventToDiff(event *ChangeEventInfo, prevData []byte, filterNoisy bool) {
	if event == nil {
		return
	}

	if len(prevData) == 0 {
		// No previous data, this is effectively a CREATE
		snapshot, err := ParseJSONToMap(event.Data)
		if err == nil && snapshot != nil {
			event.FullSnapshot = snapshot
		}
	} else {
		// Compute diff from previous
		diffs, err := ComputeJSONDiff(prevData, event.Data)
		if err == nil {
			if filterNoisy {
				diffs = FilterNoisyPaths(diffs)
			}
			event.Diff = diffs
		}
	}

	// Clear legacy field in diff format
	event.Data = nil
}
