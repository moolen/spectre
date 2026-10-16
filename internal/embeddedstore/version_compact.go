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
	uid, name := identity.UID, identity.Name
	if previous != nil {
		if stringMapsEqual(previous.Labels, labels) {
			labels = previous.Labels
		}
		if previous.UID == uid {
			uid = previous.UID
		}
		if previous.Name == name {
			name = previous.Name
		}
	}
	return &versionMeta{
		UID:       uid,
		Kind:      internString(identity.Kind),
		APIGroup:  internString(identity.APIGroup),
		Version:   internString(identity.Version),
		Namespace: internString(identity.Namespace),
		Name:      name,
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

const (
	idFormEmpty uint8 = iota
	idFormUUID
	idFormText
)

func (v *resourceVersion) eventID() string {
	switch v.idForm {
	case idFormUUID:
		return formatUUID(v.uuid)
	case idFormText:
		return v.extra.textID
	default:
		return ""
	}
}

// setEventID stores canonical lowercase UUIDs (the watcher's event IDs) inline; other
// IDs go to extra.
func (v *resourceVersion) setEventID(id string) {
	if uuid, ok := parseCanonicalUUID(id); ok {
		v.uuid, v.idForm = uuid, idFormUUID
		v.setExtra("", v.inlinePayload())
		return
	}
	v.uuid = [16]byte{}
	v.idForm = idFormText
	if id == "" {
		v.idForm = idFormEmpty
	}
	v.setExtra(id, v.inlinePayload())
}

// eventIDMatcher compares version event IDs against one ID without formatting them.
type eventIDMatcher struct {
	id     string
	uuid   [16]byte
	isUUID bool
}

func newEventIDMatcher(id string) eventIDMatcher {
	uuid, ok := parseCanonicalUUID(id)
	return eventIDMatcher{id: id, uuid: uuid, isUUID: ok}
}

func (m eventIDMatcher) matches(v *resourceVersion) bool {
	if m.isUUID {
		return v.idForm == idFormUUID && v.uuid == m.uuid
	}
	return v.idForm != idFormUUID && v.eventID() == m.id
}

// inlinePayload returns the payload held in memory (unflushed versions), or nil.
func (v *resourceVersion) inlinePayload() []byte {
	if v.extra == nil {
		return nil
	}
	return v.extra.pending
}

func (v *resourceVersion) setInlinePayload(data []byte) {
	if len(data) == 0 {
		data = nil
	}
	textID := ""
	if v.idForm == idFormText {
		textID = v.extra.textID
	}
	v.setExtra(textID, data)
}

// setExtra replaces the version's extra fields, dropping the allocation when both are
// empty. It never mutates the previous extra, so copies of a version stay independent.
func (v *resourceVersion) setExtra(textID string, pending []byte) {
	if textID == "" && pending == nil {
		v.extra = nil
		return
	}
	v.extra = &versionExtra{textID: textID, pending: pending}
}

func parseCanonicalUUID(id string) ([16]byte, bool) {
	var raw [16]byte
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return raw, false
	}
	j := 0
	for i := 0; i < 36; i += 2 {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			i--
			continue
		}
		hi, ok1 := lowerHexValue(id[i])
		lo, ok2 := lowerHexValue(id[i+1])
		if !ok1 || !ok2 {
			return [16]byte{}, false
		}
		raw[j] = hi<<4 | lo
		j++
	}
	return raw, true
}

func formatUUID(raw [16]byte) string {
	const hexDigits = "0123456789abcdef"
	var out [36]byte
	j := 0
	for i := 0; i < 16; i++ {
		if j == 8 || j == 13 || j == 18 || j == 23 {
			out[j] = '-'
			j++
		}
		out[j] = hexDigits[raw[i]>>4]
		out[j+1] = hexDigits[raw[i]&0x0F]
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
