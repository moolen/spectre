package embeddedstore

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/moolen/spectre/internal/models"
)

const (
	segmentsDirName      = "segments"
	segmentTempDirName   = "tmp"
	segmentEventsFile    = "events.bin"
	segmentTimeIndexFile = "time.idx"
	segmentUIDIndexFile  = "resource.idx"
	segmentDimIndexFile  = "dim.idx"
	segmentStatsFile     = "stats.json"

	segmentIndexStride   = 32
	maxSegmentRecordSize = 8 * 1024 * 1024 // 8 MiB
)

type segmentBundleMeta struct {
	ID             string              `json:"id"`
	EventCount     int                 `json:"event_count"`
	MinTimestamp   int64               `json:"min_timestamp"`
	MaxTimestamp   int64               `json:"max_timestamp"`
	NamespaceKinds map[string][]string `json:"namespace_kinds"`
}

func (m segmentBundleMeta) MayContain(namespace, kind string) bool {
	kinds, ok := m.NamespaceKinds[namespace]
	if !ok {
		return false
	}

	for i := range kinds {
		if kinds[i] == kind {
			return true
		}
	}

	return false
}

type segmentTimeIndexEntry struct {
	Timestamp int64
	Offset    int64
}

type segmentResourceIndex struct {
	UIDOffsets map[string][]int64 `json:"uid_offsets"`
}

type segmentDimensionEntry struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
}

type segmentDimensionIndex struct {
	Entries []segmentDimensionEntry `json:"entries"`
}

func normalizeNamespaceKinds(namespaceKinds map[string][]string) map[string][]string {
	if namespaceKinds == nil {
		return map[string][]string{}
	}

	normalized := make(map[string][]string, len(namespaceKinds))
	for namespace, kinds := range namespaceKinds {
		uniqueKinds := make(map[string]struct{}, len(kinds))
		for i := range kinds {
			uniqueKinds[kinds[i]] = struct{}{}
		}

		sortedKinds := make([]string, 0, len(uniqueKinds))
		for kind := range uniqueKinds {
			sortedKinds = append(sortedKinds, kind)
		}
		sort.Strings(sortedKinds)
		normalized[namespace] = sortedKinds
	}

	return normalized
}

func encodeFramedEvent(event models.Event) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("encode framed event: marshal event: %w", err)
	}
	if len(payload) > maxSegmentRecordSize {
		return nil, fmt.Errorf("encode framed event: payload size %d exceeds max %d", len(payload), maxSegmentRecordSize)
	}

	record := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(record[:4], uint32(len(payload)))
	copy(record[4:], payload)

	return record, nil
}

func decodeFramedEvent(reader io.Reader) (models.Event, int64, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		if err == io.EOF {
			return models.Event{}, 0, io.EOF
		}
		if err == io.ErrUnexpectedEOF {
			return models.Event{}, 0, fmt.Errorf("decode framed event: truncated header: %w", err)
		}
		return models.Event{}, 0, fmt.Errorf("decode framed event: read header: %w", err)
	}

	payloadLen := binary.BigEndian.Uint32(header[:])
	if payloadLen > uint32(maxSegmentRecordSize) {
		return models.Event{}, 0, fmt.Errorf("decode framed event: payload size %d exceeds max %d", payloadLen, maxSegmentRecordSize)
	}

	payload := make([]byte, int(payloadLen))
	if _, err := io.ReadFull(reader, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return models.Event{}, 0, fmt.Errorf("decode framed event: truncated payload: %w", err)
		}
		return models.Event{}, 0, fmt.Errorf("decode framed event: read payload: %w", err)
	}

	var event models.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return models.Event{}, 0, fmt.Errorf("decode framed event: corrupt payload: %w", err)
	}

	return event, int64(4 + payloadLen), nil
}
