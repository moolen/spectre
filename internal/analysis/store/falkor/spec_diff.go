package falkor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type diffEntry struct {
	Path     string
	OldValue any
	NewValue any
	Op       string
}

func computeJSONDiff(oldData, newData []byte) ([]diffEntry, error) {
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
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	return diffs, nil
}

func diffMaps(prefix string, oldMap, newMap map[string]any) []diffEntry {
	var diffs []diffEntry
	seen := make(map[string]bool)

	for key, oldVal := range oldMap {
		seen[key] = true
		path := joinPath(prefix, key)
		newVal, exists := newMap[key]

		if !exists {
			diffs = append(diffs, diffEntry{
				Path:     path,
				OldValue: simplifyValue(oldVal),
				Op:       "remove",
			})
			continue
		}
		if deepEqual(oldVal, newVal) {
			continue
		}

		oldNestedMap, oldIsMap := oldVal.(map[string]any)
		newNestedMap, newIsMap := newVal.(map[string]any)
		oldArr, oldIsArr := oldVal.([]any)
		newArr, newIsArr := newVal.([]any)

		switch {
		case oldIsMap && newIsMap:
			diffs = append(diffs, diffMaps(path, oldNestedMap, newNestedMap)...)
		case oldIsArr && newIsArr:
			diffs = append(diffs, diffArrays(path, oldArr, newArr)...)
		default:
			diffs = append(diffs, diffEntry{
				Path:     path,
				OldValue: simplifyValue(oldVal),
				NewValue: simplifyValue(newVal),
				Op:       "replace",
			})
		}
	}

	for key, newVal := range newMap {
		if seen[key] {
			continue
		}
		diffs = append(diffs, diffEntry{
			Path:     joinPath(prefix, key),
			NewValue: simplifyValue(newVal),
			Op:       "add",
		})
	}

	return diffs
}

func diffArrays(prefix string, oldArr, newArr []any) []diffEntry {
	var diffs []diffEntry

	keyFields := []string{"name", "containerPort", "port", "type", "key"}
	keyField := ""
	for _, field := range keyFields {
		if arrayHasKeyField(oldArr, field) && arrayHasKeyField(newArr, field) {
			keyField = field
			break
		}
	}

	if keyField != "" {
		oldByKey := indexArrayByKey(oldArr, keyField)
		newByKey := indexArrayByKey(newArr, keyField)
		seen := make(map[string]bool)

		for key, oldElem := range oldByKey {
			seen[key] = true
			elemPath := fmt.Sprintf("%s[%s=%s]", prefix, keyField, key)

			newElem, exists := newByKey[key]
			if !exists {
				diffs = append(diffs, diffEntry{
					Path:     elemPath,
					OldValue: simplifyValue(oldElem),
					Op:       "remove",
				})
				continue
			}

			oldMap, oldIsMap := oldElem.(map[string]any)
			newMap, newIsMap := newElem.(map[string]any)
			switch {
			case oldIsMap && newIsMap:
				diffs = append(diffs, diffMaps(elemPath, oldMap, newMap)...)
			case !deepEqual(oldElem, newElem):
				diffs = append(diffs, diffEntry{
					Path:     elemPath,
					OldValue: simplifyValue(oldElem),
					NewValue: simplifyValue(newElem),
					Op:       "replace",
				})
			}
		}

		for key, newElem := range newByKey {
			if seen[key] {
				continue
			}
			diffs = append(diffs, diffEntry{
				Path:     fmt.Sprintf("%s[%s=%s]", prefix, keyField, key),
				NewValue: simplifyValue(newElem),
				Op:       "add",
			})
		}

		return diffs
	}

	maxLen := len(oldArr)
	if len(newArr) > maxLen {
		maxLen = len(newArr)
	}

	for i := 0; i < maxLen; i++ {
		elemPath := fmt.Sprintf("%s[%d]", prefix, i)
		switch {
		case i >= len(oldArr):
			diffs = append(diffs, diffEntry{
				Path:     elemPath,
				NewValue: simplifyValue(newArr[i]),
				Op:       "add",
			})
		case i >= len(newArr):
			diffs = append(diffs, diffEntry{
				Path:     elemPath,
				OldValue: simplifyValue(oldArr[i]),
				Op:       "remove",
			})
		case !deepEqual(oldArr[i], newArr[i]):
			oldMap, oldIsMap := oldArr[i].(map[string]any)
			newMap, newIsMap := newArr[i].(map[string]any)
			switch {
			case oldIsMap && newIsMap:
				diffs = append(diffs, diffMaps(elemPath, oldMap, newMap)...)
			default:
				diffs = append(diffs, diffEntry{
					Path:     elemPath,
					OldValue: simplifyValue(oldArr[i]),
					NewValue: simplifyValue(newArr[i]),
					Op:       "replace",
				})
			}
		}
	}

	return diffs
}

func arrayHasKeyField(arr []any, field string) bool {
	if len(arr) == 0 {
		return false
	}
	for _, elem := range arr {
		m, ok := elem.(map[string]any)
		if !ok {
			return false
		}
		if _, hasKey := m[field]; !hasKey {
			return false
		}
	}
	return true
}

func indexArrayByKey(arr []any, field string) map[string]any {
	result := make(map[string]any)
	for _, elem := range arr {
		m, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		key, ok := m[field]
		if !ok {
			continue
		}
		result[fmt.Sprintf("%v", key)] = elem
	}
	return result
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func simplifyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		if len(val) > 10 {
			return map[string]any{
				"_type":  "object",
				"_keys":  len(val),
				"_value": val,
			}
		}
		return val
	case []any:
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

func filterSpecOnly(diffs []diffEntry) []diffEntry {
	excludePrefixes := []string{
		"status",
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.generation",
		"metadata.uid",
		"metadata.creationTimestamp",
	}

	var filtered []diffEntry
	for _, diff := range diffs {
		exclude := false
		for _, prefix := range excludePrefixes {
			if diff.Path == prefix || strings.HasPrefix(diff.Path, prefix+".") {
				exclude = true
				break
			}
		}
		if !exclude {
			filtered = append(filtered, diff)
		}
	}
	return filtered
}

func formatUnifiedDiff(diffs []diffEntry) string {
	if len(diffs) == 0 {
		return ""
	}

	grouped := make(map[string][]diffEntry)
	for _, diff := range diffs {
		section := diff.Path
		if idx := strings.Index(section, "."); idx > 0 {
			section = section[:idx]
		}
		grouped[section] = append(grouped[section], diff)
	}

	sections := make([]string, 0, len(grouped))
	for section := range grouped {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	var sb strings.Builder
	for _, section := range sections {
		sb.WriteString("@@ ")
		sb.WriteString(section)
		sb.WriteString(" @@\n")

		for _, diff := range grouped[section] {
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

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}
