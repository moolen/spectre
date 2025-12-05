package storage

import (
	"encoding/json"
	"strings"
)

const (
	resourceStatusReady       = "Ready"
	resourceStatusWarning     = "Warning"
	resourceStatusError       = "Error"
	resourceStatusTerminating = "Terminating"
	resourceStatusUnknown     = "Unknown"
)

// InferStatusFromResource inspects the resource payload and produces a best-effort status.
func InferStatusFromResource(kind string, data json.RawMessage, eventType string) string {
	if strings.EqualFold(eventType, "DELETE") {
		return resourceStatusTerminating
	}

	if len(data) == 0 {
		return inferStatusFromEventType(eventType)
	}

	obj, err := newResourceData(data)
	if err != nil {
		return inferStatusFromEventType(eventType)
	}

	if obj.isDeleting() {
		return resourceStatusTerminating
	}

	status := inferResourceSpecificStatus(strings.ToLower(kind), obj)
	if status != "" {
		return status
	}

	conditionStatus := inferStatusFromConditions(obj.conditions())
	if conditionStatus != "" {
		return conditionStatus
	}

	return inferStatusFromEventType(eventType)
}

func inferResourceSpecificStatus(kind string, obj *resourceData) string {
	switch kind {
	case "deployment":
		return inferDeploymentStatus(obj)
	case "statefulset":
		return inferStatefulSetStatus(obj)
	case "daemonset":
		return inferDaemonSetStatus(obj)
	case "replicaset":
		return inferReplicaSetStatus(obj)
	case "pod":
		return inferPodStatus(obj)
	case "persistentvolumeclaim":
		return inferPVCStatus(obj)
	case "node":
		return inferNodeStatus(obj)
	case "job":
		return inferJobStatus(obj)
	case "service", "configmap", "secret":
		return resourceStatusReady
	default:
		return ""
	}
}

func inferDeploymentStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")
	unavailable := obj.statusInt("unavailableReplicas")

	if desired > 0 && ready >= desired && available >= desired && unavailable == 0 {
		return resourceStatusReady
	}

	if cond := obj.condition("Available"); cond != nil && cond.isFalse() {
		if cond.isErrorLike() {
			return resourceStatusError
		}
		return resourceStatusWarning
	}

	if unavailable > 0 {
		return resourceStatusWarning
	}

	if cond := obj.condition("Progressing"); cond != nil && cond.isTrue() {
		return resourceStatusWarning
	}

	if desired > 0 && available < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferStatefulSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := firstNonZero(obj.specInt("replicas"), obj.statusInt("replicas"))
	ready := obj.statusInt("readyReplicas")
	current := obj.statusInt("currentReplicas")

	if desired > 0 && ready >= desired {
		return resourceStatusReady
	}

	if desired > 0 && current < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferDaemonSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	// TODO: does not work as expected. see demo data.
	desired := obj.statusInt("desiredNumberScheduled")
	ready := obj.statusInt("numberReady")
	unavailable := obj.statusInt("numberUnavailable")
	misscheduled := obj.statusInt("numberMisscheduled")

	if desired > 0 && ready >= desired && unavailable == 0 && misscheduled == 0 {
		return resourceStatusReady
	}

	if unavailable > 0 || misscheduled > 0 {
		return resourceStatusWarning
	}

	return ""
}

func inferReplicaSetStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	desired := obj.specInt("replicas")
	ready := obj.statusInt("readyReplicas")
	available := obj.statusInt("availableReplicas")

	if obj.specInt("replicas") == obj.statusInt("replicas") {
		return resourceStatusReady
	}

	if desired > 0 && ready >= desired && available >= desired {
		return resourceStatusReady
	}

	if desired > 0 && available < desired {
		return resourceStatusWarning
	}

	return ""
}

func inferPodStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	switch strings.ToLower(obj.statusString("phase")) {
	case "running":
		if cond := obj.condition("Ready"); cond != nil && cond.isTrue() {
			return resourceStatusReady
		}
		return resourceStatusWarning
	case "pending":
		return resourceStatusWarning
	case "succeeded":
		return resourceStatusReady
	case "failed":
		return resourceStatusError
	case "unknown":
		return resourceStatusWarning
	default:
		return ""
	}
}

func inferPVCStatus(obj *resourceData) string {
	phase := strings.ToLower(obj.statusString("phase"))
	switch phase {
	case "bound":
		return resourceStatusReady
	case "pending":
		return resourceStatusWarning
	case "lost":
		return resourceStatusError
	default:
		return ""
	}
}

func inferNodeStatus(obj *resourceData) string {
	readyCond := obj.condition("Ready")
	if readyCond == nil {
		return ""
	}

	if readyCond.isFalse() || readyCond.isUnknown() {
		return resourceStatusError
	}

	for _, t := range []string{"MemoryPressure", "DiskPressure", "PIDPressure", "NetworkUnavailable"} {
		if cond := obj.condition(t); cond != nil && cond.isTrue() {
			return resourceStatusWarning
		}
	}

	return resourceStatusReady
}

func inferJobStatus(obj *resourceData) string {
	status := obj.status()
	if status == nil {
		return ""
	}

	if cond := obj.condition("Complete"); cond != nil && cond.isTrue() {
		return resourceStatusReady
	}
	if cond := obj.condition("Failed"); cond != nil && cond.isTrue() {
		return resourceStatusError
	}

	if obj.statusInt("succeeded") > 0 {
		return resourceStatusReady
	}
	if obj.statusInt("failed") > 0 {
		return resourceStatusError
	}
	if obj.statusInt("active") > 0 {
		return resourceStatusWarning
	}

	return ""
}

func inferStatusFromConditions(conditions []condition) string {
	if len(conditions) == 0 {
		return ""
	}

	if cond := findCondition(conditions, "Ready"); cond != nil {
		if cond.isTrue() {
			return resourceStatusReady
		}
		if cond.isFalse() {
			if cond.isErrorLike() {
				return resourceStatusError
			}
			return resourceStatusWarning
		}
		if cond.isUnknown() {
			return resourceStatusWarning
		}
	}

	if cond := findCondition(conditions, "Healthy"); cond != nil {
		if cond.isTrue() {
			return resourceStatusReady
		}
		if cond.isFalse() {
			if cond.isErrorLike() {
				return resourceStatusError
			}
			return resourceStatusWarning
		}
	}

	for _, name := range []string{"Stalled", "Degraded", "Failing", "Failed"} {
		if cond := findCondition(conditions, name); cond != nil && cond.isTrue() {
			if name == "Degraded" {
				return resourceStatusWarning
			}
			return resourceStatusError
		}
	}

	for _, name := range []string{"Reconciling", "Progressing"} {
		if cond := findCondition(conditions, name); cond != nil && cond.isTrue() {
			return resourceStatusWarning
		}
	}

	return ""
}

func inferStatusFromEventType(eventType string) string {
	typeUpper := strings.ToUpper(eventType)
	switch typeUpper {
	case "CREATE", "UPDATE":
		return resourceStatusReady
	case "DELETE":
		return resourceStatusTerminating
	default:
		return resourceStatusUnknown
	}
}

type resourceData struct {
	object map[string]any
}

func newResourceData(data json.RawMessage) (*resourceData, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return &resourceData{object: obj}, nil
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
	var conds []condition
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

func (r *resourceData) specInt(key string) int64 {
	return getIntValue(r.spec(), key)
}

func (r *resourceData) statusInt(key string) int64 {
	return getIntValue(r.status(), key)
}

func (r *resourceData) statusString(key string) string {
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

func findCondition(conditions []condition, condType string) *condition {
	for _, cond := range conditions {
		if strings.EqualFold(cond.Type, condType) {
			c := cond
			return &c
		}
	}
	return nil
}

func containsErrorKeyword(text string) bool {
	textLower := strings.ToLower(text)
	if textLower == "" {
		return false
	}

	keywords := []string{"error", "fail", "invalid", "crash", "timeout", "stalled"}
	for _, keyword := range keywords {
		if strings.Contains(textLower, keyword) {
			return true
		}
	}
	return false
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
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
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
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
