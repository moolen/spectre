package analysis

import (
	"encoding/json"
	"fmt"
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

// diffArrays computes differences between two arrays.
// It attempts to match elements by a key field (e.g., "name" for containers)
// and produces element-level diffs.
func diffArrays(prefix string, old, new []any) []EventDiff {
	var diffs []EventDiff

	// Try to identify elements by common key fields
	keyFields := []string{"name", "containerPort", "port", "type", "key"}

	// Find a suitable key field
	keyField := ""
	for _, field := range keyFields {
		if arrayHasKeyField(old, field) && arrayHasKeyField(new, field) {
			keyField = field
			break
		}
	}

	if keyField != "" {
		// Match elements by key field
		oldByKey := indexArrayByKey(old, keyField)
		newByKey := indexArrayByKey(new, keyField)

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
		if len(new) > maxLen {
			maxLen = len(new)
		}

		for i := 0; i < maxLen; i++ {
			elemPath := fmt.Sprintf("%s[%d]", prefix, i)

			if i >= len(old) {
				// Element added
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					NewValue: simplifyValue(new[i]),
					Op:       "add",
				})
			} else if i >= len(new) {
				// Element removed
				diffs = append(diffs, EventDiff{
					Path:     elemPath,
					OldValue: simplifyValue(old[i]),
					Op:       "remove",
				})
			} else if !deepEqual(old[i], new[i]) {
				// Element changed
				oldMap, oldIsMap := old[i].(map[string]any)
				newMap, newIsMap := new[i].(map[string]any)

				if oldIsMap && newIsMap {
					diffs = append(diffs, diffMaps(elemPath, oldMap, newMap)...)
				} else {
					diffs = append(diffs, EventDiff{
						Path:     elemPath,
						OldValue: simplifyValue(old[i]),
						NewValue: simplifyValue(new[i]),
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

// FilterSpecOnly filters diffs to only include spec changes.
// This excludes all status fields and noisy metadata fields,
// keeping only meaningful configuration changes.
func FilterSpecOnly(diffs []EventDiff) []EventDiff {
	excludePrefixes := []string{
		"status",                       // All status fields
		"metadata.managedFields",       // Auto-managed
		"metadata.resourceVersion",     // Auto-incremented
		"metadata.generation",          // Auto-incremented
		"metadata.uid",                 // Immutable
		"metadata.creationTimestamp",   // Immutable
	}

	var filtered []EventDiff
	for _, d := range diffs {
		exclude := false
		for _, prefix := range excludePrefixes {
			if d.Path == prefix || strings.HasPrefix(d.Path, prefix+".") {
				exclude = true
				break
			}
		}
		if !exclude {
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
		// Keep Data field for anomaly detection (needed to check container statuses)
		// Even though we're adding Diff/FullSnapshot, Data is still useful for state checks
		// result[i].Data = nil // Don't clear - needed for anomaly detection

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

// FormatUnifiedDiff converts a slice of EventDiff to a git-style unified diff string.
// This provides a compact, human-readable representation of all changes.
// For arrays like status.conditions, it matches elements by key fields (e.g., "type")
// and shows per-element changes.
func FormatUnifiedDiff(diffs []EventDiff) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// Group diffs by top-level path for better organization
	// Also handle array element matching for conditions
	grouped := groupDiffsBySection(diffs)

	// Sort section names for consistent output
	sections := make([]string, 0, len(grouped))
	for section := range grouped {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	for _, section := range sections {
		sectionDiffs := grouped[section]

		// Check if this is a conditions array that needs special handling
		if section == "status.conditions" && len(sectionDiffs) == 1 && sectionDiffs[0].Op == "replace" {
			// Format conditions with element-level diffs
			sb.WriteString(formatConditionsDiff(sectionDiffs[0]))
			continue
		}

		// Regular section header
		sb.WriteString("@@ ")
		sb.WriteString(section)
		sb.WriteString(" @@\n")

		for _, diff := range sectionDiffs {
			// Get the field name (last part of path after the section prefix)
			fieldName := diff.Path
			if strings.HasPrefix(diff.Path, section+".") {
				fieldName = diff.Path[len(section)+1:]
			}

			switch diff.Op {
			case "remove":
				sb.WriteString("-  ")
				sb.WriteString(fieldName)
				sb.WriteString(": ")
				sb.WriteString(formatValue(diff.OldValue))
				sb.WriteString("\n")
			case "add":
				sb.WriteString("+  ")
				sb.WriteString(fieldName)
				sb.WriteString(": ")
				sb.WriteString(formatValue(diff.NewValue))
				sb.WriteString("\n")
			case "replace":
				sb.WriteString("-  ")
				sb.WriteString(fieldName)
				sb.WriteString(": ")
				sb.WriteString(formatValue(diff.OldValue))
				sb.WriteString("\n")
				sb.WriteString("+  ")
				sb.WriteString(fieldName)
				sb.WriteString(": ")
				sb.WriteString(formatValue(diff.NewValue))
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// groupDiffsBySection groups diffs by their top-level or second-level path.
// For example, "spec.replicas" goes to "spec", "status.conditions" stays as "status.conditions".
func groupDiffsBySection(diffs []EventDiff) map[string][]EventDiff {
	grouped := make(map[string][]EventDiff)

	for _, diff := range diffs {
		section := getSectionForPath(diff.Path)
		grouped[section] = append(grouped[section], diff)
	}

	return grouped
}

// getSectionForPath determines the section name for a given path.
// Special paths like "status.conditions" are kept as-is.
func getSectionForPath(path string) string {
	// Special cases that should be kept together
	specialPaths := []string{
		"status.conditions",
		"status.containerStatuses",
		"status.initContainerStatuses",
		"metadata.labels",
		"metadata.annotations",
	}

	for _, special := range specialPaths {
		if path == special || strings.HasPrefix(path, special+".") || strings.HasPrefix(path, special+"[") {
			return special
		}
	}

	// Otherwise use first path segment
	if idx := strings.Index(path, "."); idx > 0 {
		return path[:idx]
	}
	return path
}

// formatConditionsDiff formats status.conditions changes by matching conditions by type.
func formatConditionsDiff(diff EventDiff) string {
	var sb strings.Builder

	oldConditions := extractConditionsArray(diff.OldValue)
	newConditions := extractConditionsArray(diff.NewValue)

	// Index conditions by type
	oldByType := indexConditionsByType(oldConditions)
	newByType := indexConditionsByType(newConditions)

	// Collect all condition types
	allTypes := make(map[string]bool)
	for t := range oldByType {
		allTypes[t] = true
	}
	for t := range newByType {
		allTypes[t] = true
	}

	// Sort types for consistent output
	types := make([]string, 0, len(allTypes))
	for t := range allTypes {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, condType := range types {
		oldCond := oldByType[condType]
		newCond := newByType[condType]

		if oldCond == nil && newCond != nil {
			// Condition added
			sb.WriteString("@@ status.conditions[type=")
			sb.WriteString(condType)
			sb.WriteString("] @@\n")
			writeConditionFields(&sb, "+  ", newCond)
		} else if oldCond != nil && newCond == nil {
			// Condition removed
			sb.WriteString("@@ status.conditions[type=")
			sb.WriteString(condType)
			sb.WriteString("] @@\n")
			writeConditionFields(&sb, "-  ", oldCond)
		} else if oldCond != nil && newCond != nil {
			// Check for changes
			changes := getConditionChanges(oldCond, newCond)
			if len(changes) > 0 {
				sb.WriteString("@@ status.conditions[type=")
				sb.WriteString(condType)
				sb.WriteString("] @@\n")
				for _, change := range changes {
					sb.WriteString(change)
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

// extractConditionsArray extracts conditions from a diff value.
func extractConditionsArray(v any) []map[string]any {
	if v == nil {
		return nil
	}

	// Handle wrapped array format from simplifyValue
	if wrapped, ok := v.(map[string]any); ok {
		if wrapped["_type"] == "array" {
			if arr, ok := wrapped["_value"].([]any); ok {
				return convertToConditionMaps(arr)
			}
		}
	}

	// Handle direct array
	if arr, ok := v.([]any); ok {
		return convertToConditionMaps(arr)
	}

	return nil
}

// convertToConditionMaps converts []any to []map[string]any.
func convertToConditionMaps(arr []any) []map[string]any {
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

// indexConditionsByType creates a map of conditions indexed by their "type" field.
func indexConditionsByType(conditions []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, cond := range conditions {
		if condType, ok := cond["type"].(string); ok {
			result[condType] = cond
		}
	}
	return result
}

// writeConditionFields writes condition fields with a prefix.
func writeConditionFields(sb *strings.Builder, prefix string, cond map[string]any) {
	// Order fields for readability
	orderedFields := []string{"status", "reason", "message", "lastTransitionTime"}

	for _, field := range orderedFields {
		if val, ok := cond[field]; ok {
			sb.WriteString(prefix)
			sb.WriteString(field)
			sb.WriteString(": ")
			sb.WriteString(formatValue(val))
			sb.WriteString("\n")
		}
	}
}

// getConditionChanges returns the diff lines for changed fields between two conditions.
func getConditionChanges(old, new map[string]any) []string {
	var changes []string

	// Fields to compare (in order)
	fields := []string{"status", "reason", "message", "lastTransitionTime"}

	for _, field := range fields {
		oldVal, oldOk := old[field]
		newVal, newOk := new[field]

		if !oldOk && newOk {
			changes = append(changes, "+  "+field+": "+formatValue(newVal))
		} else if oldOk && !newOk {
			changes = append(changes, "-  "+field+": "+formatValue(oldVal))
		} else if oldOk && newOk && !deepEqual(oldVal, newVal) {
			changes = append(changes, "-  "+field+": "+formatValue(oldVal))
			changes = append(changes, "+  "+field+": "+formatValue(newVal))
		}
	}

	return changes
}

// FormatValueDiff creates a git-style unified diff string for a single value change.
// This is useful for anomaly details where we have old/new values to display.
// Example output:
//
//	@@ spec.containers[0].image @@
//	-  "nginx:1.19"
//	+  "nginx:1.20"
func FormatValueDiff(path string, oldVal, newVal any) string {
	var sb strings.Builder

	// Write section header
	sb.WriteString("@@ ")
	sb.WriteString(path)
	sb.WriteString(" @@\n")

	// Write old value (if present)
	if oldVal != nil {
		sb.WriteString("-  ")
		sb.WriteString(formatValue(oldVal))
		sb.WriteString("\n")
	}

	// Write new value (if present)
	if newVal != nil {
		sb.WriteString("+  ")
		sb.WriteString(formatValue(newVal))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatValue converts a value to a string representation for the diff output.
func formatValue(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		// Truncate long strings
		if len(val) > 100 {
			return "\"" + val[:97] + "...\""
		}
		return "\"" + val + "\""
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		// Use JSON encoding for proper number formatting
		b, _ := json.Marshal(val)
		return string(b)
	case int:
		b, _ := json.Marshal(val)
		return string(b)
	case int64:
		b, _ := json.Marshal(val)
		return string(b)
	case map[string]any:
		// For objects, show a compact representation
		b, err := json.Marshal(val)
		if err != nil {
			return "{...}"
		}
		s := string(b)
		if len(s) > 100 {
			return s[:97] + "..."
		}
		return s
	case []any:
		// For arrays, show length
		return fmt.Sprintf("[%d items]", len(val))
	default:
		// Use JSON encoding as fallback
		b, err := json.Marshal(val)
		if err != nil {
			return "?"
		}
		return string(b)
	}
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
