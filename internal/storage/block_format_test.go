package storage

import (
	"bytes"
	"testing"
	"time"
)

func TestNewFileHeader(t *testing.T) {
	header := NewFileHeader()

	if header.MagicBytes != FileHeaderMagic {
		t.Errorf("expected magic bytes %s, got %s", FileHeaderMagic, header.MagicBytes)
	}

	if header.FormatVersion != DefaultFormatVersion {
		t.Errorf("expected format version %s, got %s", DefaultFormatVersion, header.FormatVersion)
	}

	if header.CreatedAt <= 0 {
		t.Errorf("expected positive created at timestamp, got %d", header.CreatedAt)
	}

	if header.CompressionAlgorithm != DefaultCompressionAlgorithm {
		t.Errorf("expected compression algorithm %s, got %s", DefaultCompressionAlgorithm, header.CompressionAlgorithm)
	}

	if header.BlockSize != int32(DefaultBlockSize) {
		t.Errorf("expected block size %d, got %d", DefaultBlockSize, header.BlockSize)
	}

	if header.EncodingFormat != "protobuf" {
		t.Errorf("expected encoding format protobuf, got %s", header.EncodingFormat)
	}

	if header.ChecksumEnabled {
		t.Error("expected checksum to be disabled by default")
	}
}

func TestWriteReadFileHeader(t *testing.T) {
	original := &FileHeader{
		MagicBytes:           FileHeaderMagic,
		FormatVersion:        "1.0",
		CreatedAt:            time.Now().UnixNano(),
		CompressionAlgorithm: "gzip",
		BlockSize:            256 * 1024,
		EncodingFormat:       "protobuf",
		ChecksumEnabled:      true,
		Reserved:             [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}

	var buf bytes.Buffer
	if err := WriteFileHeader(&buf, original); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	if buf.Len() != FileHeaderSize {
		t.Errorf("expected header size %d, got %d", FileHeaderSize, buf.Len())
	}

	read, err := ReadFileHeader(&buf)
	if err != nil {
		t.Fatalf("failed to read header: %v", err)
	}

	if read.MagicBytes != original.MagicBytes {
		t.Errorf("magic bytes mismatch: expected %s, got %s", original.MagicBytes, read.MagicBytes)
	}

	if read.FormatVersion != original.FormatVersion {
		t.Errorf("format version mismatch: expected %s, got %s", original.FormatVersion, read.FormatVersion)
	}

	if read.CreatedAt != original.CreatedAt {
		t.Errorf("created at mismatch: expected %d, got %d", original.CreatedAt, read.CreatedAt)
	}

	if read.CompressionAlgorithm != original.CompressionAlgorithm {
		t.Errorf("compression algorithm mismatch: expected %s, got %s", original.CompressionAlgorithm, read.CompressionAlgorithm)
	}

	if read.BlockSize != original.BlockSize {
		t.Errorf("block size mismatch: expected %d, got %d", original.BlockSize, read.BlockSize)
	}

	if read.EncodingFormat != original.EncodingFormat {
		t.Errorf("encoding format mismatch: expected %s, got %s", original.EncodingFormat, read.EncodingFormat)
	}

	if read.ChecksumEnabled != original.ChecksumEnabled {
		t.Errorf("checksum enabled mismatch: expected %v, got %v", original.ChecksumEnabled, read.ChecksumEnabled)
	}

	if read.Reserved != original.Reserved {
		t.Errorf("reserved bytes mismatch")
	}
}

func TestReadFileHeaderInvalidMagic(t *testing.T) {
	header := &FileHeader{
		MagicBytes:    "INVALID",
		FormatVersion: "1.0",
		CreatedAt:     time.Now().UnixNano(),
	}

	var buf bytes.Buffer
	if err := WriteFileHeader(&buf, header); err == nil {
		// Overwrite magic bytes
		buf.Reset()
		buf.WriteString("INVALID")
		buf.Write(make([]byte, FileHeaderSize-8))

		_, err := ReadFileHeader(&buf)
		if err == nil {
			t.Error("expected error for invalid magic bytes")
		}
	}
}

func TestWriteReadFileFooter(t *testing.T) {
	original := &FileFooter{
		IndexSectionOffset: 12345,
		IndexSectionLength: 67890,
		Checksum:           "abc123def456",
		Reserved:           [48]byte{1, 2, 3},
		MagicBytes:         FileFooterMagic,
	}

	var buf bytes.Buffer
	if err := WriteFileFooter(&buf, original); err != nil {
		t.Fatalf("failed to write footer: %v", err)
	}

	if buf.Len() != FileFooterSize {
		t.Errorf("expected footer size %d, got %d", FileFooterSize, buf.Len())
	}

	read, err := ReadFileFooter(&buf)
	if err != nil {
		t.Fatalf("failed to read footer: %v", err)
	}

	if read.IndexSectionOffset != original.IndexSectionOffset {
		t.Errorf("index section offset mismatch: expected %d, got %d", original.IndexSectionOffset, read.IndexSectionOffset)
	}

	if read.IndexSectionLength != original.IndexSectionLength {
		t.Errorf("index section length mismatch: expected %d, got %d", original.IndexSectionLength, read.IndexSectionLength)
	}

	if read.Checksum != original.Checksum {
		t.Errorf("checksum mismatch: expected %s, got %s", original.Checksum, read.Checksum)
	}

	if read.MagicBytes != original.MagicBytes {
		t.Errorf("magic bytes mismatch: expected %s, got %s", original.MagicBytes, read.MagicBytes)
	}
}

func TestReadFileFooterInvalidMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(make([]byte, FileFooterSize-8))
	buf.WriteString("INVALID")

	_, err := ReadFileFooter(&buf)
	if err == nil {
		t.Error("expected error for invalid footer magic bytes")
	}
}

func TestBuildInvertedIndexes(t *testing.T) {
	blocks := []*BlockMetadata{
		{
			ID:           0,
			KindSet:      []string{"Pod", "Service"},
			NamespaceSet: []string{"default", "kube-system"},
			GroupSet:     []string{"", "apps"},
		},
		{
			ID:           1,
			KindSet:      []string{"Pod", "Deployment"},
			NamespaceSet: []string{"default"},
			GroupSet:     []string{"apps"},
		},
		{
			ID:           2,
			KindSet:      []string{"Service"},
			NamespaceSet: []string{"kube-system"},
			GroupSet:     []string{""},
		},
	}

	index := BuildInvertedIndexes(blocks)

	// Check kind indexes
	if len(index.KindToBlocks["Pod"]) != 2 {
		t.Errorf("expected 2 blocks for Pod, got %d", len(index.KindToBlocks["Pod"]))
	}
	if len(index.KindToBlocks["Service"]) != 2 {
		t.Errorf("expected 2 blocks for Service, got %d", len(index.KindToBlocks["Service"]))
	}
	if len(index.KindToBlocks["Deployment"]) != 1 {
		t.Errorf("expected 1 block for Deployment, got %d", len(index.KindToBlocks["Deployment"]))
	}

	// Check namespace indexes
	if len(index.NamespaceToBlocks["default"]) != 2 {
		t.Errorf("expected 2 blocks for default namespace, got %d", len(index.NamespaceToBlocks["default"]))
	}
	if len(index.NamespaceToBlocks["kube-system"]) != 2 {
		t.Errorf("expected 2 blocks for kube-system namespace, got %d", len(index.NamespaceToBlocks["kube-system"]))
	}

	// Check group indexes
	if len(index.GroupToBlocks[""]) != 2 {
		t.Errorf("expected 2 blocks for empty group, got %d", len(index.GroupToBlocks[""]))
	}
	if len(index.GroupToBlocks["apps"]) != 2 {
		t.Errorf("expected 2 blocks for apps group, got %d", len(index.GroupToBlocks["apps"]))
	}
}

func TestBuildInvertedIndexesEmpty(t *testing.T) {
	index := BuildInvertedIndexes([]*BlockMetadata{})
	if index == nil {
		t.Error("expected non-nil index")
	}
	if len(index.KindToBlocks) != 0 {
		t.Errorf("expected empty kind index, got %d entries", len(index.KindToBlocks))
	}
}

func TestGetCandidateBlocks(t *testing.T) {
	index := &InvertedIndex{
		KindToBlocks: map[string][]int32{
			"Pod":        {0, 1, 2},
			"Service":    {0, 2},
			"Deployment": {1},
		},
		NamespaceToBlocks: map[string][]int32{
			"default":     {0, 1},
			"kube-system": {0, 2},
		},
		GroupToBlocks: map[string][]int32{
			"":     {0, 2},
			"apps": {0, 1},
		},
	}

	// Test single filter
	filters := map[string]string{"kind": "Pod"}
	candidates := GetCandidateBlocks(index, filters)
	if len(candidates) != 3 {
		t.Errorf("expected 3 candidate blocks for Pod, got %d", len(candidates))
	}

	// Test multiple filters (AND logic)
	filters = map[string]string{
		"kind":      "Pod",
		"namespace": "default",
	}
	candidates = GetCandidateBlocks(index, filters)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidate blocks for Pod in default namespace, got %d", len(candidates))
	}
	// Should be blocks 0 and 1
	if !contains(candidates, 0) || !contains(candidates, 1) {
		t.Error("expected blocks 0 and 1")
	}

	// Test no match
	filters = map[string]string{"kind": "NonExistent"}
	candidates = GetCandidateBlocks(index, filters)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidate blocks for non-existent kind, got %d", len(candidates))
	}

	// Test empty filters
	candidates = GetCandidateBlocks(index, map[string]string{})
	if candidates != nil {
		t.Errorf("expected nil for empty filters, got %v", candidates)
	}

	// Test nil index
	candidates = GetCandidateBlocks(nil, filters)
	if candidates != nil {
		t.Errorf("expected nil for nil index, got %v", candidates)
	}
}

func TestGetCandidateBlocksAllFilters(t *testing.T) {
	index := &InvertedIndex{
		KindToBlocks: map[string][]int32{
			"Pod": {0, 1},
		},
		NamespaceToBlocks: map[string][]int32{
			"default": {0, 1},
		},
		GroupToBlocks: map[string][]int32{
			"apps": {0, 1},
		},
	}

	filters := map[string]string{
		"kind":      "Pod",
		"namespace": "default",
		"group":     "apps",
	}
	candidates := GetCandidateBlocks(index, filters)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidate blocks, got %d", len(candidates))
	}
}

func TestGetCandidateBlocksNoIntersection(t *testing.T) {
	index := &InvertedIndex{
		KindToBlocks: map[string][]int32{
			"Pod": {0, 1},
		},
		NamespaceToBlocks: map[string][]int32{
			"other": {2, 3},
		},
	}

	filters := map[string]string{
		"kind":      "Pod",
		"namespace": "other",
	}
	candidates := GetCandidateBlocks(index, filters)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidate blocks (no intersection), got %d", len(candidates))
	}
}

func TestWriteReadIndexSection(t *testing.T) {
	section := &IndexSection{
		FormatVersion: "1.0",
		BlockMetadata: []*BlockMetadata{
			{
				ID:           0,
				KindSet:      []string{"Pod"},
				NamespaceSet: []string{"default"},
				EventCount:   10,
				TimestampMin: 1000,
				TimestampMax: 2000,
			},
		},
		InvertedIndexes: &InvertedIndex{
			KindToBlocks: map[string][]int32{
				"Pod": {0},
			},
		},
		Statistics: &IndexStatistics{
			TotalBlocks:  1,
			TotalEvents:  10,
			UniqueKinds:  1,
			TimestampMin: 1000,
			TimestampMax: 2000,
		},
	}

	var buf bytes.Buffer
	bytesWritten, err := WriteIndexSection(&buf, section)
	if err != nil {
		t.Fatalf("failed to write index section: %v", err)
	}

	if bytesWritten <= 0 {
		t.Error("expected positive bytes written")
	}

	read, err := ReadIndexSection(&buf)
	if err != nil {
		t.Fatalf("failed to read index section: %v", err)
	}

	if read.FormatVersion != section.FormatVersion {
		t.Errorf("format version mismatch: expected %s, got %s", section.FormatVersion, read.FormatVersion)
	}

	if len(read.BlockMetadata) != len(section.BlockMetadata) {
		t.Errorf("block metadata count mismatch: expected %d, got %d", len(section.BlockMetadata), len(read.BlockMetadata))
	}

	if read.Statistics.TotalBlocks != section.Statistics.TotalBlocks {
		t.Errorf("total blocks mismatch: expected %d, got %d", section.Statistics.TotalBlocks, read.Statistics.TotalBlocks)
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"valid 1.0", "1.0", false},
		{"valid 1.1", "1.1", false},
		{"valid 1.2", "1.2", false},
		{"valid 2.0", "2.0", false},
		{"valid 1", "1", true},
		{"invalid empty", "", true},
		{"invalid format", "invalid", true},
		{"invalid starts with dot", ".1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
		})
	}
}

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo("1.0")
	if info == nil {
		t.Fatal("expected version info for 1.0")
	}

	if info.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", info.Version)
	}

	if len(info.Features) == 0 {
		t.Error("expected features list")
	}

	// Test non-existent version
	info = GetVersionInfo("99.0")
	if info != nil {
		t.Error("expected nil for non-existent version")
	}
}

func contains(slice []int32, val int32) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
