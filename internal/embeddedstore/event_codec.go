package embeddedstore

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moolen/spectre/internal/models"
)

// Binary event records replace the JSON encoding used by earlier segment and tail
// journal versions. Readers still accept JSON records: a JSON payload always starts
// with '{', the binary formats start with a magic byte that can never open JSON.
//
// Layout after the magic byte:
//
//	[uvarint high-water mark]            tail records only
//	string ID
//	varint timestamp
//	byte   event type (1 create, 2 update, 3 delete, 0 + string other)
//	string group, version, kind, namespace, name, uid, involvedObjectUID
//	varint dataSize, compressedSize
//	byte   flags (bit0 preExisting)
//	byte   data encoding (0 none, 1 raw, 2 zstd)
//	[uvarint length + bytes]             when encoding != 0
//
// Strings are uvarint length prefixed. Keeping metadata outside the compressed data lets
// scans filter on namespace/kind/time before paying for decompression.
const (
	segmentRecordMagic byte = 0xB1
	tailRecordMagic    byte = 0xB2

	eventTypeCodeOther  byte = 0
	eventTypeCodeCreate byte = 1
	eventTypeCodeUpdate byte = 2
	eventTypeCodeDelete byte = 3

	eventDataNone byte = 0
	eventDataRaw  byte = 1
	eventDataZstd byte = 2

	eventFlagPreExisting byte = 1
)

var errEventCodecTruncated = errors.New("truncated binary event record")

func appendEventRecord(dst []byte, magic byte, highWaterMark uint64, event *models.Event, compressData bool) []byte {
	dst = append(dst, magic)
	if magic == tailRecordMagic {
		dst = binary.AppendUvarint(dst, highWaterMark)
	}
	dst = appendCodecString(dst, event.ID)
	dst = binary.AppendVarint(dst, event.Timestamp)
	switch event.Type {
	case models.EventTypeCreate:
		dst = append(dst, eventTypeCodeCreate)
	case models.EventTypeUpdate:
		dst = append(dst, eventTypeCodeUpdate)
	case models.EventTypeDelete:
		dst = append(dst, eventTypeCodeDelete)
	default:
		dst = append(dst, eventTypeCodeOther)
		dst = appendCodecString(dst, string(event.Type))
	}
	resource := &event.Resource
	dst = appendCodecString(dst, resource.Group)
	dst = appendCodecString(dst, resource.Version)
	dst = appendCodecString(dst, resource.Kind)
	dst = appendCodecString(dst, resource.Namespace)
	dst = appendCodecString(dst, resource.Name)
	dst = appendCodecString(dst, resource.UID)
	dst = appendCodecString(dst, resource.InvolvedObjectUID)
	dst = binary.AppendVarint(dst, int64(event.DataSize))
	dst = binary.AppendVarint(dst, int64(event.CompressedSize))
	var flags byte
	if event.PreExisting {
		flags |= eventFlagPreExisting
	}
	dst = append(dst, flags)

	if len(event.Data) == 0 {
		return append(dst, eventDataNone)
	}
	if compressData {
		if compressed, ok := compressSegmentFramePayload(event.Data); ok {
			dst = append(dst, eventDataZstd)
			dst = binary.AppendUvarint(dst, uint64(len(compressed)))
			return append(dst, compressed...)
		}
	}
	dst = append(dst, eventDataRaw)
	dst = binary.AppendUvarint(dst, uint64(len(event.Data)))
	return append(dst, event.Data...)
}

func appendCodecString(dst []byte, value string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

// eventRecord is a decoded binary record whose payload is decoded on demand.
type eventRecord struct {
	event         models.Event
	highWaterMark uint64
	dataEncoding  byte
	dataBytes     []byte
}

// decodeData materialises event.Data. It copies raw data so the record never aliases
// the read buffer.
func (r *eventRecord) decodeData() error {
	switch r.dataEncoding {
	case eventDataNone:
		r.event.Data = nil
	case eventDataRaw:
		r.event.Data = append(json.RawMessage(nil), r.dataBytes...)
	case eventDataZstd:
		expanded, err := decompressSegmentFramePayload(r.dataBytes)
		if err != nil {
			return fmt.Errorf("decompress event data: %w", err)
		}
		r.event.Data = expanded
	default:
		return fmt.Errorf("unknown event data encoding %d", r.dataEncoding)
	}
	r.dataBytes = nil
	return nil
}

func decodeEventRecordHeader(payload []byte) (eventRecord, error) {
	if len(payload) == 0 {
		return eventRecord{}, errEventCodecTruncated
	}
	magic := payload[0]
	if magic != segmentRecordMagic && magic != tailRecordMagic {
		return eventRecord{}, fmt.Errorf("unknown event record magic 0x%x", magic)
	}
	d := codecDecoder{buf: payload[1:]}
	var record eventRecord
	if magic == tailRecordMagic {
		record.highWaterMark = d.uvarint()
	}
	event := &record.event
	event.ID = d.string()
	event.Timestamp = d.varint()
	switch code := d.byte(); code {
	case eventTypeCodeCreate:
		event.Type = models.EventTypeCreate
	case eventTypeCodeUpdate:
		event.Type = models.EventTypeUpdate
	case eventTypeCodeDelete:
		event.Type = models.EventTypeDelete
	default:
		event.Type = models.EventType(d.string())
	}
	event.Resource.Group = d.string()
	event.Resource.Version = d.string()
	event.Resource.Kind = d.string()
	event.Resource.Namespace = d.string()
	event.Resource.Name = d.string()
	event.Resource.UID = d.string()
	event.Resource.InvolvedObjectUID = d.string()
	event.DataSize = int32(d.varint())
	event.CompressedSize = int32(d.varint())
	event.PreExisting = d.byte()&eventFlagPreExisting != 0
	record.dataEncoding = d.byte()
	if record.dataEncoding != eventDataNone {
		length := d.uvarint()
		record.dataBytes = d.bytes(length)
	}
	if d.err != nil {
		return eventRecord{}, d.err
	}
	return record, nil
}

// decodeEventPayload decodes a full event from either a binary or a legacy JSON payload.
func decodeEventPayload(payload []byte) (models.Event, error) {
	if len(payload) > 0 && payload[0] == '{' {
		var event models.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return models.Event{}, err
		}
		return event, nil
	}
	record, err := decodeEventRecordHeader(payload)
	if err != nil {
		return models.Event{}, err
	}
	if err := record.decodeData(); err != nil {
		return models.Event{}, err
	}
	return record.event, nil
}

// decodeEventPayloadIf decodes the event's data only when keep accepts its metadata.
// Legacy JSON payloads are decoded fully before keep is consulted.
func decodeEventPayloadIf(payload []byte, keep func(*models.Event) bool) (models.Event, bool, error) {
	if len(payload) > 0 && payload[0] == '{' {
		var event models.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return models.Event{}, false, err
		}
		if keep != nil && !keep(&event) {
			return models.Event{}, false, nil
		}
		return event, true, nil
	}
	record, err := decodeEventRecordHeader(payload)
	if err != nil {
		return models.Event{}, false, err
	}
	if keep != nil && !keep(&record.event) {
		return models.Event{}, false, nil
	}
	if err := record.decodeData(); err != nil {
		return models.Event{}, false, err
	}
	return record.event, true, nil
}

type codecDecoder struct {
	buf []byte
	err error
}

func (d *codecDecoder) fail() {
	if d.err == nil {
		d.err = errEventCodecTruncated
	}
	d.buf = nil
}

func (d *codecDecoder) uvarint() uint64 {
	value, n := binary.Uvarint(d.buf)
	if n <= 0 {
		d.fail()
		return 0
	}
	d.buf = d.buf[n:]
	return value
}

func (d *codecDecoder) varint() int64 {
	value, n := binary.Varint(d.buf)
	if n <= 0 {
		d.fail()
		return 0
	}
	d.buf = d.buf[n:]
	return value
}

func (d *codecDecoder) byte() byte {
	if len(d.buf) < 1 {
		d.fail()
		return 0
	}
	value := d.buf[0]
	d.buf = d.buf[1:]
	return value
}

func (d *codecDecoder) bytes(length uint64) []byte {
	if uint64(len(d.buf)) < length {
		d.fail()
		return nil
	}
	value := d.buf[:length:length]
	d.buf = d.buf[length:]
	return value
}

func (d *codecDecoder) string() string {
	length := d.uvarint()
	if length == 0 {
		return ""
	}
	return string(d.bytes(length))
}
