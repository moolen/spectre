package embeddedstore

import (
	"sync"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// versionMeta holds the identity fields and labels shared by consecutive versions of a
// resource. Values are immutable once versions reference them.
type versionMeta struct {
	UID       string
	Kind      string
	APIGroup  string
	Version   string
	Namespace string
	Name      string
	FirstSeen int64
	Labels    map[string]string
}

const (
	versionFlagConfigChanged uint8 = 1 << iota
	versionFlagStatusChanged
)

// smallStringTable interns the few distinct event types and statuses into one-byte
// codes. Code 0 is the empty string; values past 255 distinct strings fall back to it.
type smallStringTable struct {
	mu     sync.RWMutex
	codes  map[string]uint8
	values []string
}

func newSmallStringTable(initial ...string) *smallStringTable {
	table := &smallStringTable{codes: map[string]uint8{"": 0}, values: []string{""}}
	for _, value := range initial {
		table.code(value)
	}
	return table
}

func (t *smallStringTable) code(value string) uint8 {
	t.mu.RLock()
	code, ok := t.codes[value]
	t.mu.RUnlock()
	if ok {
		return code
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if code, ok := t.codes[value]; ok {
		return code
	}
	if len(t.values) > 255 {
		return 0
	}
	code = uint8(len(t.values))
	t.codes[value] = code
	t.values = append(t.values, value)
	return code
}

func (t *smallStringTable) value(code uint8) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if int(code) < len(t.values) {
		return t.values[code]
	}
	return ""
}

var (
	versionEventTypes = newSmallStringTable(string(models.EventTypeCreate), string(models.EventTypeUpdate), string(models.EventTypeDelete))
	versionStatuses   = newSmallStringTable()
	deleteTypeCode    = versionEventTypes.code(string(models.EventTypeDelete))
)

func (v *resourceVersion) eventType() models.EventType {
	return models.EventType(versionEventTypes.value(v.typeCode))
}

func (v *resourceVersion) isDelete() bool {
	return v.typeCode == deleteTypeCode
}

func (v *resourceVersion) status() string {
	return versionStatuses.value(v.statusCode)
}

func (v *resourceVersion) configChanged() bool {
	return v.flags&versionFlagConfigChanged != 0
}

func (v *resourceVersion) statusChanged() bool {
	return v.flags&versionFlagStatusChanged != 0
}

func (v *resourceVersion) metaOrEmpty() *versionMeta {
	if v.meta == nil {
		return &versionMeta{}
	}
	return v.meta
}

// identity returns the graph identity of this version. Labels are shared with the
// projection and must not be modified.
func (v *resourceVersion) identity() graph.ResourceIdentity {
	meta := v.metaOrEmpty()
	identity := graph.ResourceIdentity{
		UID:       meta.UID,
		Kind:      meta.Kind,
		APIGroup:  meta.APIGroup,
		Version:   meta.Version,
		Namespace: meta.Namespace,
		Name:      meta.Name,
		Labels:    meta.Labels,
		FirstSeen: meta.FirstSeen,
		LastSeen:  v.timestamp,
		Deleted:   v.isDelete(),
	}
	if identity.Deleted {
		identity.DeletedAt = v.timestamp
	}
	return identity
}

func (v *resourceVersion) changeEventInfo() analysisstore.ChangeEventInfo {
	eventType := v.eventType()
	return analysisstore.ChangeEventInfo{
		EventID:       v.eventID(),
		Timestamp:     timeFromNs(v.timestamp),
		EventType:     string(eventType),
		Status:        v.status(),
		ConfigChanged: v.configChanged(),
		StatusChanged: v.statusChanged(),
		Description:   changeDescription(eventType),
	}
}

func (v *resourceVersion) resourceMetadata() models.ResourceMetadata {
	meta := v.metaOrEmpty()
	return models.ResourceMetadata{
		Group:     meta.APIGroup,
		Version:   meta.Version,
		Kind:      meta.Kind,
		Namespace: meta.Namespace,
		Name:      meta.Name,
		UID:       meta.UID,
	}
}

// sharedVersionMeta returns previous when it already describes identity, so long
// histories share one meta value.
func sharedVersionMeta(previous *versionMeta, identity graph.ResourceIdentity) *versionMeta {
	if previous != nil &&
		previous.UID == identity.UID &&
		previous.Kind == identity.Kind &&
		previous.APIGroup == identity.APIGroup &&
		previous.Version == identity.Version &&
		previous.Namespace == identity.Namespace &&
		previous.Name == identity.Name &&
		previous.FirstSeen == identity.FirstSeen &&
		stringMapsEqual(previous.Labels, identity.Labels) {
		return previous
	}
	labels := identity.Labels
	if previous != nil && stringMapsEqual(previous.Labels, labels) {
		labels = previous.Labels
	}
	return &versionMeta{
		UID:       identity.UID,
		Kind:      identity.Kind,
		APIGroup:  identity.APIGroup,
		Version:   identity.Version,
		Namespace: identity.Namespace,
		Name:      identity.Name,
		FirstSeen: identity.FirstSeen,
		Labels:    labels,
	}
}

// setChangeInfo stores the parts of a ChangeEventInfo that are not derived from the
// version itself.
func (v *resourceVersion) setChangeInfo(info analysisstore.ChangeEventInfo) {
	v.statusCode = versionStatuses.code(info.Status)
	v.flags = 0
	if info.ConfigChanged {
		v.flags |= versionFlagConfigChanged
	}
	if info.StatusChanged {
		v.flags |= versionFlagStatusChanged
	}
}

// setIdentity stores identity, sharing meta with the previous version when possible.
func (v *resourceVersion) setIdentity(identity graph.ResourceIdentity, previous *versionMeta) {
	v.meta = sharedVersionMeta(previous, identity)
}

// labels returns the version's labels; they are shared and must not be modified.
func (v *resourceVersion) labels() map[string]string {
	if v.meta == nil {
		return nil
	}
	return v.meta.Labels
}

func (v *resourceVersion) eventID() string {
	return expandEventID(v.id)
}

// inlinePayload returns the payload held in memory (unflushed versions), or nil.
func (v *resourceVersion) inlinePayload() []byte {
	if v.pending == nil {
		return nil
	}
	return *v.pending
}

func (v *resourceVersion) setInlinePayload(data []byte) {
	if len(data) == 0 {
		v.pending = nil
		return
	}
	v.pending = &data
}

// compactUUIDMarker prefixes event IDs stored as the 16 raw bytes of a UUID.
const compactUUIDMarker = 0xFF

// compactEventID stores canonical lowercase UUIDs (the watcher's event IDs) as 17
// bytes instead of 36; other IDs are kept as they are.
func compactEventID(id string) string {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return id
	}
	var raw [17]byte
	raw[0] = compactUUIDMarker
	j := 1
	for i := 0; i < 36; i += 2 {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			i--
			continue
		}
		hi, ok1 := lowerHexValue(id[i])
		lo, ok2 := lowerHexValue(id[i+1])
		if !ok1 || !ok2 {
			return id
		}
		raw[j] = hi<<4 | lo
		j++
	}
	return string(raw[:])
}

func expandEventID(id string) string {
	if len(id) != 17 || id[0] != compactUUIDMarker {
		return id
	}
	const hexDigits = "0123456789abcdef"
	var out [36]byte
	j := 0
	for i := 1; i < 17; i++ {
		if j == 8 || j == 13 || j == 18 || j == 23 {
			out[j] = '-'
			j++
		}
		out[j] = hexDigits[id[i]>>4]
		out[j+1] = hexDigits[id[i]&0x0F]
		j += 2
	}
	return string(out[:])
}

func lowerHexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	default:
		return 0, false
	}
}
