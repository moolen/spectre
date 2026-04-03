package analyzer

import (
	"encoding/json"
	"strings"
)

// ResourceData represents parsed Kubernetes resource data for status inference.
type ResourceData struct {
	object map[string]any
}

type resourceData = ResourceData

func newResourceData(data json.RawMessage) (*ResourceData, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return &ResourceData{object: obj}, nil
}

// ParseResourceData parses resource JSON data into a ResourceData structure.
func ParseResourceData(data json.RawMessage) (*ResourceData, error) {
	return newResourceData(data)
}

func (r *resourceData) status() map[string]any {
	return getMapValue(r.object, "status")
}

func (r *resourceData) spec() map[string]any {
	return getMapValue(r.object, "spec")
}

func (r *resourceData) metadata() map[string]any {
	return getMapValue(r.object, "metadata")
}

func (r *resourceData) isDeleting() bool {
	meta := r.metadata()
	if meta == nil {
		return false
	}
	return getStringValue(meta, "deletionTimestamp") != ""
}

func (r *resourceData) conditions() []condition {
	status := r.status()
	if status == nil {
		return nil
	}

	raw := getSliceValue(status, "conditions")
	conds := make([]condition, 0, len(raw))
	for _, item := range raw {
		condMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		conds = append(conds, condition{
			Type:    getStringValue(condMap, "type"),
			Status:  getStringValue(condMap, "status"),
			Reason:  getStringValue(condMap, "reason"),
			Message: getStringValue(condMap, "message"),
		})
	}
	return conds
}

func (r *resourceData) condition(name string) *condition {
	for _, cond := range r.conditions() {
		if strings.EqualFold(cond.Type, name) {
			c := cond
			return &c
		}
	}
	return nil
}

func (r *resourceData) specInt(_ string) int64 {
	return getIntValue(r.spec(), "replicas")
}

func (r *resourceData) statusInt(key string) int64 {
	return getIntValue(r.status(), key)
}

func (r *resourceData) statusString(key string) string { //nolint:unparam
	return getStringValue(r.status(), key)
}

type condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

func (c condition) isTrue() bool {
	return strings.EqualFold(c.Status, "True")
}

func (c condition) isFalse() bool {
	return strings.EqualFold(c.Status, "False")
}

func (c condition) isUnknown() bool {
	return strings.EqualFold(c.Status, "Unknown")
}

func (c condition) isErrorLike() bool {
	return containsErrorKeyword(c.Reason) || containsErrorKeyword(c.Message)
}

func getMapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	value, ok := m[key]
	if !ok {
		return nil
	}
	result, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return result
}

func getSliceValue(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	value, ok := m[key]
	if !ok {
		return nil
	}
	result, ok := value.([]any)
	if !ok {
		return nil
	}
	return result
}

func getStringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func getIntValue(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	value, ok := m[key]
	if !ok {
		return 0
	}
	return normalizeNumber(value)
}

func normalizeNumber(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}
