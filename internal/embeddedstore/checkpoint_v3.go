package embeddedstore

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

// Checkpoint format v3 is a compact binary encoding of the projection. Versions are
// written as deltas against the previous version of the same resource (timestamps,
// shared identity and labels), payloads as segment references, and the whole stream is
// zstd compressed. Encoding works from a checkpointSnapshot so it can run without the
// engine lock while ingest continues.
const (
	checkpointFormatVersionV3 = 3
	checkpointResourcesFileV3 = "resources.v3"
	checkpointK8sEventsFileV3 = "k8s-events.v3"
	checkpointV3Magic         = "SPCHK3\n"

	checkpointV3PayloadNone   byte = 0
	checkpointV3PayloadInline byte = 1
	checkpointV3PayloadRef    byte = 2

	checkpointV3SameAsPrevious byte = 0
	checkpointV3Nil            byte = 1
	checkpointV3New            byte = 2

	checkpointV3StateKnown   byte = 1
	checkpointV3StateHasData byte = 2
	checkpointV3StateFPValid byte = 4
)

type checkpointRecordSnapshot struct {
	uid string
	// versions is capped at capture time; elements are shared with the projection and
	// their payload fields must be read under the projection lock.
	versions []resourceVersion
	latest   recordChangeState
}

type checkpointSnapshot struct {
	highWaterMark  uint64
	minTimestampNs int64
	maxTimestampNs int64
	records        []checkpointRecordSnapshot
	k8sEvents      map[string][]analysisstore.K8sEventInfo
	k8sTruncated   map[string]int64
	segmentTable   []string
	eventTypes     []string
	statuses       []string
}

// captureCheckpointSnapshot records the projection's state at highWaterMark. It is
// cheap (no payloads or versions are copied); the caller must hold the engine lock so no
// event above the high-water mark has been applied yet.
func (p *Projection) captureCheckpointSnapshot(highWaterMark uint64) *checkpointSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot := &checkpointSnapshot{
		highWaterMark:  highWaterMark,
		minTimestampNs: p.minTimestampNs,
		maxTimestampNs: p.maxTimestampNs,
		records:        make([]checkpointRecordSnapshot, 0, len(p.resourcesByUID)),
		k8sEvents:      make(map[string][]analysisstore.K8sEventInfo, len(p.k8sEventsByInvolvedUID)),
		k8sTruncated:   make(map[string]int64, len(p.k8sEventsTruncatedThrough)),
	}
	p.orderedResources.Ascend(func(key orderedResourceKey) bool {
		record := p.resourcesByUID[key.uid]
		if record == nil || len(record.versions) == 0 {
			return true
		}
		snapshot.records = append(snapshot.records, checkpointRecordSnapshot{
			uid:      record.uid,
			versions: record.versions[:len(record.versions):len(record.versions)],
			latest:   record.latestChangeState(),
		})
		return true
	})
	for uid, events := range p.k8sEventsByInvolvedUID {
		snapshot.k8sEvents[uid] = append([]analysisstore.K8sEventInfo(nil), events...)
	}
	for uid, truncated := range p.k8sEventsTruncatedThrough {
		snapshot.k8sTruncated[uid] = truncated
	}

	snapshot.segmentTable = make([]string, 0, len(p.segmentHandles))
	for id := range p.segmentHandles {
		snapshot.segmentTable = append(snapshot.segmentTable, id)
	}
	sortStrings(snapshot.segmentTable)
	for i, id := range snapshot.segmentTable {
		handle := p.segmentHandles[id]
		handle.mu.Lock()
		handle.checkpointIndex = uint32(i + 1)
		handle.mu.Unlock()
	}

	versionEventTypes.mu.RLock()
	snapshot.eventTypes = append([]string(nil), versionEventTypes.values...)
	versionEventTypes.mu.RUnlock()
	versionStatuses.mu.RLock()
	snapshot.statuses = append([]string(nil), versionStatuses.values...)
	versionStatuses.mu.RUnlock()
	return snapshot
}

// writeCheckpointV3 encodes a snapshot into a new checkpoint directory. It may run
// concurrently with ingest: it only takes the projection read lock per resource.
func writeCheckpointV3(rootDir string, projection *Projection, snapshot *checkpointSnapshot) (CheckpointMeta, error) {
	checkpointsRoot := filepath.Join(rootDir, checkpointsDirName)
	if err := ensureDirWithParentSync(checkpointsRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: ensure checkpoints dir: %w", err)
	}
	tmpRoot := filepath.Join(rootDir, segmentTempDirName)
	if err := ensureDirWithParentSync(tmpRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: ensure temp dir: %w", err)
	}

	checkpointID := newCheckpointID(snapshot.highWaterMark)
	finalDir := filepath.Join(checkpointsRoot, checkpointID)
	tmpDir, err := os.MkdirTemp(tmpRoot, checkpointID+".tmp-*")
	if err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: create temp dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	state := checkpointState{
		FormatVersion:  checkpointFormatVersionV3,
		HighWaterMark:  snapshot.highWaterMark,
		MinTimestampNs: snapshot.minTimestampNs,
		MaxTimestampNs: snapshot.maxTimestampNs,
		SegmentTable:   snapshot.segmentTable,
	}
	if err := writeJSONFile(filepath.Join(tmpDir, checkpointStateFile), state); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: state file: %w", err)
	}
	if err := writeCompressedStream(filepath.Join(tmpDir, checkpointResourcesFileV3), func(w *bufio.Writer) error {
		return encodeCheckpointResourcesV3(w, projection, snapshot)
	}); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: resources: %w", err)
	}
	if err := writeCompressedStream(filepath.Join(tmpDir, checkpointK8sEventsFileV3), func(w *bufio.Writer) error {
		return encodeCheckpointK8sEventsV3(w, snapshot)
	}); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: k8s events: %w", err)
	}
	if err := syncPath(tmpDir); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: sync temp dir: %w", err)
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: move into place: %w", err)
	}
	if err := syncPath(checkpointsRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: sync checkpoints dir: %w", err)
	}
	cleanup = false
	return CheckpointMeta{ID: checkpointID, HighWaterMark: snapshot.highWaterMark}, nil
}

func writeCompressedStream(path string, encode func(*bufio.Writer) error) error {
	file, encoder, err := openCompressedCheckpointWriter(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	writer := bufio.NewWriterSize(encoder, 1<<20)
	encodeErr := encode(writer)
	if encodeErr == nil {
		encodeErr = writer.Flush()
	}
	closeErr := encoder.Close()
	if encodeErr == nil && closeErr == nil {
		closeErr = file.Sync()
	}
	fileErr := file.Close()
	switch {
	case encodeErr != nil:
		return encodeErr
	case closeErr != nil:
		return closeErr
	default:
		return fileErr
	}
}

type checkpointV3Writer struct {
	w       *bufio.Writer
	scratch []byte
	err     error
}

func (c *checkpointV3Writer) bytes(value []byte) {
	if c.err != nil {
		return
	}
	_, c.err = c.w.Write(value)
}

func (c *checkpointV3Writer) byte(value byte) {
	if c.err != nil {
		return
	}
	c.err = c.w.WriteByte(value)
}

func (c *checkpointV3Writer) uvarint(value uint64) {
	c.scratch = binary.AppendUvarint(c.scratch[:0], value)
	c.bytes(c.scratch)
}

func (c *checkpointV3Writer) varint(value int64) {
	c.scratch = binary.AppendVarint(c.scratch[:0], value)
	c.bytes(c.scratch)
}

func (c *checkpointV3Writer) uint64(value uint64) {
	c.scratch = binary.LittleEndian.AppendUint64(c.scratch[:0], value)
	c.bytes(c.scratch)
}

func (c *checkpointV3Writer) string(value string) {
	c.uvarint(uint64(len(value)))
	if c.err != nil || len(value) == 0 {
		return
	}
	_, c.err = c.w.WriteString(value)
}

func (c *checkpointV3Writer) blob(value []byte) {
	c.uvarint(uint64(len(value)))
	c.bytes(value)
}

func (c *checkpointV3Writer) stringList(values []string) {
	c.uvarint(uint64(len(values)))
	for _, value := range values {
		c.string(value)
	}
}

func (c *checkpointV3Writer) labels(labels, previous map[string]string, first bool) {
	switch {
	case !first && sameStringMap(labels, previous):
		c.byte(checkpointV3SameAsPrevious)
	case labels == nil:
		c.byte(checkpointV3Nil)
	default:
		c.byte(checkpointV3New)
		c.uvarint(uint64(len(labels)))
		for key, value := range labels {
			c.string(key)
			c.string(value)
		}
	}
}

// sameStringMap reports whether two maps are the same instance (or both nil). Versions
// share label maps when unchanged, so identity is the cheap and exact test here.
func sameStringMap(a, b map[string]string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	return mapIdentity(a) == mapIdentity(b)
}

func encodeCheckpointResourcesV3(w *bufio.Writer, projection *Projection, snapshot *checkpointSnapshot) error {
	out := &checkpointV3Writer{w: w}
	out.string(checkpointV3Magic)
	out.stringList(snapshot.eventTypes)
	out.stringList(snapshot.statuses)
	out.uvarint(uint64(len(snapshot.records)))

	for i := range snapshot.records {
		record := &snapshot.records[i]
		projection.mu.RLock()
		encodeCheckpointRecordV3(out, record)
		projection.mu.RUnlock()
		if out.err != nil {
			return out.err
		}
	}
	return out.err
}

func encodeCheckpointRecordV3(out *checkpointV3Writer, record *checkpointRecordSnapshot) {
	out.string(record.uid)

	versions := make([]*resourceVersion, 0, len(record.versions))
	var last *resourceVersion
	for i := range record.versions {
		version := &record.versions[i]
		if last != nil && checkpointEquivalentState(*last, *version) {
			continue
		}
		versions = append(versions, version)
		last = version
	}

	out.uvarint(uint64(len(versions)))
	var previous *resourceVersion
	for _, version := range versions {
		out.string(version.eventID)
		if previous == nil {
			out.varint(version.timestamp)
		} else {
			out.varint(version.timestamp - previous.timestamp)
		}
		out.byte(version.typeCode)
		out.byte(version.statusCode)
		out.byte(version.flags)
		if previous != nil && version.meta == previous.meta {
			out.byte(checkpointV3SameAsPrevious)
		} else {
			meta := version.metaOrEmpty()
			out.byte(checkpointV3New)
			out.string(meta.UID)
			out.string(meta.Kind)
			out.string(meta.APIGroup)
			out.string(meta.Version)
			out.string(meta.Namespace)
			out.string(meta.Name)
			out.varint(meta.FirstSeen)
		}
		var previousLabels map[string]string
		if previous != nil {
			previousLabels = previous.labels
		}
		out.labels(version.labels, previousLabels, previous == nil)
		out.uint64(version.dataHash)
		encodeCheckpointPayloadV3(out, version)
		previous = version
	}

	latest := record.latest
	var flags byte
	if latest.known {
		flags |= checkpointV3StateKnown
	}
	if latest.hasData {
		flags |= checkpointV3StateHasData
	}
	if latest.fingerprint.valid {
		flags |= checkpointV3StateFPValid
	}
	out.byte(flags)
	if latest.fingerprint.valid {
		out.uint64(latest.fingerprint.spec)
		out.uint64(latest.fingerprint.status)
		out.uint64(latest.fingerprint.kindSpecific)
	}
	var lastLabels map[string]string
	if previous != nil {
		lastLabels = previous.labels
	}
	out.labels(latest.labels, lastLabels, false)
}

func encodeCheckpointPayloadV3(out *checkpointV3Writer, version *resourceVersion) {
	if len(version.data) > 0 {
		out.byte(checkpointV3PayloadInline)
		out.blob(version.data)
		return
	}
	if !version.ref.valid() {
		out.byte(checkpointV3PayloadNone)
		return
	}
	version.ref.segment.mu.Lock()
	index := version.ref.segment.checkpointIndex
	version.ref.segment.mu.Unlock()
	if index == 0 {
		// The segment appeared after the snapshot was taken; inline the payload.
		out.byte(checkpointV3PayloadInline)
		out.blob(versionPayload(version))
		return
	}
	out.byte(checkpointV3PayloadRef)
	out.uvarint(uint64(index))
	out.uvarint(uint64(version.ref.offset))
	out.uvarint(uint64(version.ref.length))
}

func encodeCheckpointK8sEventsV3(w *bufio.Writer, snapshot *checkpointSnapshot) error {
	out := &checkpointV3Writer{w: w}
	out.string(checkpointV3Magic)
	uids := make([]string, 0, len(snapshot.k8sEvents)+len(snapshot.k8sTruncated))
	for uid := range snapshot.k8sEvents {
		uids = append(uids, uid)
	}
	for uid := range snapshot.k8sTruncated {
		if _, ok := snapshot.k8sEvents[uid]; !ok {
			uids = append(uids, uid)
		}
	}
	sortStrings(uids)
	out.uvarint(uint64(len(uids)))
	for _, uid := range uids {
		out.string(uid)
		out.varint(snapshot.k8sTruncated[uid])
		events := snapshot.k8sEvents[uid]
		out.uvarint(uint64(len(events)))
		for i := range events {
			event := &events[i]
			out.string(event.EventID)
			out.varint(event.Timestamp.UnixNano())
			out.string(event.Reason)
			out.string(event.Message)
			out.string(event.Type)
			out.string(event.Source)
			out.varint(int64(event.Count))
		}
	}
	return out.err
}

type checkpointV3Reader struct {
	r   *bufio.Reader
	buf []byte
	err error
}

func (c *checkpointV3Reader) fail(err error) {
	if c.err == nil {
		c.err = err
	}
}

func (c *checkpointV3Reader) uvarint() uint64 {
	if c.err != nil {
		return 0
	}
	value, err := binary.ReadUvarint(c.r)
	if err != nil {
		c.fail(err)
	}
	return value
}

func (c *checkpointV3Reader) varint() int64 {
	if c.err != nil {
		return 0
	}
	value, err := binary.ReadVarint(c.r)
	if err != nil {
		c.fail(err)
	}
	return value
}

func (c *checkpointV3Reader) byte() byte {
	if c.err != nil {
		return 0
	}
	value, err := c.r.ReadByte()
	if err != nil {
		c.fail(err)
	}
	return value
}

func (c *checkpointV3Reader) uint64() uint64 {
	if c.err != nil {
		return 0
	}
	var raw [8]byte
	if _, err := io.ReadFull(c.r, raw[:]); err != nil {
		c.fail(err)
		return 0
	}
	return binary.LittleEndian.Uint64(raw[:])
}

func (c *checkpointV3Reader) blob() []byte {
	length := c.uvarint()
	if c.err != nil || length == 0 {
		return nil
	}
	if length > math.MaxInt32 {
		c.fail(fmt.Errorf("checkpoint field length %d too large", length))
		return nil
	}
	value := make([]byte, length)
	if _, err := io.ReadFull(c.r, value); err != nil {
		c.fail(err)
		return nil
	}
	return value
}

func (c *checkpointV3Reader) string() string {
	return string(c.blob())
}

func (c *checkpointV3Reader) stringList() []string {
	count := c.uvarint()
	if c.err != nil {
		return nil
	}
	values := make([]string, 0, min(count, 4096))
	for i := uint64(0); i < count && c.err == nil; i++ {
		values = append(values, c.string())
	}
	return values
}

func (c *checkpointV3Reader) labels(previous map[string]string) map[string]string {
	switch c.byte() {
	case checkpointV3SameAsPrevious:
		return previous
	case checkpointV3Nil:
		return nil
	case checkpointV3New:
		count := c.uvarint()
		labels := make(map[string]string, min(count, 256))
		for i := uint64(0); i < count && c.err == nil; i++ {
			key := c.string()
			labels[key] = c.string()
		}
		if previous != nil && stringMapsEqual(labels, previous) {
			return previous
		}
		return labels
	default:
		c.fail(fmt.Errorf("invalid labels marker"))
		return nil
	}
}

func (c *checkpointV3Reader) magic() {
	if magic := c.string(); c.err == nil && magic != checkpointV3Magic {
		c.fail(fmt.Errorf("unexpected checkpoint magic %q", magic))
	}
}

func loadCheckpointV3(state checkpointState, checkpointDir string) (*Projection, error) {
	resources, err := openCheckpointStream(filepath.Join(checkpointDir, checkpointResourcesFileV3))
	if err != nil {
		return nil, fmt.Errorf("open resources stream: %w", err)
	}
	defer func() {
		_ = resources.Close()
	}()
	k8s, err := openCheckpointStream(filepath.Join(checkpointDir, checkpointK8sEventsFileV3))
	if err != nil {
		return nil, fmt.Errorf("open k8s events stream: %w", err)
	}
	defer func() {
		_ = k8s.Close()
	}()

	projection := NewProjection()
	projection.minTimestampNs = state.MinTimestampNs
	projection.maxTimestampNs = state.MaxTimestampNs
	if err := decodeCheckpointResourcesV3(bufio.NewReaderSize(resources, 1<<20), projection, state.SegmentTable); err != nil {
		return nil, fmt.Errorf("decode resources: %w", err)
	}
	if err := decodeCheckpointK8sEventsV3(bufio.NewReaderSize(k8s, 1<<20), projection); err != nil {
		return nil, fmt.Errorf("decode k8s events: %w", err)
	}
	return projection, nil
}

func decodeCheckpointResourcesV3(r *bufio.Reader, projection *Projection, segmentTable []string) error {
	in := &checkpointV3Reader{r: r}
	in.magic()
	fileEventTypes := in.stringList()
	fileStatuses := in.stringList()
	if in.err != nil {
		return in.err
	}
	typeCodes := make([]uint8, len(fileEventTypes))
	for i, value := range fileEventTypes {
		typeCodes[i] = versionEventTypes.code(value)
	}
	statusCodes := make([]uint8, len(fileStatuses))
	for i, value := range fileStatuses {
		statusCodes[i] = versionStatuses.code(value)
	}
	remap := func(codes []uint8, code byte) uint8 {
		if int(code) < len(codes) {
			return codes[code]
		}
		return 0
	}
	resolveSegment := projection.checkpointSegmentResolver(segmentTable)

	recordCount := in.uvarint()
	for i := uint64(0); i < recordCount && in.err == nil; i++ {
		uid := in.string()
		versionCount := in.uvarint()
		if in.err != nil {
			break
		}
		record := &resourceRecord{uid: uid, versions: make([]resourceVersion, 0, min(versionCount, 1<<16))}
		var previous *resourceVersion
		for j := uint64(0); j < versionCount && in.err == nil; j++ {
			version := resourceVersion{eventID: in.string()}
			delta := in.varint()
			if previous == nil {
				version.timestamp = delta
			} else {
				version.timestamp = previous.timestamp + delta
			}
			version.typeCode = remap(typeCodes, in.byte())
			version.statusCode = remap(statusCodes, in.byte())
			version.flags = in.byte()
			switch in.byte() {
			case checkpointV3SameAsPrevious:
				if previous != nil {
					version.meta = previous.meta
				}
			case checkpointV3New:
				version.meta = &versionMeta{
					UID:       in.string(),
					Kind:      in.string(),
					APIGroup:  in.string(),
					Version:   in.string(),
					Namespace: in.string(),
					Name:      in.string(),
					FirstSeen: in.varint(),
				}
			default:
				in.fail(fmt.Errorf("invalid meta marker"))
			}
			var previousLabels map[string]string
			if previous != nil {
				previousLabels = previous.labels
			}
			version.labels = in.labels(previousLabels)
			version.dataHash = in.uint64()
			switch in.byte() {
			case checkpointV3PayloadNone:
			case checkpointV3PayloadInline:
				version.data = in.blob()
			case checkpointV3PayloadRef:
				index := in.uvarint()
				offset := in.uvarint()
				length := in.uvarint()
				if handle := resolveSegment(uint32(index)); handle != nil {
					version.ref = versionDataRef{segment: handle, offset: int64(offset), length: uint32(length)}
				}
			default:
				in.fail(fmt.Errorf("invalid payload marker"))
			}
			record.versions = append(record.versions, version)
			previous = &record.versions[len(record.versions)-1]
		}

		flags := in.byte()
		latest := recordChangeState{
			known:   flags&checkpointV3StateKnown != 0,
			hasData: flags&checkpointV3StateHasData != 0,
		}
		if flags&checkpointV3StateFPValid != 0 {
			latest.fingerprint = changeFingerprint{valid: true, spec: in.uint64(), status: in.uint64(), kindSpecific: in.uint64()}
		}
		var lastLabels map[string]string
		if previous != nil {
			lastLabels = previous.labels
		}
		latest.labels = in.labels(lastLabels)
		record.latest = latest
		if in.err == nil && len(record.versions) > 0 {
			restoreProjectionResourceRecord(projection, record)
		}
	}
	if errors.Is(in.err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return in.err
}

func decodeCheckpointK8sEventsV3(r *bufio.Reader, projection *Projection) error {
	in := &checkpointV3Reader{r: r}
	in.magic()
	count := in.uvarint()
	for i := uint64(0); i < count && in.err == nil; i++ {
		uid := in.string()
		truncated := in.varint()
		eventCount := in.uvarint()
		if in.err != nil {
			break
		}
		events := make([]analysisstore.K8sEventInfo, 0, min(eventCount, maxK8sEventInfosPerUID))
		for j := uint64(0); j < eventCount && in.err == nil; j++ {
			events = append(events, analysisstore.K8sEventInfo{
				EventID:   in.string(),
				Timestamp: time.Unix(0, in.varint()).UTC(),
				Reason:    in.string(),
				Message:   in.string(),
				Type:      in.string(),
				Source:    in.string(),
				Count:     int(in.varint()),
			})
		}
		if truncated != 0 {
			projection.k8sEventsTruncatedThrough[uid] = truncated
		}
		if len(events) > 0 {
			projection.k8sEventsByInvolvedUID[uid] = projection.capK8sEventInfos(uid, events)
		}
	}
	if errors.Is(in.err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return in.err
}

func sortStrings(values []string) {
	sort.Strings(values)
}

// mapIdentity returns the address of a map's header, identifying shared instances.
func mapIdentity(m map[string]string) uintptr {
	return reflect.ValueOf(m).Pointer()
}
