package storage

import (
	"bytes"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/storage"
)

func TestFileHeaderSerialization(t *testing.T) {
	header := storage.NewFileHeader()
	header.CompressionAlgorithm = "zstd"
	header.BlockSize = 256 * 1024

	// Serialize
	buf := &bytes.Buffer{}
	err := storage.WriteFileHeader(buf, header)
	if err != nil {
		t.Fatalf("Failed to write file header: %v", err)
	}

	// Verify size
	if buf.Len() != 77 {
		t.Errorf("Expected header size 77, got %d", buf.Len())
	}

	// Deserialize
	buf2 := bytes.NewReader(buf.Bytes())
	header2, err := storage.ReadFileHeader(buf2)
	if err != nil {
		t.Fatalf("Failed to read file header: %v", err)
	}

	// Verify fields
	if header2.MagicBytes != "RPKBLOCK" {
		t.Errorf("Expected magic RPKBLOCK, got %s", header2.MagicBytes)
	}
	if header2.FormatVersion != "1.0" {
		t.Errorf("Expected version 1.0, got %s", header2.FormatVersion)
	}
	if header2.CompressionAlgorithm != "zstd" {
		t.Errorf("Expected zstd, got %s", header2.CompressionAlgorithm)
	}
	if header2.BlockSize != 256*1024 {
		t.Errorf("Expected block size 262144, got %d", header2.BlockSize)
	}
}

func TestFileHeaderMagicValidation(t *testing.T) {
	// Create header with invalid magic
	buf := make([]byte, 77)
	copy(buf[0:8], "INVALID!")

	// Try to read - should fail
	reader := bytes.NewReader(buf)
	_, err := storage.ReadFileHeader(reader)
	if err == nil {
		t.Error("Expected error for invalid magic bytes")
	}
}

func TestFileFooterSerialization(t *testing.T) {
	footer := &storage.FileFooter{
		IndexSectionOffset: 1000000,
		IndexSectionLength: 50000,
		Checksum:           "abc123def456",
		MagicBytes:         "RPKEND",
	}

	// Serialize
	buf := &bytes.Buffer{}
	err := storage.WriteFileFooter(buf, footer)
	if err != nil {
		t.Fatalf("Failed to write file footer: %v", err)
	}

	// Verify size
	if buf.Len() != 324 {
		t.Errorf("Expected footer size 324, got %d", buf.Len())
	}

	// Deserialize
	buf2 := bytes.NewReader(buf.Bytes())
	footer2, err := storage.ReadFileFooter(buf2)
	if err != nil {
		t.Fatalf("Failed to read file footer: %v", err)
	}

	// Verify fields
	if footer2.IndexSectionOffset != 1000000 {
		t.Errorf("Expected offset 1000000, got %d", footer2.IndexSectionOffset)
	}
	if footer2.IndexSectionLength != 50000 {
		t.Errorf("Expected length 50000, got %d", footer2.IndexSectionLength)
	}
	if footer2.Checksum != "abc123def456" {
		t.Errorf("Expected checksum abc123def456, got %s", footer2.Checksum)
	}
	if footer2.MagicBytes != "RPKEND" {
		t.Errorf("Expected magic RPKEND, got %s", footer2.MagicBytes)
	}
}

func TestFileFooterMagicValidation(t *testing.T) {
	// Create footer with invalid magic
	buf := make([]byte, 324)
	copy(buf[316:324], "INVALID!!") // Footer magic is at the end

	// Try to read - should fail
	reader := bytes.NewReader(buf)
	_, err := storage.ReadFileFooter(reader)
	if err == nil {
		t.Error("Expected error for invalid magic bytes")
	}
}

func TestBlockMetadataCreation(t *testing.T) {
	metadata := &storage.BlockMetadata{
		ID:                 0,
		TimestampMin:       time.Now().UnixNano(),
		TimestampMax:       time.Now().Add(time.Hour).UnixNano(),
		EventCount:         100,
		Checksum:           "abc123",
		KindSet:            []string{"Pod", "Deployment"},
		NamespaceSet:       []string{"default", "kube-system"},
		GroupSet:           []string{"apps", "core"},
		CompressedLength:   60000,
		UncompressedLength: 256000,
	}

	if metadata.ID != 0 {
		t.Error("Block ID not set correctly")
	}
	if metadata.EventCount != 100 {
		t.Error("Event count not set correctly")
	}
	if len(metadata.KindSet) != 2 {
		t.Error("Kind set not initialized correctly")
	}
}

func TestEventBufferBasics(t *testing.T) {
	eb := storage.NewEventBuffer(256 * 1024)

	if eb.GetEventCount() != 0 {
		t.Error("Expected 0 events initially")
	}

	// Add an event
	event := []byte(`{"id":"evt-1","timestamp":1000,"resource":{"kind":"Pod","namespace":"default"},"data":{}}`)
	ok := eb.AddEvent(event)
	if !ok {
		t.Error("Failed to add first event")
	}

	if eb.GetEventCount() != 1 {
		t.Error("Expected 1 event after add")
	}
}

func TestEventBufferFull(t *testing.T) {
	blockSize := int64(1024) // Small block size for testing
	eb := storage.NewEventBuffer(blockSize)

	// Add events until full
	eventCount := 0
	for {
		event := []byte(`{"id":"evt-x","timestamp":1000,"resource":{"kind":"Pod","namespace":"default"},"data":{}}`)
		if !eb.AddEvent(event) {
			break
		}
		eventCount++
	}

	if eventCount == 0 {
		t.Error("Expected at least one event in buffer")
	}

	// Current size should be close to block size
	currentSize := eb.GetCurrentSize()
	if currentSize > blockSize {
		t.Errorf("Buffer size %d exceeds limit %d", currentSize, blockSize)
	}
}

func TestEventBufferMetadata(t *testing.T) {
	eb := storage.NewEventBuffer(256 * 1024)

	events := []string{
		`{"id":"evt-1","timestamp":1000,"resource":{"kind":"Pod","namespace":"default","group":""},"data":{}}`,
		`{"id":"evt-2","timestamp":2000,"resource":{"kind":"Deployment","namespace":"default","group":"apps"},"data":{}}`,
		`{"id":"evt-3","timestamp":3000,"resource":{"kind":"Pod","namespace":"kube-system","group":""},"data":{}}`,
	}

	for _, evt := range events {
		eb.AddEvent([]byte(evt))
	}

	if eb.GetEventCount() != 3 {
		t.Errorf("Expected 3 events, got %d", eb.GetEventCount())
	}
}

func TestInvertedIndexBuilding(t *testing.T) {
	metadata := []*storage.BlockMetadata{
		{
			ID:           0,
			KindSet:      []string{"Pod", "Deployment"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{"apps", ""},
		},
		{
			ID:           1,
			KindSet:      []string{"Service"},
			NamespaceSet: []string{"default", "kube-system"},
			GroupSet:     []string{""},
		},
		{
			ID:           2,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"kube-system"},
			GroupSet:     []string{""},
		},
	}

	index := storage.BuildInvertedIndexes(metadata)

	// Check kind index
	if len(index.KindToBlocks) != 3 {
		t.Errorf("Expected 3 kinds, got %d", len(index.KindToBlocks))
	}

	// Pod should be in blocks 0 and 2
	podBlocks := index.KindToBlocks["Pod"]
	if len(podBlocks) != 2 {
		t.Errorf("Expected Pod in 2 blocks, got %d", len(podBlocks))
	}

	// Check namespace index
	defaultBlocks := index.NamespaceToBlocks["default"]
	if len(defaultBlocks) != 2 {
		t.Errorf("Expected 2 blocks with 'default', got %d", len(defaultBlocks))
	}
}

func TestGetCandidateBlocksWithFilters(t *testing.T) {
	metadata := []*storage.BlockMetadata{
		{
			ID:           0,
			KindSet:      []string{"Pod", "Deployment"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{"apps"},
		},
		{
			ID:           1,
			KindSet:      []string{"Service"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{""},
		},
		{
			ID:           2,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"kube-system"},
			GroupSet:     []string{""},
		},
	}

	index := storage.BuildInvertedIndexes(metadata)

	// Query: kind=Pod AND namespace=default
	filters := map[string]string{
		"kind":      "Pod",
		"namespace": "default",
	}

	candidates := storage.GetCandidateBlocks(index, filters)
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate block, got %d", len(candidates))
	}
	if candidates[0] != 0 {
		t.Errorf("Expected block 0, got %d", candidates[0])
	}

	// Query: kind=Service AND namespace=default
	filters2 := map[string]string{
		"kind":      "Service",
		"namespace": "default",
	}
	candidates2 := storage.GetCandidateBlocks(index, filters2)
	if len(candidates2) != 1 {
		t.Errorf("Expected 1 candidate block, got %d", len(candidates2))
	}
	if candidates2[0] != 1 {
		t.Errorf("Expected block 1, got %d", candidates2[0])
	}

	// Query: kind=Pod AND namespace=kube-system
	filters3 := map[string]string{
		"kind":      "Pod",
		"namespace": "kube-system",
	}
	candidates3 := storage.GetCandidateBlocks(index, filters3)
	if len(candidates3) != 1 {
		t.Errorf("Expected 1 candidate block, got %d", len(candidates3))
	}
	if candidates3[0] != 2 {
		t.Errorf("Expected block 2, got %d", candidates3[0])
	}

	// Query: kind=Deployment AND namespace=default (only in block 0)
	filters4 := map[string]string{
		"kind":      "Deployment",
		"namespace": "default",
	}
	candidates4 := storage.GetCandidateBlocks(index, filters4)
	if len(candidates4) != 1 {
		t.Errorf("Expected 1 candidate block, got %d", len(candidates4))
	}
	if candidates4[0] != 0 {
		t.Errorf("Expected block 0, got %d", candidates4[0])
	}
}

func TestGetCandidateBlocksNoMatch(t *testing.T) {
	metadata := []*storage.BlockMetadata{
		{
			ID:           0,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"default"},
		},
	}

	index := storage.BuildInvertedIndexes(metadata)

	// Query for something that doesn't exist
	filters := map[string]string{
		"kind":      "Service",
		"namespace": "default",
	}

	candidates := storage.GetCandidateBlocks(index, filters)
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(candidates))
	}
}

func TestFileHeaderAllFields(t *testing.T) {
	header := storage.NewFileHeader()
	header.FormatVersion = "1.0"
	header.CreatedAt = 1234567890000000000
	header.CompressionAlgorithm = "gzip"
	header.BlockSize = 128 * 1024
	header.EncodingFormat = "json"
	header.ChecksumEnabled = true
	copy(header.Reserved[:], []byte("reserved123456"))

	buf := &bytes.Buffer{}
	if err := storage.WriteFileHeader(buf, header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	reader := bytes.NewReader(buf.Bytes())
	header2, err := storage.ReadFileHeader(reader)
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if header2.CreatedAt != header.CreatedAt {
		t.Errorf("CreatedAt mismatch: %d vs %d", header2.CreatedAt, header.CreatedAt)
	}
	if header2.EncodingFormat != header.EncodingFormat {
		t.Errorf("EncodingFormat mismatch: %s vs %s", header2.EncodingFormat, header.EncodingFormat)
	}
	if header2.ChecksumEnabled != header.ChecksumEnabled {
		t.Errorf("ChecksumEnabled mismatch: %v vs %v", header2.ChecksumEnabled, header.ChecksumEnabled)
	}
}

func TestFileHeaderEmptyVersion(t *testing.T) {
	header := storage.NewFileHeader()
	header.FormatVersion = ""

	buf := &bytes.Buffer{}
	if err := storage.WriteFileHeader(buf, header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	reader := bytes.NewReader(buf.Bytes())
	header2, err := storage.ReadFileHeader(reader)
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	// Empty version should be read as empty string (trimmed)
	if header2.FormatVersion != "" {
		t.Errorf("Expected empty version, got %s", header2.FormatVersion)
	}
}

func TestFileFooterAllFields(t *testing.T) {
	footer := &storage.FileFooter{
		IndexSectionOffset: 5000000,
		IndexSectionLength: 100000,
		Checksum:           "a" + string(make([]byte, 255)), // 256 bytes
		MagicBytes:         "RPKEND",
	}
	copy(footer.Reserved[:], []byte("reserved data for future use"))

	buf := &bytes.Buffer{}
	if err := storage.WriteFileFooter(buf, footer); err != nil {
		t.Fatalf("Failed to write footer: %v", err)
	}

	reader := bytes.NewReader(buf.Bytes())
	footer2, err := storage.ReadFileFooter(reader)
	if err != nil {
		t.Fatalf("Failed to read footer: %v", err)
	}

	if footer2.IndexSectionOffset != footer.IndexSectionOffset {
		t.Errorf("IndexSectionOffset mismatch: %d vs %d", footer2.IndexSectionOffset, footer.IndexSectionOffset)
	}
	if footer2.IndexSectionLength != footer.IndexSectionLength {
		t.Errorf("IndexSectionLength mismatch: %d vs %d", footer2.IndexSectionLength, footer.IndexSectionLength)
	}
}

func TestIndexSectionSerialization(t *testing.T) {
	section := &storage.IndexSection{
		FormatVersion: "1.0",
		BlockMetadata: []*storage.BlockMetadata{
			{
				ID:                 0,
				TimestampMin:       1000,
				TimestampMax:       2000,
				EventCount:         10,
				CompressedLength:   5000,
				UncompressedLength: 10000,
				KindSet:            []string{"Pod"},
				NamespaceSet:       []string{"default"},
			},
		},
		InvertedIndexes: &storage.InvertedIndex{
			KindToBlocks:      map[string][]int32{"Pod": {0}},
			NamespaceToBlocks: map[string][]int32{"default": {0}},
			GroupToBlocks:     map[string][]int32{},
		},
		Statistics: &storage.IndexStatistics{
			TotalBlocks:            1,
			TotalEvents:            10,
			TotalUncompressedBytes: 10000,
			TotalCompressedBytes:   5000,
			CompressionRatio:       0.5,
			UniqueKinds:            1,
			UniqueNamespaces:       1,
			UniqueGroups:           0,
			TimestampMin:           1000,
			TimestampMax:           2000,
		},
	}

	buf := &bytes.Buffer{}
	bytesWritten, err := storage.WriteIndexSection(buf, section)
	if err != nil {
		t.Fatalf("Failed to write index section: %v", err)
	}

	if bytesWritten <= 0 {
		t.Error("Expected bytes written > 0")
	}

	reader := bytes.NewReader(buf.Bytes())
	section2, err := storage.ReadIndexSection(reader)
	if err != nil {
		t.Fatalf("Failed to read index section: %v", err)
	}

	if section2.FormatVersion != section.FormatVersion {
		t.Errorf("FormatVersion mismatch: %s vs %s", section2.FormatVersion, section.FormatVersion)
	}
	if len(section2.BlockMetadata) != len(section.BlockMetadata) {
		t.Errorf("BlockMetadata count mismatch: %d vs %d", len(section2.BlockMetadata), len(section.BlockMetadata))
	}
	if section2.Statistics.TotalEvents != section.Statistics.TotalEvents {
		t.Errorf("TotalEvents mismatch: %d vs %d", section2.Statistics.TotalEvents, section.Statistics.TotalEvents)
	}
}

func TestBuildInvertedIndexesEmpty(t *testing.T) {
	index := storage.BuildInvertedIndexes([]*storage.BlockMetadata{})
	if index == nil {
		t.Fatal("Expected non-nil index")
	}
	if len(index.KindToBlocks) != 0 {
		t.Errorf("Expected empty KindToBlocks, got %d", len(index.KindToBlocks))
	}
	if len(index.NamespaceToBlocks) != 0 {
		t.Errorf("Expected empty NamespaceToBlocks, got %d", len(index.NamespaceToBlocks))
	}
	if len(index.GroupToBlocks) != 0 {
		t.Errorf("Expected empty GroupToBlocks, got %d", len(index.GroupToBlocks))
	}
}

func TestGetCandidateBlocksEmptyFilters(t *testing.T) {
	metadata := []*storage.BlockMetadata{
		{ID: 0, KindSet: []string{"Pod"}},
		{ID: 1, KindSet: []string{"Service"}},
	}
	index := storage.BuildInvertedIndexes(metadata)

	// Empty filters should return nil
	candidates := storage.GetCandidateBlocks(index, map[string]string{})
	if candidates != nil {
		t.Errorf("Expected nil for empty filters, got %v", candidates)
	}

	// Nil index should return nil
	candidates2 := storage.GetCandidateBlocks(nil, map[string]string{"kind": "Pod"})
	if candidates2 != nil {
		t.Errorf("Expected nil for nil index, got %v", candidates2)
	}
}

func TestGetCandidateBlocksMultipleFilters(t *testing.T) {
	metadata := []*storage.BlockMetadata{
		{
			ID:           0,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{""},
		},
		{
			ID:           1,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{"apps"},
		},
		{
			ID:           2,
			KindSet:      []string{"Pod"},
			NamespaceSet: []string{"kube-system"},
			GroupSet:     []string{""},
		},
	}

	index := storage.BuildInvertedIndexes(metadata)

	// Query: kind=Pod AND namespace=default AND group=""
	filters := map[string]string{
		"kind":      "Pod",
		"namespace": "default",
		"group":     "",
	}

	candidates := storage.GetCandidateBlocks(index, filters)
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate block, got %d", len(candidates))
	}
	if candidates[0] != 0 {
		t.Errorf("Expected block 0, got %d", candidates[0])
	}
}

func TestValidateVersion(t *testing.T) {
	// Valid versions
	validVersions := []string{"1.0", "1.1", "1.2", "1.99"}
	for _, v := range validVersions {
		if err := storage.ValidateVersion(v); err != nil {
			t.Errorf("Version %s should be valid, got error: %v", v, err)
		}
	}

	// Invalid versions
	invalidVersions := []string{"", "2.0", "0.9", "invalid", "1", ".0"}
	for _, v := range invalidVersions {
		if err := storage.ValidateVersion(v); err == nil {
			t.Errorf("Version %s should be invalid, but got no error", v)
		}
	}
}

func TestGetVersionInfo(t *testing.T) {
	// Test v1.0
	info := storage.GetVersionInfo("1.0")
	if info == nil {
		t.Fatal("Expected version info for 1.0")
	}
	if info.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", info.Version)
	}
	if info.Deprecated {
		t.Error("Version 1.0 should not be deprecated")
	}
	if len(info.Features) == 0 {
		t.Error("Expected features for version 1.0")
	}

	// Test v1.1 (future)
	info2 := storage.GetVersionInfo("1.1")
	if info2 == nil {
		t.Fatal("Expected version info for 1.1")
	}

	// Test invalid version
	info3 := storage.GetVersionInfo("2.0")
	if info3 == nil {
		t.Fatal("Expected version info for 2.0 (planned)")
	}

	// Test unknown version
	info4 := storage.GetVersionInfo("999.0")
	if info4 != nil {
		t.Error("Expected nil for unknown version")
	}
}
