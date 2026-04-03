package analysis

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
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
func diffMaps(prefix string, old, newMap map[string]any) []EventDiff {
	var diffs []EventDiff

	// Track keys we've seen
	seen := make(map[string]bool)

	// Check for removed/changed keys in old
	for k, oldVal := range old {
		seen[k] = true
		path := joinPath(prefix, k)
		newVal, exists := newMap[k]

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
			oldArr, oldIsArr := oldVal.([]any)
			newArr, newIsArr := newVal.([]any)

			if oldIsMap && newIsMap {
				// Recurse into nested objects
				diffs = append(diffs, diffMaps(path, oldMap, newMap)...)
			} else if oldIsArr && newIsArr {
				// Recurse into arrays with element matching
				diffs = append(diffs, diffArrays(path, oldArr, newArr)...)
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
	for k, newVal := range newMap {
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

// diffArrays computes differences between two arrays.
// It attempts to match elements by a key field (e.g., "name" for containers)
// and produces element-level diffs.
func diffArrays(prefix string, old, newArr []any) []EventDiff {
	var diffs []EventDiff

	// Try to identify elements by common key fields
	keyFields := []string{"name", "containerPort", "port", "type", "key"}

	// Find a suitable key field
	keyField := ""
	for _, field := range keyFields {
		if arrayHasKeyField(old, field) && arrayHasKeyField(newArr, field) {
			keyField = field
			break
		}
	}

	if keyField != "" {
		// Match elements by key field
		oldByKey := indexArrayByKey(old, keyField)
		newByKey := indexArrayByKey(newArr, keyField)

		// Track seen keys
		seen := make(map[string]bool)

		// Check for removed/changed elements
		for key, oldElem := range oldByKey {
			seen[key] = true
			elemPath := fmt.Sprintf("%s[%s=%s]", prefix, keyField, key)

			if newElem, exists := newByKey[key]; exists {
				// Element exists in both - diff them
				oldMap, oldIsMap := oldElem.(map[string]any)
				newMap, newIsMap := newElem.(map[string]any)

				if oldIsMap && newIsMap {
					elemDiffs := diffMaps(elemPath, oldMap, newMap)
					diffs = append(diffs, elemDiffs...)
				} else if !deepEqual(oldElem, newElem) {
					diffs = append(diffs, EventDiff{
						Path:     elemPath,
						OldValue: simplifyValue(oldElem),
						NewValue: simplifyValue(newElem),
						Op:       "replace",
					})
				}
			} else {
				// Element removed
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					OldValue: simplifyValue(oldElem),
					Op:       "remove",
				})
			}
		}

		// Check for added elements
		for key, newElem := range newByKey {
			if !seen[key] {
				elemPath := fmt.Sprintf("%s[%s=%s]", prefix, keyField, key)
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					NewValue: simplifyValue(newElem),
					Op:       "add",
				})
			}
		}
	} else {
		// No key field found - diff by index
		maxLen := len(old)
		if len(newArr) > maxLen {
			maxLen = len(newArr)
		}

		for i := 0; i < maxLen; i++ {
			elemPath := fmt.Sprintf("%s[%d]", prefix, i)

			if i >= len(old) {
				// Element added
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					NewValue: simplifyValue(newArr[i]),
					Op:       "add",
				})
			} else if i >= len(newArr) {
				// Element removed
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					OldValue: simplifyValue(old[i]),
					Op:       "remove",
				})
			} else if !deepEqual(old[i], newArr[i]) {
				// Element changed
				oldMap, oldIsMap := old[i].(map[string]any)
				newMap, newIsMap := newArr[i].(map[string]any)

				if oldIsMap && newIsMap {
					diffs = append(diffs, diffMaps(elemPath, oldMap, newMap)...)
				} else {
					diffs = append(diffs, EventDiff{
						Path:     elemPath,
						OldValue: simplifyValue(old[i]),
						NewValue: simplifyValue(newArr[i]),
						Op:       "replace",
					})
				}
			}
		}
	}

	return diffs
}

// arrayHasKeyField checks if all map elements in the array have the given key field.
func arrayHasKeyField(arr []any, field string) bool {
	if len(arr) == 0 {
		return false
	}
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			if _, hasKey := m[field]; !hasKey {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// indexArrayByKey creates a map from key field values to array elements.
func indexArrayByKey(arr []any, field string) map[string]any {
	result := make(map[string]any)
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			if key, ok := m[field]; ok {
				keyStr := fmt.Sprintf("%v", key)
				result[keyStr] = elem
			}
		}
	}
	return result
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
