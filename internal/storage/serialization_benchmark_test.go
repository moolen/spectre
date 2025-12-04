package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

// BenchmarkJSONUnmarshal measures JSON deserialization performance
func BenchmarkJSONUnmarshal(b *testing.B) {
	// Create test event
	event := &models.Event{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: 1704067200000000000,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			UID:       "8a6d1c0f-1234-5678-9abc-def012345678",
		},
		Data:           json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`),
		DataSize:       256,
		CompressedSize: 128,
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		b.Fatalf("Failed to marshal event: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var evt models.Event
		if err := json.Unmarshal(eventJSON, &evt); err != nil {
			b.Fatalf("Failed to unmarshal: %v", err)
		}
	}
}

// BenchmarkProtobufUnmarshal measures Protobuf deserialization performance
func BenchmarkProtobufUnmarshal(b *testing.B) {
	// Create test event
	event := &models.Event{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: 1704067200000000000,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			UID:       "8a6d1c0f-1234-5678-9abc-def012345678",
		},
		Data:           json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`),
		DataSize:       256,
		CompressedSize: 128,
	}

	// Marshal to Protobuf
	eventProto, err := event.MarshalProtobuf()
	if err != nil {
		b.Fatalf("Failed to marshal to protobuf: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var evt models.Event
		if err := evt.UnmarshalProtobuf(eventProto); err != nil {
			b.Fatalf("Failed to unmarshal protobuf: %v", err)
		}
	}
}

// BenchmarkJSONMarshal measures JSON serialization performance
func BenchmarkJSONMarshal(b *testing.B) {
	event := &models.Event{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: 1704067200000000000,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			UID:       "8a6d1c0f-1234-5678-9abc-def012345678",
		},
		Data:           json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment"}`),
		DataSize:       256,
		CompressedSize: 128,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(event); err != nil {
			b.Fatalf("Failed to marshal: %v", err)
		}
	}
}

// BenchmarkProtobufMarshal measures Protobuf serialization performance
func BenchmarkProtobufMarshal(b *testing.B) {
	event := &models.Event{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: 1704067200000000000,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			UID:       "8a6d1c0f-1234-5678-9abc-def012345678",
		},
		Data:           json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment"}`),
		DataSize:       256,
		CompressedSize: 128,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := event.MarshalProtobuf(); err != nil {
			b.Fatalf("Failed to marshal: %v", err)
		}
	}
}

// BenchmarkPayloadSize compares size efficiency
func BenchmarkPayloadSize(b *testing.B) {
	event := &models.Event{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Timestamp: 1704067200000000000,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			UID:       "8a6d1c0f-1234-5678-9abc-def012345678",
		},
		Data:           json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment"}`),
		DataSize:       256,
		CompressedSize: 128,
	}

	jsonData, _ := json.Marshal(event)
	protoData, _ := event.MarshalProtobuf()

	b.Logf("JSON size: %d bytes", len(jsonData))
	b.Logf("Protobuf size: %d bytes", len(protoData))
	reduction := float64(len(jsonData)-len(protoData)) / float64(len(jsonData)) * 100
	b.Logf("Size reduction: %.1f%%", reduction)
}

// BenchmarkBlockDeserializationJSON measures end-to-end JSON block deserialization
func BenchmarkBlockDeserializationJSON(b *testing.B) {
	// Create 100 events in JSON format
	var events []*models.Event
	for i := 0; i < 100; i++ {
		events = append(events, &models.Event{
			ID:        "evt-" + string(rune(i)),
			Timestamp: 1704067200000000000 + int64(i*1000),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-" + string(rune(i)),
			},
			Data: json.RawMessage(`{"spec":{"containers":[{"name":"app"}]}}`),
		})
	}

	// Encode as NDJSON (current format)
	var ndjson []byte
	for _, evt := range events {
		data, _ := json.Marshal(evt)
		ndjson = append(ndjson, data...)
		ndjson = append(ndjson, '\n')
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result []*models.Event
		lines := bytes.Split(ndjson, []byte("\n"))
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			evt := &models.Event{}
			json.Unmarshal(line, evt)
			result = append(result, evt)
		}
	}
}

func BenchmarkBlockDeserializationProtobuf(b *testing.B) {
	// Create 100 events, marshal to protobuf
	var events []*models.Event
	for i := 0; i < 100; i++ {
		events = append(events, &models.Event{
			ID:        "evt-" + string(rune(i)),
			Timestamp: 1704067200000000000 + int64(i*1000),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-" + string(rune(i)),
			},
			Data: json.RawMessage(`{"spec":{"containers":[{"name":"app"}]}}`),
		})
	}

	// Encode as length-prefixed protobuf
	var protoBuf bytes.Buffer
	for _, evt := range events {
		data, _ := evt.MarshalProtobuf()
		varint := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(varint, uint64(len(data)))
		protoBuf.Write(varint[:n])
		protoBuf.Write(data)
	}

	blockData := protoBuf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result []*models.Event
		offset := 0
		for offset < len(blockData) {
			length, n := binary.Uvarint(blockData[offset:])
			if n <= 0 {
				break
			}
			offset += n
			if offset+int(length) > len(blockData) {
				break
			}
			msgData := blockData[offset : offset+int(length)]
			offset += int(length)

			evt := &models.Event{}
			evt.UnmarshalProtobuf(msgData)
			result = append(result, evt)
		}
	}
}
