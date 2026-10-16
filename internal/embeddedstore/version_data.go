package embeddedstore

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/moolen/spectre/internal/models"
)

// Resource versions keep their object payload in memory only until the event has been
// flushed into a segment. After that the projection holds a versionDataRef pointing at
// the segment record, and readers fetch the payload on demand. This keeps projection
// memory proportional to the number of versions' metadata instead of their payloads.

// segmentDataHandle is shared by every version stored in one segment. It owns a lazily
// opened read-only file descriptor; once no version references the handle any more it
// is collected and the descriptor closed, which keeps reads valid while a compaction or
// retention pass replaces the segment underneath concurrent readers.
type segmentDataHandle struct {
	id string

	mu              sync.Mutex
	path            string
	file            *os.File
	checkpointIndex uint32
}

// versionDataRef locates a payload record. Offset and length share one word: records
// are at most maxSegmentRecordSize (24 bits) and offsets stay below 2^40.
type versionDataRef struct {
	segment *segmentDataHandle
	packed  uint64
}

const (
	versionRefLengthBits = 24
	versionRefLengthMask = 1<<versionRefLengthBits - 1
)

func newVersionDataRef(segment *segmentDataHandle, offset int64, length uint32) versionDataRef {
	if segment == nil || offset < 0 || offset >= 1<<40 || length == 0 || length > versionRefLengthMask {
		return versionDataRef{}
	}
	return versionDataRef{segment: segment, packed: uint64(offset)<<versionRefLengthBits | uint64(length)}
}

func (r versionDataRef) valid() bool {
	return r.segment != nil && r.length() > 0
}

func (r versionDataRef) offset() int64 {
	return int64(r.packed >> versionRefLengthBits)
}

func (r versionDataRef) length() uint32 {
	return uint32(r.packed & versionRefLengthMask)
}

var versionDataReadErrors atomic.Uint64

func newSegmentDataHandle(id, root string) *segmentDataHandle {
	handle := &segmentDataHandle{id: id}
	if root != "" {
		handle.path = segmentEventsPath(root, id)
	}
	runtime.SetFinalizer(handle, func(h *segmentDataHandle) {
		h.mu.Lock()
		if h.file != nil {
			_ = h.file.Close()
			h.file = nil
		}
		h.mu.Unlock()
	})
	return handle
}

func segmentEventsPath(root, segmentID string) string {
	return filepath.Join(root, segmentsDirName, segmentID, segmentEventsFile)
}

func (h *segmentDataHandle) openFile() (*os.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file != nil {
		return h.file, nil
	}
	if h.path == "" {
		return nil, fmt.Errorf("segment %q has no data root", h.id)
	}
	file, err := os.Open(h.path)
	if err != nil {
		return nil, err
	}
	h.file = file
	return file, nil
}

func (h *segmentDataHandle) setRoot(root string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.path == "" && root != "" {
		h.path = segmentEventsPath(root, h.id)
	}
}

func (h *segmentDataHandle) readPayload(offset int64, length uint32) ([]byte, error) {
	if length < 4 {
		return nil, fmt.Errorf("segment %q record at %d: invalid length %d", h.id, offset, length)
	}
	file, err := h.openFile()
	if err != nil {
		return nil, fmt.Errorf("open segment %q: %w", h.id, err)
	}
	buf := make([]byte, length)
	if _, err := file.ReadAt(buf, offset); err != nil {
		return nil, fmt.Errorf("read segment %q record at %d: %w", h.id, offset, err)
	}
	header := binary.BigEndian.Uint32(buf[:4])
	compressed := header&segmentRecordCompressedFlag != 0
	if int(header&segmentRecordLengthMask) != len(buf)-4 {
		return nil, fmt.Errorf("segment %q record at %d: length mismatch", h.id, offset)
	}
	event, _, err := decodeFramedPayload(buf[4:], compressed, nil)
	if err != nil {
		return nil, fmt.Errorf("segment %q record at %d: %w", h.id, offset, err)
	}
	return event.Data, nil
}

// versionPayload returns the version's object payload, reading it from its segment when
// it is no longer held in memory. The result must not be modified: for in-memory
// versions it aliases the projection's copy.
func versionPayload(version *resourceVersion) []byte {
	if version == nil {
		return nil
	}
	if inline := version.inlinePayload(); len(inline) > 0 || !version.ref.valid() {
		return inline
	}
	data, err := version.ref.segment.readPayload(version.ref.offset(), version.ref.length())
	if err != nil {
		versionDataReadErrors.Add(1)
		return nil
	}
	return data
}

// versionPayloadCopy is versionPayload for callers that hand the bytes out.
func versionPayloadCopy(version *resourceVersion) []byte {
	if version == nil {
		return nil
	}
	if inline := version.inlinePayload(); len(inline) > 0 || !version.ref.valid() {
		return cloneBytes(inline)
	}
	return versionPayload(version)
}

func versionHasPayload(version *resourceVersion) bool {
	return version != nil && (len(version.inlinePayload()) > 0 || version.ref.valid())
}

func payloadHash(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	return xxhash.Sum64(data)
}

// SetDataRoot tells the projection where segment bundles live so it can resolve
// payload references restored from checkpoints.
func (p *Projection) SetDataRoot(root string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dataRoot = root
	for _, handle := range p.segmentHandles {
		handle.setRoot(root)
	}
}

func (p *Projection) segmentHandleLocked(segmentID string) *segmentDataHandle {
	if p.segmentHandles == nil {
		p.segmentHandles = make(map[string]*segmentDataHandle)
	}
	if handle, ok := p.segmentHandles[segmentID]; ok {
		return handle
	}
	handle := newSegmentDataHandle(segmentID, p.dataRoot)
	p.segmentHandles[segmentID] = handle
	return handle
}

// AttachSegmentLocations points the matching resource versions at their records in a
// newly written segment and drops their in-memory payloads. Versions that are not found
// (for example because retention already pruned them) are skipped.
func (p *Projection) AttachSegmentLocations(segmentID string, locations []segmentRecordLocation) int {
	if p == nil || len(locations) == 0 {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	handle := p.segmentHandleLocked(segmentID)
	attached := 0
	for i := range locations {
		location := &locations[i]
		record := p.resourcesByUID[location.UID]
		if record == nil {
			continue
		}
		version := record.findVersion(location.Timestamp, location.EventID)
		if version == nil {
			continue
		}
		version.ref = newVersionDataRef(handle, location.Offset, location.Length)
		version.setInlinePayload(nil)
		attached++
	}
	return attached
}

// attachReplayLocation points a just-replayed version at its segment record so repair
// replays do not hold every payload in memory.
func (p *Projection) attachReplayLocation(event models.Event, location replayLocation) {
	if p == nil || location.segmentID == "" || location.length == 0 || event.Resource.Kind == "Event" || event.Resource.UID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	record := p.resourcesByUID[event.Resource.UID]
	if record == nil {
		return
	}
	version := record.findVersion(event.Timestamp, event.ID)
	if version == nil {
		return
	}
	version.ref = newVersionDataRef(p.segmentHandleLocked(location.segmentID), location.offset, location.length)
	version.setInlinePayload(nil)
}

// ReleaseSegmentHandles forgets handles for segments that no version references any more,
// so their descriptors can be closed once in-flight readers finish.
func (p *Projection) ReleaseSegmentHandles(segmentIDs []string) {
	if p == nil || len(segmentIDs) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range segmentIDs {
		delete(p.segmentHandles, id)
	}
}

// pinnedVersionKeys lists versions that must stay readable even though they are older
// than cutoffTimestamp: the state each resource was in when the retained window starts.
func (p *Projection) pinnedVersionKeys(cutoffTimestamp int64) map[pinnedVersionKey]struct{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pinned := make(map[pinnedVersionKey]struct{})
	for uid, record := range p.resourcesByUID {
		retained := retainedVersionsBefore(record, cutoffTimestamp)
		for _, version := range retained {
			pinned[pinnedVersionKey{uid: uid, eventID: version.eventID(), timestamp: version.timestamp}] = struct{}{}
		}
	}
	return pinned
}

type pinnedVersionKey struct {
	uid       string
	eventID   string
	timestamp int64
}

// retainedVersionsBefore returns the versions older than cutoff that retention keeps.
func retainedVersionsBefore(record *resourceRecord, cutoffTimestamp int64) []*resourceVersion {
	if record == nil || len(record.versions) == 0 || cutoffTimestamp <= 0 {
		return nil
	}
	retainedStart := sort.Search(len(record.versions), func(i int) bool {
		return record.versions[i].timestamp >= cutoffTimestamp
	})
	if retainedStart == 0 {
		return nil
	}
	if retainedStart >= len(record.versions) {
		latest := &record.versions[len(record.versions)-1]
		if latest.eventType() == models.EventTypeDelete {
			return nil
		}
		return []*resourceVersion{latest}
	}
	sentinel := &record.versions[retainedStart-1]
	if checkpointEquivalentState(*sentinel, record.versions[retainedStart]) {
		return nil
	}
	return []*resourceVersion{sentinel}
}

func (r *resourceRecord) findVersion(timestamp int64, eventID string) *resourceVersion {
	idx := sort.Search(len(r.versions), func(i int) bool {
		return r.versions[i].timestamp >= timestamp
	})
	id := newEventIDMatcher(eventID)
	for i := idx; i < len(r.versions) && r.versions[i].timestamp == timestamp; i++ {
		if id.matches(&r.versions[i]) {
			return &r.versions[i]
		}
	}
	return nil
}
