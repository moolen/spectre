package embeddedstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

type resourceKey struct {
	namespace string
	kind      string
	name      string
}

type orderedResourceKey struct {
	kind      string
	namespace string
	name      string
	uid       string
}

type resourceVersion struct {
	event       models.Event
	timestamp   int64
	eventType   models.EventType
	identity    graph.ResourceIdentity
	data        []byte
	object      map[string]any
	changeEvent analysisstore.ChangeEventInfo
}

type resourceRecord struct {
	uid      string
	versions []resourceVersion
}

func (r *resourceRecord) latestVersion() *resourceVersion {
	if len(r.versions) == 0 {
		return nil
	}
	return &r.versions[len(r.versions)-1]
}

func (r *resourceRecord) versionAt(timestamp int64) *resourceVersion {
	if len(r.versions) == 0 {
		return nil
	}
	idx := sort.Search(len(r.versions), func(i int) bool {
		return r.versions[i].timestamp > timestamp
	})
	if idx == 0 {
		return nil
	}
	return &r.versions[idx-1]
}

func (r *resourceRecord) activeVersionAt(timestamp int64) *resourceVersion {
	version := r.versionAt(timestamp)
	if version == nil || version.eventType == models.EventTypeDelete {
		return nil
	}
	return version
}

func (r *resourceRecord) visibleVersionWithinWindow(failureTimestampNs, windowStartNs int64) *resourceVersion {
	version := r.versionAt(failureTimestampNs)
	if version == nil {
		return nil
	}
	if version.eventType != models.EventTypeDelete {
		return version
	}
	if version.timestamp >= windowStartNs {
		return version
	}
	return nil
}

// Projection maintains shared mutable embedded indexes for timeline and analysis reads.
type Projection struct {
	mu sync.RWMutex

	events                    []models.Event
	eventsByResourceUID       map[string][]models.Event
	resourceMetaByUID         map[string]models.ResourceMetadata
	resourcesByUID            map[string]*resourceRecord
	resourcesByKey            map[resourceKey][]*resourceRecord
	k8sRawEventsByInvolvedUID map[string][]models.Event
	k8sEventsByInvolvedUID    map[string][]analysisstore.K8sEventInfo
	orderedResources          []orderedResourceKey
	minTimestampNs            int64
	maxTimestampNs            int64
}

func NewProjection() *Projection {
	return &Projection{
		eventsByResourceUID:       make(map[string][]models.Event),
		resourceMetaByUID:         make(map[string]models.ResourceMetadata),
		resourcesByUID:            make(map[string]*resourceRecord),
		resourcesByKey:            make(map[resourceKey][]*resourceRecord),
		k8sRawEventsByInvolvedUID: make(map[string][]models.Event),
		k8sEventsByInvolvedUID:    make(map[string][]analysisstore.K8sEventInfo),
		minTimestampNs:            -1,
		maxTimestampNs:            -1,
	}
}

func BuildProjection(events []models.Event) (*Projection, error) {
	projection := NewProjection()

	sortedEvents := cloneEvents(events)
	sort.SliceStable(sortedEvents, func(i, j int) bool {
		return compareEventOrder(sortedEvents[i], sortedEvents[j]) < 0
	})

	projection.events = sortedEvents
	for i := range sortedEvents {
		event := sortedEvents[i]

		if event.Resource.Kind == "Event" {
			if event.Resource.InvolvedObjectUID == "" {
				continue
			}
			involvedUID := event.Resource.InvolvedObjectUID
			projection.k8sRawEventsByInvolvedUID[involvedUID] = append(projection.k8sRawEventsByInvolvedUID[involvedUID], event)
			projection.k8sEventsByInvolvedUID[involvedUID] = append(projection.k8sEventsByInvolvedUID[involvedUID], buildK8sEventInfo(event))
			continue
		}

		if event.Resource.UID == "" {
			continue
		}

		uid := event.Resource.UID
		record := projection.resourcesByUID[uid]
		if record == nil {
			record = &resourceRecord{uid: uid}
			projection.resourcesByUID[uid] = record

			// Kubernetes object identity fields used for cursor ordering are immutable per UID.
			key := resourceKey{
				namespace: event.Resource.Namespace,
				kind:      event.Resource.Kind,
				name:      event.Resource.Name,
			}
			projection.resourcesByKey[key] = append(projection.resourcesByKey[key], record)
			projection.orderedResources = append(projection.orderedResources, orderedResourceKey{
				kind:      event.Resource.Kind,
				namespace: event.Resource.Namespace,
				name:      event.Resource.Name,
				uid:       uid,
			})
		}

		projection.eventsByResourceUID[uid] = append(projection.eventsByResourceUID[uid], event)
		projection.resourceMetaByUID[uid] = event.Resource
		if projection.minTimestampNs < 0 || event.Timestamp < projection.minTimestampNs {
			projection.minTimestampNs = event.Timestamp
		}
		if projection.maxTimestampNs < 0 || event.Timestamp > projection.maxTimestampNs {
			projection.maxTimestampNs = event.Timestamp
		}
	}

	sort.Slice(projection.orderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(projection.orderedResources[i], projection.orderedResources[j]) < 0
	})

	for uid := range projection.resourcesByUID {
		projection.rebuildRecord(uid)
	}
	for involvedUID := range projection.k8sEventsByInvolvedUID {
		sort.Slice(projection.k8sEventsByInvolvedUID[involvedUID], func(i, j int) bool {
			left := projection.k8sEventsByInvolvedUID[involvedUID][i]
			right := projection.k8sEventsByInvolvedUID[involvedUID][j]
			if left.Timestamp.Equal(right.Timestamp) {
				return left.EventID > right.EventID
			}
			return left.Timestamp.After(right.Timestamp)
		})
	}

	return projection, nil
}

func (p *Projection) Apply(event models.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cloned := cloneEvent(event)
	p.events = insertEventSorted(p.events, cloned)

	if cloned.Resource.Kind == "Event" {
		if cloned.Resource.InvolvedObjectUID == "" {
			return nil
		}
		involvedUID := cloned.Resource.InvolvedObjectUID
		p.k8sRawEventsByInvolvedUID[involvedUID] = insertEventSorted(p.k8sRawEventsByInvolvedUID[involvedUID], cloned)
		p.k8sEventsByInvolvedUID[involvedUID] = insertK8sEventInfoDescending(
			p.k8sEventsByInvolvedUID[involvedUID],
			buildK8sEventInfo(cloned),
		)
		return nil
	}

	if cloned.Resource.UID == "" {
		return nil
	}

	uid := cloned.Resource.UID
	if _, ok := p.resourcesByUID[uid]; !ok {
		record := &resourceRecord{uid: uid}
		p.resourcesByUID[uid] = record
		key := resourceKey{
			namespace: cloned.Resource.Namespace,
			kind:      cloned.Resource.Kind,
			name:      cloned.Resource.Name,
		}
		p.resourcesByKey[key] = append(p.resourcesByKey[key], record)
		p.orderedResources = insertOrderedResourceKey(p.orderedResources, orderedResourceKey{
			kind:      cloned.Resource.Kind,
			namespace: cloned.Resource.Namespace,
			name:      cloned.Resource.Name,
			uid:       uid,
		})
	}

	p.eventsByResourceUID[uid] = insertEventSorted(p.eventsByResourceUID[uid], cloned)
	p.resourceMetaByUID[uid] = latestResourceMeta(p.eventsByResourceUID[uid])
	p.rebuildRecord(uid)

	if p.minTimestampNs < 0 || cloned.Timestamp < p.minTimestampNs {
		p.minTimestampNs = cloned.Timestamp
	}
	if p.maxTimestampNs < 0 || cloned.Timestamp > p.maxTimestampNs {
		p.maxTimestampNs = cloned.Timestamp
	}

	return nil
}

func (p *Projection) SnapshotEvents() []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return cloneEvents(p.events)
}

func (p *Projection) rebuildRecord(uid string) {
	record := p.resourcesByUID[uid]
	if record == nil {
		return
	}

	events := p.eventsByResourceUID[uid]
	versions := make([]resourceVersion, 0, len(events))
	for i := range events {
		event := cloneEvent(events[i])
		object := parseObject(event.Data)

		var previousData []byte
		if len(versions) > 0 {
			previousData = versions[len(versions)-1].data
		}

		version := resourceVersion{
			event:     event,
			timestamp: event.Timestamp,
			eventType: event.Type,
			data:      cloneBytes(event.Data),
			object:    object,
		}
		version.identity = buildResourceIdentity(event, object, versions, previousData)
		version.changeEvent = buildChangeEventInfo(event, version.data, previousData)
		versions = append(versions, version)
	}

	record.versions = versions
}

func (p *Projection) resolveByName(namespace, kind, name string, failureTimestampNs, windowStartNs int64) *resourceVersion {
	records := p.resourcesByKey[resourceKey{namespace: namespace, kind: kind, name: name}]
	var deletedCandidate *resourceVersion
	for _, record := range records {
		if active := record.activeVersionAt(failureTimestampNs); active != nil {
			return active
		}
		if windowStartNs == 0 {
			continue
		}
		if deleted := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs); deleted != nil && deleted.eventType == models.EventTypeDelete {
			if deletedCandidate == nil || deleted.timestamp > deletedCandidate.timestamp {
				deletedCandidate = deleted
			}
		}
	}
	return deletedCandidate
}

func (p *Projection) activeResourcesInNamespace(namespace string, timestampNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range p.resourcesByUID {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.identity.Kind == "Event" || version.identity.Namespace != namespace {
			continue
		}
		result = append(result, version)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].identity.Kind != result[j].identity.Kind {
			return result[i].identity.Kind < result[j].identity.Kind
		}
		if result[i].identity.Name != result[j].identity.Name {
			return result[i].identity.Name < result[j].identity.Name
		}
		return result[i].identity.UID < result[j].identity.UID
	})
	return result
}

func latestResourceMeta(events []models.Event) models.ResourceMetadata {
	if len(events) == 0 {
		return models.ResourceMetadata{}
	}
	return events[len(events)-1].Resource
}

func insertEventSorted(events []models.Event, event models.Event) []models.Event {
	idx := sort.Search(len(events), func(i int) bool {
		return compareEventOrder(events[i], event) > 0
	})

	events = append(events, models.Event{})
	copy(events[idx+1:], events[idx:])
	events[idx] = event
	return events
}

func insertOrderedResourceKey(keys []orderedResourceKey, key orderedResourceKey) []orderedResourceKey {
	idx := sort.Search(len(keys), func(i int) bool {
		return compareOrderedResourceKey(keys[i], key) >= 0
	})
	if idx < len(keys) && compareOrderedResourceKey(keys[idx], key) == 0 {
		return keys
	}

	keys = append(keys, orderedResourceKey{})
	copy(keys[idx+1:], keys[idx:])
	keys[idx] = key
	return keys
}

func insertK8sEventInfoDescending(events []analysisstore.K8sEventInfo, event analysisstore.K8sEventInfo) []analysisstore.K8sEventInfo {
	idx := sort.Search(len(events), func(i int) bool {
		if events[i].Timestamp.Equal(event.Timestamp) {
			return events[i].EventID <= event.EventID
		}
		return events[i].Timestamp.Before(event.Timestamp)
	})

	events = append(events, analysisstore.K8sEventInfo{})
	copy(events[idx+1:], events[idx:])
	events[idx] = event
	return events
}

func compareEventOrder(left, right models.Event) int {
	if left.Timestamp != right.Timestamp {
		if left.Timestamp < right.Timestamp {
			return -1
		}
		return 1
	}
	switch {
	case left.ID < right.ID:
		return -1
	case left.ID > right.ID:
		return 1
	default:
		return 0
	}
}

func compareOrderedResourceKey(left, right orderedResourceKey) int {
	if left.kind != right.kind {
		if left.kind < right.kind {
			return -1
		}
		return 1
	}
	if left.namespace != right.namespace {
		if left.namespace < right.namespace {
			return -1
		}
		return 1
	}
	if left.name != right.name {
		if left.name < right.name {
			return -1
		}
		return 1
	}
	if left.uid < right.uid {
		return -1
	}
	if left.uid > right.uid {
		return 1
	}
	return 0
}

func cloneEvent(event models.Event) models.Event {
	cloned := event
	cloned.Data = cloneBytes(event.Data)
	return cloned
}

func cloneEvents(events []models.Event) []models.Event {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]models.Event, len(events))
	for i := range events {
		cloned[i] = cloneEvent(events[i])
	}
	return cloned
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	return append([]byte(nil), data...)
}

func buildResourceIdentity(event models.Event, object map[string]any, priorVersions []resourceVersion, previousData []byte) graph.ResourceIdentity {
	labels := extractLabels(object)
	if len(labels) == 0 && len(previousData) > 0 {
		labels = extractLabels(parseObject(previousData))
	}

	firstSeen := event.Timestamp
	if len(priorVersions) > 0 {
		firstSeen = priorVersions[0].timestamp
	}

	return graph.ResourceIdentity{
		UID:       event.Resource.UID,
		Kind:      event.Resource.Kind,
		APIGroup:  event.Resource.Group,
		Version:   event.Resource.Version,
		Namespace: event.Resource.Namespace,
		Name:      event.Resource.Name,
		Labels:    labels,
		FirstSeen: firstSeen,
		LastSeen:  event.Timestamp,
		Deleted:   event.Type == models.EventTypeDelete,
		DeletedAt: func() int64 {
			if event.Type == models.EventTypeDelete {
				return event.Timestamp
			}
			return 0
		}(),
	}
}

func buildChangeEventInfo(event models.Event, data, previousData []byte) analysisstore.ChangeEventInfo {
	var parsed *analyzerpkg.ResourceData
	if len(data) > 0 {
		parsed, _ = analyzerpkg.ParseResourceData(data)
	}

	status := analyzerpkg.InferStatusFromParsedData(event.Resource.Kind, parsed, string(event.Type))
	configChanged, statusChanged := detectChangeKinds(previousData, data)

	return analysisstore.ChangeEventInfo{
		EventID:       event.ID,
		Timestamp:     timeFromNs(event.Timestamp),
		EventType:     string(event.Type),
		Status:        status,
		ConfigChanged: configChanged,
		StatusChanged: statusChanged,
		Description:   fmt.Sprintf("%s event", event.Type),
		Data:          cloneBytes(data),
		FullSnapshot:  nil,
		Significance:  nil,
		Diff:          nil,
	}
}

func buildK8sEventInfo(event models.Event) analysisstore.K8sEventInfo {
	object := parseObject(event.Data)
	count := 1
	if value, ok := getInt(object, "count"); ok {
		count = int(value)
	}

	source := ""
	if sourceMap := getMap(object, "source"); sourceMap != nil {
		source = getString(sourceMap, "component")
		if source == "" {
			source = getString(sourceMap, "host")
		}
	}

	return analysisstore.K8sEventInfo{
		EventID:   event.ID,
		Timestamp: timeFromNs(event.Timestamp),
		Reason:    getString(object, "reason"),
		Message:   getString(object, "message"),
		Type:      getString(object, "type"),
		Count:     count,
		Source:    source,
	}
}

func detectChangeKinds(previousData, currentData []byte) (bool, bool) {
	if len(previousData) == 0 || len(currentData) == 0 {
		return false, false
	}

	previousObject := parseObject(previousData)
	currentObject := parseObject(currentData)
	kind := getString(currentObject, "kind")

	configChanged := detectKindSpecificConfigChange(kind, previousObject, currentObject)
	statusChanged := fieldChanged(previousObject, currentObject, "status")

	diffs, err := analysis.ComputeJSONDiff(previousData, currentData)
	if err != nil {
		return configChanged, statusChanged
	}
	for _, diff := range diffs {
		if bytes.HasPrefix([]byte(diff.Path), []byte("/spec")) || bytes.HasPrefix([]byte(diff.Path), []byte("spec")) {
			configChanged = true
		}
		if bytes.HasPrefix([]byte(diff.Path), []byte("/status")) || bytes.HasPrefix([]byte(diff.Path), []byte("status")) {
			statusChanged = true
		}
	}
	return configChanged, statusChanged
}

func detectKindSpecificConfigChange(kind string, previousObject, currentObject map[string]any) bool {
	switch kind {
	case "ConfigMap", "Secret":
		return fieldChanged(previousObject, currentObject, "data") ||
			fieldChanged(previousObject, currentObject, "binaryData") ||
			fieldChanged(previousObject, currentObject, "stringData")
	case "Role", "ClusterRole":
		return fieldChanged(previousObject, currentObject, "rules") ||
			fieldChanged(previousObject, currentObject, "aggregationRule")
	case "RoleBinding", "ClusterRoleBinding":
		return fieldChanged(previousObject, currentObject, "roleRef") ||
			fieldChanged(previousObject, currentObject, "subjects")
	default:
		return false
	}
}

func fieldChanged(previousObject, currentObject map[string]any, key string) bool {
	return !reflect.DeepEqual(previousObject[key], currentObject[key])
}

func extractLabels(object map[string]any) map[string]string {
	meta := getMap(object, "metadata")
	labelsMap := getMap(meta, "labels")
	if len(labelsMap) == 0 {
		return nil
	}
	labels := make(map[string]string, len(labelsMap))
	for key, value := range labelsMap {
		if stringValue, ok := value.(string); ok {
			labels[key] = stringValue
		}
	}
	return labels
}

func getMap(object map[string]any, key string) map[string]any {
	if object == nil {
		return nil
	}
	value, ok := object[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func getSlice(object map[string]any, key string) []any {
	if object == nil {
		return nil
	}
	value, ok := object[key]
	if !ok {
		return nil
	}
	typed, _ := value.([]any)
	return typed
}

func getString(object map[string]any, key string) string {
	if object == nil {
		return ""
	}
	value, ok := object[key]
	if !ok {
		return ""
	}
	typed, _ := value.(string)
	return typed
}

func getInt(object map[string]any, key string) (int64, bool) {
	if object == nil {
		return 0, false
	}
	value, ok := object[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}

func copyIdentity(version *resourceVersion) graph.ResourceIdentity {
	identity := version.identity
	identity.Labels = copyStringMap(identity.Labels)
	return identity
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func parseObject(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil
	}
	return object
}

func timeFromNs(value int64) time.Time {
	return time.Unix(0, value)
}
