package embeddedstore

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// recordChangeState summarises the latest version's object so the next version can be
// classified without re-reading or re-parsing the previous payload.
type recordChangeState struct {
	known       bool
	hasData     bool
	fingerprint changeFingerprint
	labels      map[string]string
}

// changeFingerprint holds order-independent hashes of the top-level fields that drive
// change classification. Two objects with equal fingerprints are treated as equal for
// those fields, matching reflect.DeepEqual on the parsed JSON up to hash collisions.
type changeFingerprint struct {
	valid        bool
	spec         uint64
	status       uint64
	kindSpecific uint64
}

func newRecordChangeState(data []byte, object map[string]any) recordChangeState {
	if len(data) == 0 {
		return recordChangeState{known: true}
	}
	return recordChangeState{
		known:       true,
		hasData:     true,
		fingerprint: computeChangeFingerprint(object),
		labels:      extractLabels(object),
	}
}

// buildResourceVersion derives a version (identity and change info) from an event and the
// change state of the version before it. It parses the payload exactly once.
func buildResourceVersion(event models.Event, priorVersions []resourceVersion, previous recordChangeState) (resourceVersion, recordChangeState) {
	object := parseObject(event.Data)
	current := newRecordChangeState(event.Data, object)

	labels := current.labels
	if len(labels) == 0 && previous.hasData {
		labels = previous.labels
	} else if previous.hasData && stringMapsEqual(labels, previous.labels) {
		// Share the previous map: identities never mutate labels in place.
		labels = previous.labels
		current.labels = previous.labels
	}

	version := resourceVersion{
		eventID:   event.ID,
		timestamp: event.Timestamp,
		typeCode:  versionEventTypes.code(string(event.Type)),
		data:      cloneBytes(event.Data),
		dataHash:  payloadHash(event.Data),
	}
	var previousMeta *versionMeta
	if len(priorVersions) > 0 {
		previousMeta = priorVersions[len(priorVersions)-1].meta
	}
	version.setIdentity(buildResourceIdentityWithLabels(event, labels, priorVersions), previousMeta)
	version.setChangeInfo(buildChangeEventInfoFromObject(event, object, previous, current))
	return version, current
}

func buildResourceIdentityWithLabels(event models.Event, labels map[string]string, priorVersions []resourceVersion) graph.ResourceIdentity {
	firstSeen := event.Timestamp
	if len(priorVersions) > 0 {
		firstSeen = priorVersions[0].timestamp
	}

	identity := graph.ResourceIdentity{
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
	}
	if event.Type == models.EventTypeDelete {
		identity.DeletedAt = event.Timestamp
	}
	if len(priorVersions) > 0 {
		// Reuse the previous version's strings so long histories share one copy.
		prior := priorVersions[len(priorVersions)-1].identity()
		identity.UID = internEqual(identity.UID, prior.UID)
		identity.Kind = internEqual(identity.Kind, prior.Kind)
		identity.APIGroup = internEqual(identity.APIGroup, prior.APIGroup)
		identity.Version = internEqual(identity.Version, prior.Version)
		identity.Namespace = internEqual(identity.Namespace, prior.Namespace)
		identity.Name = internEqual(identity.Name, prior.Name)
	}
	return identity
}

func internEqual(value, prior string) string {
	if value == prior {
		return prior
	}
	return value
}

func buildChangeEventInfoFromObject(event models.Event, object map[string]any, previous, current recordChangeState) analysisstore.ChangeEventInfo {
	var parsed *analyzerpkg.ResourceData
	if object != nil {
		parsed = analyzerpkg.ResourceDataFromObject(object)
	}

	status := analyzerpkg.InferStatusFromParsedData(event.Resource.Kind, parsed, string(event.Type))
	configChanged, statusChanged := changeKindsFromStates(previous, current)

	return analysisstore.ChangeEventInfo{
		EventID:       event.ID,
		Timestamp:     timeFromNs(event.Timestamp),
		EventType:     string(event.Type),
		Status:        status,
		ConfigChanged: configChanged,
		StatusChanged: statusChanged,
		Description:   changeDescription(event.Type),
	}
}

func changeDescription(eventType models.EventType) string {
	switch eventType {
	case models.EventTypeCreate:
		return "CREATE event"
	case models.EventTypeUpdate:
		return "UPDATE event"
	case models.EventTypeDelete:
		return "DELETE event"
	default:
		return fmt.Sprintf("%s event", eventType)
	}
}

// changeKindsFromStates reports whether spec-like and status-like top-level fields differ
// between two consecutive versions. A version without a payload (or with an unparsable
// one) never counts as a change.
func changeKindsFromStates(previous, current recordChangeState) (bool, bool) {
	if !previous.hasData || !current.hasData {
		return false, false
	}
	prev, cur := previous.fingerprint, current.fingerprint
	if !prev.valid || !cur.valid {
		// An unparsable payload compares as an empty object, and its spec fields are not
		// diffed; kind-specific fields only count when the current payload parses.
		configChanged := cur.valid && prev.kindSpecific != cur.kindSpecific
		return configChanged, prev.status != cur.status
	}
	configChanged := prev.spec != cur.spec || prev.kindSpecific != cur.kindSpecific
	statusChanged := prev.status != cur.status
	return configChanged, statusChanged
}

func computeChangeFingerprint(object map[string]any) changeFingerprint {
	if object == nil {
		return changeFingerprint{}
	}
	fp := changeFingerprint{valid: true}
	specificKeys := kindSpecificConfigKeys(getString(object, "kind"))
	for key, value := range object {
		var target *uint64
		switch {
		case strings.HasPrefix(key, "spec"):
			target = &fp.spec
		case strings.HasPrefix(key, "status"):
			target = &fp.status
		default:
			// Kind-specific fields compare like map lookups: null equals absent.
			if value == nil {
				continue
			}
			for _, specific := range specificKeys {
				if key == specific {
					target = &fp.kindSpecific
					break
				}
			}
		}
		if target == nil {
			continue
		}
		*target += hashKeyedValue(key, value)
	}
	return fp
}

func kindSpecificConfigKeys(kind string) []string {
	switch kind {
	case "ConfigMap", "Secret":
		return []string{"data", "binaryData", "stringData"}
	case "Role", "ClusterRole":
		return []string{"rules", "aggregationRule"}
	case "RoleBinding", "ClusterRoleBinding":
		return []string{"roleRef", "subjects"}
	default:
		return nil
	}
}

// hashKeyedValue hashes one map entry. Entries are summed by callers, which keeps the
// map hash independent of iteration order.
func hashKeyedValue(key string, value any) uint64 {
	h := xxhash.New()
	_, _ = h.WriteString(key)
	_, _ = h.Write([]byte{0})
	hashJSONValue(h, value)
	return mix64(h.Sum64())
}

func hashJSONValue(h *xxhash.Digest, value any) {
	var scratch [9]byte
	switch typed := value.(type) {
	case nil:
		_, _ = h.Write([]byte{'n'})
	case bool:
		if typed {
			_, _ = h.Write([]byte{'t'})
		} else {
			_, _ = h.Write([]byte{'f'})
		}
	case float64:
		scratch[0] = 'd'
		binary.LittleEndian.PutUint64(scratch[1:], math.Float64bits(typed))
		_, _ = h.Write(scratch[:])
	case string:
		scratch[0] = 's'
		binary.LittleEndian.PutUint64(scratch[1:], uint64(len(typed)))
		_, _ = h.Write(scratch[:])
		_, _ = h.WriteString(typed)
	case []any:
		scratch[0] = 'a'
		binary.LittleEndian.PutUint64(scratch[1:], uint64(len(typed)))
		_, _ = h.Write(scratch[:])
		for i := range typed {
			hashJSONValue(h, typed[i])
		}
	case map[string]any:
		var sum uint64
		for key, item := range typed {
			sum += hashKeyedValue(key, item)
		}
		scratch[0] = 'm'
		binary.LittleEndian.PutUint64(scratch[1:], sum)
		_, _ = h.Write(scratch[:])
	default:
		_, _ = fmt.Fprintf(h, "?%v", typed)
	}
}

// mix64 is the splitmix64 finalizer; it spreads entry hashes before they are summed.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
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
	identity := version.identity()
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

func parsedVersionObject(version *resourceVersion) map[string]any {
	if version == nil {
		return nil
	}

	return parseObject(versionPayload(version))
}

func timeFromNs(value int64) time.Time {
	return time.Unix(0, value)
}
