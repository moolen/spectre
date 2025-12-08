package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/models"
)

// Block represents a fixed-size unit of compressed event data with associated metadata
type Block struct {
	// ID is the sequential block number within the file (0-based)
	ID int32

	// Offset is the byte offset in file where block starts
	Offset int64

	// Length is the byte length of compressed data
	Length int64

	// UncompressedLength is the byte length before compression
	UncompressedLength int64

	// EventCount is the number of events in block
	EventCount int32

	// TimestampMin is the minimum event timestamp in block (nanoseconds)
	TimestampMin int64

	// TimestampMax is the maximum event timestamp in block (nanoseconds)
	TimestampMax int64

	// Metadata contains filtering-relevant information about block contents
	Metadata *BlockMetadata

	// CompressedData contains the actual gzip/zstd compressed event payload
	CompressedData []byte
}

// BlockMetadata tracks filtering-relevant information about block contents
type BlockMetadata struct {
	// ID is the same as Block.ID for reference
	ID int32 `json:"id"`

	// Offset is the byte offset where the block data starts in the file
	Offset int64 `json:"offset"`

	// BloomFilterKinds contains resources.kinds present in block
	BloomFilterKinds *StandardBloomFilter `json:"bloom_filter_kinds,omitempty"`

	// BloomFilterNamespaces contains namespaces present in block
	BloomFilterNamespaces *StandardBloomFilter `json:"bloom_filter_namespaces,omitempty"`

	// BloomFilterGroups contains resource groups present in block
	BloomFilterGroups *StandardBloomFilter `json:"bloom_filter_groups,omitempty"`

	// KindSet is the exact set of unique kinds in block
	KindSet []string `json:"kind_set,omitempty"`

	// NamespaceSet is the exact set of unique namespaces in block
	NamespaceSet []string `json:"namespace_set,omitempty"`

	// GroupSet is the exact set of unique groups in block
	GroupSet []string `json:"group_set,omitempty"`

	// Checksum is CRC32 hex-encoded if enabled, empty if disabled
	Checksum string `json:"checksum,omitempty"`

	// TimestampMin stores minimum timestamp for index
	TimestampMin int64 `json:"timestamp_min"`

	// TimestampMax stores maximum timestamp for index
	TimestampMax int64 `json:"timestamp_max"`

	// EventCount stores event count for index
	EventCount int32 `json:"event_count"`

	// CompressedLength stores compressed size for statistics
	CompressedLength int64 `json:"compressed_length"`

	// UncompressedLength stores uncompressed size for statistics
	UncompressedLength int64 `json:"uncompressed_length"`
}

// EventBuffer accumulates events for a block until it's full
type EventBuffer struct {
	// events stores the buffered events in JSON format
	events [][]byte

	// metadata tracks information about block contents
	metadata *BlockMetadata

	// blockSize is the maximum uncompressed block size
	blockSize int64

	// currentSize tracks current uncompressed size
	currentSize int64

	// timestampMin and timestampMax track time bounds
	timestampMin int64
	timestampMax int64

	// kindSet, namespaceSet, groupSet track unique values
	kindSet      map[string]bool
	namespaceSet map[string]bool
	groupSet     map[string]bool

	// bloomFilters for efficient filtering
	bloomKinds      *StandardBloomFilter
	bloomNamespaces *StandardBloomFilter
	bloomGroups     *StandardBloomFilter
}

// NewEventBuffer creates a new event buffer for accumulating events into a block
func NewEventBuffer(blockSizeBytes int64) *EventBuffer {
	return &EventBuffer{
		events:          make([][]byte, 0),
		metadata:        &BlockMetadata{},
		blockSize:       blockSizeBytes,
		currentSize:     0,
		timestampMin:    0,
		timestampMax:    0,
		kindSet:         make(map[string]bool),
		namespaceSet:    make(map[string]bool),
		groupSet:        make(map[string]bool),
		bloomKinds:      NewBloomFilter(1000, 0.05),
		bloomNamespaces: NewBloomFilter(100, 0.05),
		bloomGroups:     NewBloomFilter(100, 0.05),
	}
}

// AddEvent adds an event to the buffer if it fits within block size
// Returns true if event was added, false if block would exceed size limit
func (eb *EventBuffer) AddEvent(eventJSON []byte) bool {
	eventSize := int64(len(eventJSON))

	// Check if adding this event would exceed block size
	if eb.currentSize+eventSize > eb.blockSize && len(eb.events) > 0 {
		return false // Block is full
	}

	eb.events = append(eb.events, eventJSON)
	eb.currentSize += eventSize

	// Parse event to extract metadata
	// This is a simplified version; in production, would use proper event struct
	var eventData map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventData); err == nil {
		// Extract timestamp
		if ts, ok := eventData["timestamp"].(float64); ok {
			tsNs := int64(ts)
			if eb.timestampMin == 0 || tsNs < eb.timestampMin {
				eb.timestampMin = tsNs
			}
			if tsNs > eb.timestampMax {
				eb.timestampMax = tsNs
			}
		}

		// Extract resource information
		if resource, ok := eventData["resource"].(map[string]interface{}); ok {
			if kind, ok := resource["kind"].(string); ok {
				eb.kindSet[kind] = true
				eb.bloomKinds.Add(kind)
			}
			if ns, ok := resource["namespace"].(string); ok {
				eb.namespaceSet[ns] = true
				eb.bloomNamespaces.Add(ns)
			}
			if group, ok := resource["group"].(string); ok {
				eb.groupSet[group] = true
				eb.bloomGroups.Add(group)
			}
		}
	}

	return true
}

// IsFull returns true if adding another event would exceed block size
func (eb *EventBuffer) IsFull(nextEventSize int64) bool {
	if len(eb.events) == 0 {
		return false // Never full on first event
	}
	return eb.currentSize+nextEventSize > eb.blockSize
}

// GetEventCount returns the number of events in the buffer
func (eb *EventBuffer) GetEventCount() int32 {
	return int32(len(eb.events))
}

// GetCurrentSize returns the current uncompressed size
func (eb *EventBuffer) GetCurrentSize() int64 {
	return eb.currentSize
}

// GetEvents parses and returns all buffered events as Event objects
func (eb *EventBuffer) GetEvents() ([]*models.Event, error) {
	var events []*models.Event
	for _, eventJSON := range eb.events {
		var event models.Event
		if err := json.Unmarshal(eventJSON, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal buffered event: %w", err)
		}
		events = append(events, &event)
	}
	return events, nil
}

// Finalize creates a Block from the buffered events and compresses it
// Events are encoded using protobuf format
func (eb *EventBuffer) Finalize(blockID int32, compressionAlgorithm string) (*Block, error) {
	if len(eb.events) == 0 {
		return nil, fmt.Errorf("cannot finalize empty event buffer")
	}

	// Encode events using protobuf format
	uncompressedData, err := eb.encodeProtobuf()
	if err != nil {
		return nil, err
	}

	// Create metadata
	metadata := &BlockMetadata{
		ID:                    blockID,
		BloomFilterKinds:      eb.bloomKinds,
		BloomFilterNamespaces: eb.bloomNamespaces,
		BloomFilterGroups:     eb.bloomGroups,
		KindSet:               mapToSlice(eb.kindSet),
		NamespaceSet:          mapToSlice(eb.namespaceSet),
		GroupSet:              mapToSlice(eb.groupSet),
		TimestampMin:          eb.timestampMin,
		TimestampMax:          eb.timestampMax,
		EventCount:            int32(len(eb.events)),
		UncompressedLength:    int64(len(uncompressedData)),
	}

	// Create block with uncompressed data
	block := &Block{
		ID:                 blockID,
		EventCount:         int32(len(eb.events)),
		TimestampMin:       eb.timestampMin,
		TimestampMax:       eb.timestampMax,
		UncompressedLength: int64(len(uncompressedData)),
		CompressedData:     uncompressedData, // Will be compressed in next step
		Metadata:           metadata,
	}

	return block, nil
}

// encodeProtobuf encodes events as length-prefixed protobuf messages
func (eb *EventBuffer) encodeProtobuf() ([]byte, error) {
	var buf bytes.Buffer

	for _, eventJSON := range eb.events {
		// Unmarshal JSON to Event
		event := &models.Event{}
		if err := json.Unmarshal(eventJSON, event); err != nil {
			return nil, fmt.Errorf("failed to parse event JSON: %w", err)
		}

		// Marshal to protobuf
		pbData, err := event.MarshalProtobuf()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
		}

		// Write length-prefixed message using varint encoding
		varint := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(varint, uint64(len(pbData)))
		if _, err := buf.Write(varint[:n]); err != nil {
			return nil, err
		}
		if _, err := buf.Write(pbData); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// CompressBlock compresses the block's data using gzip compression
func CompressBlock(block *Block) (*Block, error) {
	compressor := NewCompressor()

	compressedData, err := compressor.Compress(block.CompressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to compress block: %w", err)
	}

	// Update block with compressed data
	block.Length = int64(len(compressedData))
	block.CompressedData = compressedData

	// Update metadata with compressed length
	if block.Metadata != nil {
		block.Metadata.CompressedLength = block.Length
	}

	return block, nil
}

// DecompressBlock decompresses the block's data using gzip decompression
func DecompressBlock(block *Block) ([]byte, error) {
	compressor := NewCompressor()

	decompressedData, err := compressor.Decompress(block.CompressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress block: %w", err)
	}

	return decompressedData, nil
}

// mapToSlice converts a map[string]bool to a sorted string slice
func mapToSlice(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
