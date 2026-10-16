package embeddedstore

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/moolen/spectre/internal/models"
)

const (
	segmentsDirName               = "segments"
	segmentTempDirName            = "tmp"
	segmentEventsFile             = "events.bin"
	segmentTimeIndexFile          = "time.idx"
	segmentUIDIndexFile           = "resource.idx"
	segmentAssociatedUIDIndexFile = "associated.idx"
	segmentDimIndexFile           = "dim.idx"
	segmentStatsFile              = "stats.json"

	segmentIndexStride          = 32
	maxSegmentRecordSize        = 8 * 1024 * 1024 // 8 MiB
	segmentRecordCompressedFlag = uint32(1 << 31)
	segmentRecordLengthMask     = ^segmentRecordCompressedFlag
)

var (
	segmentFrameEncoderPool = sync.Pool{New: func() any {
		encoder, err := zstd.NewWriter(nil)
		if err != nil {
			panic(err)
		}
		return encoder
	}}
	segmentFrameDecoderPool = sync.Pool{New: func() any {
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			panic(err)
		}
		return decoder
	}}
)

type segmentBundleMeta struct {
	ID             string              `json:"id"`
	SizeBytes      int64               `json:"size_bytes,omitempty"`
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

type segmentAssociatedIndex struct {
	InvolvedUIDOffsets map[string][]int64 `json:"involved_uid_offsets"`
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
	record := make([]byte, 4, 4+len(event.Data)/2+256)
	record = appendEventRecord(record, segmentRecordMagic, 0, &event, true)
	payloadLen := len(record) - 4
	if payloadLen > maxSegmentRecordSize {
		return nil, fmt.Errorf("encode framed event: payload size %d exceeds max %d", payloadLen, maxSegmentRecordSize)
	}
	binary.BigEndian.PutUint32(record[:4], uint32(payloadLen))
	return record, nil
}

func decodeFramedEvent(reader io.Reader) (models.Event, int64, error) {
	event, _, size, err := decodeFramedEventIf(reader, nil)
	return event, size, err
}

// decodeFramedEventIf reads one framed record and decodes its data only if keep accepts
// the record's metadata. The returned size is the full frame length either way.
func decodeFramedEventIf(reader io.Reader, keep func(*models.Event) bool) (models.Event, bool, int64, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		if err == io.EOF {
			return models.Event{}, false, 0, io.EOF
		}
		if err == io.ErrUnexpectedEOF {
			return models.Event{}, false, 0, fmt.Errorf("decode framed event: truncated header: %w", err)
		}
		return models.Event{}, false, 0, fmt.Errorf("decode framed event: read header: %w", err)
	}

	headerValue := binary.BigEndian.Uint32(header[:])
	compressed := headerValue&segmentRecordCompressedFlag != 0
	payloadLen := headerValue & segmentRecordLengthMask
	if payloadLen > uint32(maxSegmentRecordSize) {
		return models.Event{}, false, 0, fmt.Errorf("decode framed event: payload size %d exceeds max %d", payloadLen, maxSegmentRecordSize)
	}

	payload := make([]byte, int(payloadLen))
	if _, err := io.ReadFull(reader, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return models.Event{}, false, 0, fmt.Errorf("decode framed event: truncated payload: %w", err)
		}
		return models.Event{}, false, 0, fmt.Errorf("decode framed event: read payload: %w", err)
	}

	event, kept, err := decodeFramedPayload(payload, compressed, keep)
	if err != nil {
		return models.Event{}, false, 0, err
	}
	return event, kept, int64(4 + payloadLen), nil
}

func decodeFramedPayload(payload []byte, compressed bool, keep func(*models.Event) bool) (models.Event, bool, error) {
	decodedPayload := payload
	if compressed {
		expanded, err := decompressSegmentFramePayload(payload)
		if err != nil {
			return models.Event{}, false, fmt.Errorf("decode framed event: decompress payload: %w", err)
		}
		decodedPayload = expanded
	}
	if len(decodedPayload) > maxSegmentRecordSize {
		return models.Event{}, false, fmt.Errorf("decode framed event: expanded payload size %d exceeds max %d", len(decodedPayload), maxSegmentRecordSize)
	}

	event, kept, err := decodeEventPayloadIf(decodedPayload, keep)
	if err != nil {
		return models.Event{}, false, fmt.Errorf("decode framed event: corrupt payload: %w", err)
	}
	return event, kept, nil
}

func compressSegmentFramePayload(payload []byte) ([]byte, bool) {
	encoder := segmentFrameEncoderPool.Get().(*zstd.Encoder)
	defer segmentFrameEncoderPool.Put(encoder)

	compressed := encoder.EncodeAll(payload, nil)
	if len(compressed) >= len(payload) {
		return nil, false
	}
	return compressed, true
}

func decompressSegmentFramePayload(payload []byte) ([]byte, error) {
	decoder := segmentFrameDecoderPool.Get().(*zstd.Decoder)
	defer segmentFrameDecoderPool.Put(decoder)
	return decoder.DecodeAll(payload, nil)
}
