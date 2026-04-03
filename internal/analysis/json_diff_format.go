package analysis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FormatUnifiedDiff converts a slice of EventDiff to a git-style unified diff string.
func FormatUnifiedDiff(diffs []EventDiff) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder
	grouped := groupDiffsBySection(diffs)

	sections := make([]string, 0, len(grouped))
	for section := range grouped {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	for _, section := range sections {
		sectionDiffs := grouped[section]
		if section == "status.conditions" && len(sectionDiffs) == 1 && sectionDiffs[0].Op == "replace" {
			sb.WriteString(formatConditionsDiff(sectionDiffs[0]))
			continue
		}

		sb.WriteString("@@ ")
		sb.WriteString(section)
		sb.WriteString(" @@\n")

		for _, diff := range sectionDiffs {
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

func groupDiffsBySection(diffs []EventDiff) map[string][]EventDiff {
	grouped := make(map[string][]EventDiff)
	for _, diff := range diffs {
		section := getSectionForPath(diff.Path)
		grouped[section] = append(grouped[section], diff)
	}
	return grouped
}

func getSectionForPath(path string) string {
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

	if idx := strings.Index(path, "."); idx > 0 {
		return path[:idx]
	}
	return path
}

func formatConditionsDiff(diff EventDiff) string {
	var sb strings.Builder

	oldConditions := extractConditionsArray(diff.OldValue)
	newConditions := extractConditionsArray(diff.NewValue)
	oldByType := indexConditionsByType(oldConditions)
	newByType := indexConditionsByType(newConditions)

	allTypes := make(map[string]bool)
	for t := range oldByType {
		allTypes[t] = true
	}
	for t := range newByType {
		allTypes[t] = true
	}

	types := make([]string, 0, len(allTypes))
	for t := range allTypes {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, condType := range types {
		oldCond := oldByType[condType]
		newCond := newByType[condType]

		if oldCond == nil && newCond != nil {
			sb.WriteString("@@ status.conditions[type=")
			sb.WriteString(condType)
			sb.WriteString("] @@\n")
			writeConditionFields(&sb, "+  ", newCond)
		} else if oldCond != nil && newCond == nil {
			sb.WriteString("@@ status.conditions[type=")
			sb.WriteString(condType)
			sb.WriteString("] @@\n")
			writeConditionFields(&sb, "-  ", oldCond)
		} else if oldCond != nil && newCond != nil {
			changes := getConditionChanges(oldCond, newCond)
			if len(changes) == 0 {
				continue
			}

			sb.WriteString("@@ status.conditions[type=")
			sb.WriteString(condType)
			sb.WriteString("] @@\n")
			for _, change := range changes {
				sb.WriteString(change)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func extractConditionsArray(v any) []map[string]any {
	if v == nil {
		return nil
	}

	if wrapped, ok := v.(map[string]any); ok && wrapped["_type"] == "array" {
		if arr, ok := wrapped["_value"].([]any); ok {
			return convertToConditionMaps(arr)
		}
	}

	if arr, ok := v.([]any); ok {
		return convertToConditionMaps(arr)
	}

	return nil
}

func convertToConditionMaps(arr []any) []map[string]any {
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func indexConditionsByType(conditions []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, cond := range conditions {
		if condType, ok := cond["type"].(string); ok {
			result[condType] = cond
		}
	}
	return result
}

func writeConditionFields(sb *strings.Builder, prefix string, cond map[string]any) {
	orderedFields := []string{"status", "reason", "message", "lastTransitionTime"}
	for _, field := range orderedFields {
		val, ok := cond[field]
		if !ok {
			continue
		}
		sb.WriteString(prefix)
		sb.WriteString(field)
		sb.WriteString(": ")
		sb.WriteString(formatValue(val))
		sb.WriteString("\n")
	}
}

func getConditionChanges(old, newCond map[string]any) []string {
	var changes []string
	fields := []string{"status", "reason", "message", "lastTransitionTime"}

	for _, field := range fields {
		oldVal, oldOk := old[field]
		newVal, newOk := newCond[field]

		if !oldOk && newOk {
			changes = append(changes, "+  "+field+": "+formatValue(newVal))
		} else if oldOk && !newOk {
			changes = append(changes, "-  "+field+": "+formatValue(oldVal))
		} else if oldOk && newOk && !deepEqual(oldVal, newVal) {
			changes = append(changes,
				"-  "+field+": "+formatValue(oldVal),
				"+  "+field+": "+formatValue(newVal))
		}
	}

	return changes
}

// FormatValueDiff creates a git-style unified diff string for a single value change.
func FormatValueDiff(path string, oldVal, newVal any) string {
	var sb strings.Builder
	sb.WriteString("@@ ")
	sb.WriteString(path)
	sb.WriteString(" @@\n")

	if oldVal != nil {
		sb.WriteString("-  ")
		sb.WriteString(formatValue(oldVal))
		sb.WriteString("\n")
	}
	if newVal != nil {
		sb.WriteString("+  ")
		sb.WriteString(formatValue(newVal))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatValue(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		if len(val) > 100 {
			return "\"" + val[:97] + "...\""
		}
		return "\"" + val + "\""
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64, int, int64:
		b, _ := json.Marshal(val)
		return string(b)
	case map[string]any:
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
		return fmt.Sprintf("[%d items]", len(val))
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return "?"
		}
		return string(b)
	}
}
