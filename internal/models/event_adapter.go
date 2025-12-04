package models

import (
	"encoding/json"
	"fmt"

	pb "github.com/moolen/spectre/internal/models/pb"
	"google.golang.org/protobuf/proto"
)

// MarshalProtobuf serializes Event to protobuf binary format
func (e *Event) MarshalProtobuf() ([]byte, error) {
	// Convert Go Event to protobuf Event
	pbEvent := &pb.Event{
		Id:             e.ID,
		Timestamp:      e.Timestamp,
		DataSize:       e.DataSize,
		CompressedSize: e.CompressedSize,
		Data:           e.Data, // json.RawMessage as bytes
	}

	// Set EventType enum
	switch e.Type {
	case EventTypeCreate:
		pbEvent.Type = pb.EventType_CREATE
	case EventTypeUpdate:
		pbEvent.Type = pb.EventType_UPDATE
	case EventTypeDelete:
		pbEvent.Type = pb.EventType_DELETE
	default:
		return nil, fmt.Errorf("unknown event type: %s", e.Type)
	}

	// Convert ResourceMetadata
	if e.Resource.Kind != "" || e.Resource.Version != "" {
		pbEvent.Resource = &pb.ResourceMetadata{
			Group:             e.Resource.Group,
			Version:           e.Resource.Version,
			Kind:              e.Resource.Kind,
			Namespace:         e.Resource.Namespace,
			Name:              e.Resource.Name,
			Uid:               e.Resource.UID,
			InvolvedObjectUid: e.Resource.InvolvedObjectUID,
		}
	}

	// Serialize to protobuf binary
	return proto.Marshal(pbEvent)
}

// UnmarshalProtobuf deserializes Event from protobuf binary format
func (e *Event) UnmarshalProtobuf(data []byte) error {
	pbEvent := &pb.Event{}

	if err := proto.Unmarshal(data, pbEvent); err != nil {
		return fmt.Errorf("failed to unmarshal protobuf event: %w", err)
	}

	// Convert protobuf Event to Go Event
	e.ID = pbEvent.Id
	e.Timestamp = pbEvent.Timestamp
	e.DataSize = pbEvent.DataSize
	e.CompressedSize = pbEvent.CompressedSize
	e.Data = json.RawMessage(pbEvent.Data) // bytes back to RawMessage

	// Convert EventType enum
	switch pbEvent.Type {
	case pb.EventType_CREATE:
		e.Type = EventTypeCreate
	case pb.EventType_UPDATE:
		e.Type = EventTypeUpdate
	case pb.EventType_DELETE:
		e.Type = EventTypeDelete
	default:
		return fmt.Errorf("unknown protobuf event type: %v", pbEvent.Type)
	}

	// Convert ResourceMetadata
	if pbEvent.Resource != nil {
		e.Resource = ResourceMetadata{
			Group:             pbEvent.Resource.Group,
			Version:           pbEvent.Resource.Version,
			Kind:              pbEvent.Resource.Kind,
			Namespace:         pbEvent.Resource.Namespace,
			Name:              pbEvent.Resource.Name,
			UID:               pbEvent.Resource.Uid,
			InvolvedObjectUID: pbEvent.Resource.InvolvedObjectUid,
		}
	}

	return nil
}

// IsMarshalProtobuf returns true if data is protobuf-encoded (heuristic)
// Protobuf messages typically start with field tags like 0x0A (field 1 length-delimited)
// This is a quick heuristic; for production, the EncodingFormat field in file header should be used
func IsMarshalProtobuf(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Protobuf varint-encoded first field (1) would be 0x0A (field number 1, wire type 2=length-delimited)
	// JSON starts with {, [, ", whitespace, digit, or letter
	// This is a conservative check - protobuf will always start with field tags
	firstByte := data[0]
	// Check if it looks like a protobuf field tag
	return firstByte == 0x0A || (firstByte >= 0x08 && firstByte <= 0x0F)
}
