package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const (
	// Magic bytes for file identification
	FileHeaderMagic = "RPKBLOCK"
	FileFooterMagic = "RPKEND"

	// Format versions - supports future evolution
	// Version 1.0: Initial release with block-based compression and inverted indexing
	// Version 1.1: (Future) Enhanced metadata
	// Version 2.0: (Future) Protobuf encoding support
	DefaultFormatVersion = "1.0"
	FormatVersionV1_0    = "1.0"
	FormatVersionV1_1    = "1.1" // Future: enhanced metadata
	FormatVersionV2_0    = "2.0" // Future: protobuf support

	// Supported versions - newer readers must handle older formats
	MinSupportedVersion = "1.0"
	MaxSupportedVersion = "1.0" // Currently only 1.0 fully implemented

	// Default compression algorithm
	DefaultCompressionAlgorithm = "zstd"

	// Default block size (256KB)
	DefaultBlockSize = 256 * 1024

	// Fixed header size (77 bytes)
	FileHeaderSize = 77

	// Fixed footer size (324 bytes)
	FileFooterSize = 324
)

// FileHeader identifies and describes the storage file format, version, and configuration
type FileHeader struct {
	// MagicBytes must be exactly "RPKBLOCK" for format identification
	MagicBytes string

	// FormatVersion is e.g., "1.0" for major.minor versioning
	FormatVersion string

	// CreatedAt is Unix timestamp in nanoseconds
	CreatedAt int64

	// CompressionAlgorithm is "gzip" or "zstd"
	CompressionAlgorithm string

	// BlockSize is the uncompressed block size limit in bytes
	BlockSize int32

	// EncodingFormat is "json" or "protobuf"
	EncodingFormat string

	// ChecksumEnabled indicates whether checksums are computed
	ChecksumEnabled bool

	// Reserved is 16 bytes for future extensions
	Reserved [16]byte
}

// NewFileHeader creates a new file header with default values
func NewFileHeader() *FileHeader {
	return &FileHeader{
		MagicBytes:           FileHeaderMagic,
		FormatVersion:        DefaultFormatVersion,
		CreatedAt:            time.Now().UnixNano(),
		CompressionAlgorithm: DefaultCompressionAlgorithm,
		BlockSize:            int32(DefaultBlockSize),
		EncodingFormat:       "json",
		ChecksumEnabled:      false,
	}
}

// WriteFileHeader serializes FileHeader to a writer (77 bytes fixed)
func WriteFileHeader(w io.Writer, header *FileHeader) error {
	buf := make([]byte, FileHeaderSize)
	pos := 0

	// Write magic bytes (8 bytes)
	copy(buf[pos:pos+8], []byte(FileHeaderMagic))
	pos += 8

	// Write format version (8 bytes, null-padded)
	versionBytes := make([]byte, 8)
	copy(versionBytes, header.FormatVersion)
	copy(buf[pos:pos+8], versionBytes)
	pos += 8

	// Write created at timestamp (8 bytes)
	binary.LittleEndian.PutUint64(buf[pos:pos+8], uint64(header.CreatedAt))
	pos += 8

	// Write compression algorithm (16 bytes, null-padded)
	algoBytes := make([]byte, 16)
	copy(algoBytes, header.CompressionAlgorithm)
	copy(buf[pos:pos+16], algoBytes)
	pos += 16

	// Write block size (4 bytes)
	binary.LittleEndian.PutUint32(buf[pos:pos+4], uint32(header.BlockSize))
	pos += 4

	// Write encoding format (16 bytes, null-padded)
	encBytes := make([]byte, 16)
	copy(encBytes, header.EncodingFormat)
	copy(buf[pos:pos+16], encBytes)
	pos += 16

	// Write checksum enabled (1 byte)
	if header.ChecksumEnabled {
		buf[pos] = 1
	} else {
		buf[pos] = 0
	}
	pos += 1

	// Write reserved (16 bytes)
	copy(buf[pos:pos+16], header.Reserved[:])
	pos += 16

	// Verify buffer is exactly FileHeaderSize
	if pos != FileHeaderSize {
		return fmt.Errorf("header buffer size mismatch: expected %d, got %d", FileHeaderSize, pos)
	}

	_, err := w.Write(buf)
	return err
}

// ReadFileHeader deserializes FileHeader from a reader
func ReadFileHeader(r io.Reader) (*FileHeader, error) {
	buf := make([]byte, FileHeaderSize)
	if _, err := r.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	pos := 0
	header := &FileHeader{}

	// Read magic bytes
	header.MagicBytes = string(bytes.TrimRight(buf[pos:pos+8], "\x00"))
	if header.MagicBytes != FileHeaderMagic {
		return nil, fmt.Errorf("invalid file header magic bytes: %s", header.MagicBytes)
	}
	pos += 8

	// Read format version
	header.FormatVersion = string(bytes.TrimRight(buf[pos:pos+8], "\x00"))
	pos += 8

	// Read created at
	header.CreatedAt = int64(binary.LittleEndian.Uint64(buf[pos : pos+8]))
	pos += 8

	// Read compression algorithm
	header.CompressionAlgorithm = string(bytes.TrimRight(buf[pos:pos+16], "\x00"))
	pos += 16

	// Read block size
	header.BlockSize = int32(binary.LittleEndian.Uint32(buf[pos : pos+4]))
	pos += 4

	// Read encoding format
	header.EncodingFormat = string(bytes.TrimRight(buf[pos:pos+16], "\x00"))
	pos += 16

	// Read checksum enabled
	header.ChecksumEnabled = buf[pos] != 0
	pos += 1

	// Read reserved
	copy(header.Reserved[:], buf[pos:pos+16])
	pos += 16

	if pos != FileHeaderSize {
		return nil, fmt.Errorf("header buffer size mismatch: expected %d, got %d", FileHeaderSize, pos)
	}

	return header, nil
}

// FileFooter enables backward seeking to find index section and validates file integrity
type FileFooter struct {
	// IndexSectionOffset is the byte offset where IndexSection starts in file
	IndexSectionOffset int64

	// IndexSectionLength is the byte length of IndexSection
	IndexSectionLength int32

	// Checksum is CRC32 of entire file before footer, if enabled
	Checksum string

	// Reserved is 48 bytes for future extensions
	Reserved [48]byte

	// MagicBytes must be exactly "RPKEND" for EOF validation
	MagicBytes string
}

// WriteFileFooter serializes FileFooter to a writer (324 bytes fixed)
func WriteFileFooter(w io.Writer, footer *FileFooter) error {
	buf := make([]byte, FileFooterSize)
	pos := 0

	// Write index section offset (8 bytes)
	binary.LittleEndian.PutUint64(buf[pos:pos+8], uint64(footer.IndexSectionOffset))
	pos += 8

	// Write index section length (4 bytes)
	binary.LittleEndian.PutUint32(buf[pos:pos+4], uint32(footer.IndexSectionLength))
	pos += 4

	// Write checksum (256 bytes, null-padded)
	checksumBytes := make([]byte, 256)
	copy(checksumBytes, footer.Checksum)
	copy(buf[pos:pos+256], checksumBytes)
	pos += 256

	// Write reserved (48 bytes)
	copy(buf[pos:pos+48], footer.Reserved[:])
	pos += 48

	// Write magic bytes (8 bytes)
	copy(buf[pos:pos+8], []byte(FileFooterMagic))
	pos += 8

	// Verify buffer is exactly FileFooterSize
	if pos != FileFooterSize {
		return fmt.Errorf("footer buffer size mismatch: expected %d, got %d", FileFooterSize, pos)
	}

	_, err := w.Write(buf)
	return err
}

// ReadFileFooter deserializes FileFooter from a reader (reads from end of file)
func ReadFileFooter(r io.Reader) (*FileFooter, error) {
	buf := make([]byte, FileFooterSize)
	if _, err := r.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to read file footer: %w", err)
	}

	pos := 0
	footer := &FileFooter{}

	// Read index section offset
	footer.IndexSectionOffset = int64(binary.LittleEndian.Uint64(buf[pos : pos+8]))
	pos += 8

	// Read index section length
	footer.IndexSectionLength = int32(binary.LittleEndian.Uint32(buf[pos : pos+4]))
	pos += 4

	// Read checksum
	footer.Checksum = string(bytes.TrimRight(buf[pos:pos+256], "\x00"))
	pos += 256

	// Read reserved
	copy(footer.Reserved[:], buf[pos:pos+48])
	pos += 48

	// Read magic bytes
	footer.MagicBytes = string(bytes.TrimRight(buf[pos:pos+8], "\x00"))
	if footer.MagicBytes != FileFooterMagic {
		return nil, fmt.Errorf("invalid file footer magic bytes: %s", footer.MagicBytes)
	}
	pos += 8

	if pos != FileFooterSize {
		return nil, fmt.Errorf("footer buffer size mismatch: expected %d, got %d", FileFooterSize, pos)
	}

	return footer, nil
}

// InvertedIndex maps resource metadata values to candidate blocks for rapid filtering
type InvertedIndex struct {
	// KindToBlocks maps resource kind → list of block IDs
	KindToBlocks map[string][]int32 `json:"kind_to_blocks"`

	// NamespaceToBlocks maps namespace → list of block IDs
	NamespaceToBlocks map[string][]int32 `json:"namespace_to_blocks"`

	// GroupToBlocks maps resource group → list of block IDs
	GroupToBlocks map[string][]int32 `json:"group_to_blocks"`
}

// IndexStatistics contains file-level statistics
type IndexStatistics struct {
	TotalBlocks            int32   `json:"total_blocks"`
	TotalEvents            int64   `json:"total_events"`
	TotalUncompressedBytes int64   `json:"total_uncompressed_bytes"`
	TotalCompressedBytes   int64   `json:"total_compressed_bytes"`
	CompressionRatio       float32 `json:"compression_ratio"`
	UniqueKinds            int32   `json:"unique_kinds"`
	UniqueNamespaces       int32   `json:"unique_namespaces"`
	UniqueGroups           int32   `json:"unique_groups"`
	TimestampMin           int64   `json:"timestamp_min"`
	TimestampMax           int64   `json:"timestamp_max"`
}

// IndexSection is a collection of metadata and indexes written to end of file for fast access
type IndexSection struct {
	// FormatVersion matches FileHeader.FormatVersion
	FormatVersion string `json:"format_version"`

	// BlockMetadata is metadata for each block
	BlockMetadata []*BlockMetadata `json:"block_metadata"`

	// InvertedIndexes maps values to block IDs
	InvertedIndexes *InvertedIndex `json:"inverted_indexes"`

	// Statistics contains file-level stats
	Statistics *IndexStatistics `json:"statistics"`
}

// WriteIndexSection serializes IndexSection to a writer using JSON encoding
func WriteIndexSection(w io.Writer, section *IndexSection) (int64, error) {
	// Serialize to JSON
	jsonData, err := json.MarshalIndent(section, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal index section: %w", err)
	}

	// Write to writer
	n, err := w.Write(jsonData)
	if err != nil {
		return 0, fmt.Errorf("failed to write index section: %w", err)
	}

	return int64(n), nil
}

// ReadIndexSection deserializes IndexSection from a reader
func ReadIndexSection(r io.Reader) (*IndexSection, error) {
	var section IndexSection
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&section); err != nil {
		return nil, fmt.Errorf("failed to decode index section: %w", err)
	}

	return &section, nil
}

// BuildInvertedIndexes creates inverted indexes from block metadata
func BuildInvertedIndexes(blocks []*BlockMetadata) *InvertedIndex {
	index := &InvertedIndex{
		KindToBlocks:      make(map[string][]int32),
		NamespaceToBlocks: make(map[string][]int32),
		GroupToBlocks:     make(map[string][]int32),
	}

	for _, block := range blocks {
		// Add kinds
		for _, kind := range block.KindSet {
			index.KindToBlocks[kind] = append(index.KindToBlocks[kind], block.ID)
		}

		// Add namespaces
		for _, ns := range block.NamespaceSet {
			index.NamespaceToBlocks[ns] = append(index.NamespaceToBlocks[ns], block.ID)
		}

		// Add groups
		for _, group := range block.GroupSet {
			index.GroupToBlocks[group] = append(index.GroupToBlocks[group], block.ID)
		}
	}

	return index
}

// GetCandidateBlocks returns candidate block IDs for a query with AND logic on filters
func GetCandidateBlocks(index *InvertedIndex, filters map[string]string) []int32 {
	if index == nil || len(filters) == 0 {
		return nil
	}

	var candidates map[int32]bool
	filterCount := 0

	// For each filter, intersect the candidate blocks
	if kind, ok := filters["kind"]; ok && kind != "" {
		if blocks, ok := index.KindToBlocks[kind]; ok {
			// Initialize candidates with first filter's blocks
			candidates = make(map[int32]bool)
			for _, b := range blocks {
				candidates[b] = true
			}
		} else {
			return nil // Kind not found, no candidates
		}
		filterCount++
	}

	if ns, ok := filters["namespace"]; ok && ns != "" {
		if blocks, ok := index.NamespaceToBlocks[ns]; ok {
			if candidates == nil {
				// Initialize candidates with namespace filter's blocks
				candidates = make(map[int32]bool)
				for _, b := range blocks {
					candidates[b] = true
				}
			} else {
				// Intersect with existing candidates
				newCandidates := make(map[int32]bool)
				for _, b := range blocks {
					if candidates[b] {
						newCandidates[b] = true
					}
				}
				candidates = newCandidates
			}
		} else {
			return nil // Namespace not found, no candidates
		}
		filterCount++
	}

	if group, ok := filters["group"]; ok && group != "" {
		if blocks, ok := index.GroupToBlocks[group]; ok {
			if candidates == nil {
				candidates = make(map[int32]bool)
				for _, b := range blocks {
					candidates[b] = true
				}
			} else {
				newCandidates := make(map[int32]bool)
				for _, b := range blocks {
					if candidates[b] {
						newCandidates[b] = true
					}
				}
				candidates = newCandidates
			}
		} else {
			return nil // Group not found, no candidates
		}
		filterCount++
	}

	if candidates == nil {
		return nil
	}

	// Convert map to sorted slice
	result := make([]int32, 0, len(candidates))
	for blockID := range candidates {
		result = append(result, blockID)
	}

	return result
}

// VersionInfo provides information about file format versions
type VersionInfo struct {
	Version     string
	Description string
	Introduced  string
	Features    []string
	Deprecated  bool
}

// GetVersionInfo returns information about a specific file format version
func GetVersionInfo(version string) *VersionInfo {
	switch version {
	case FormatVersionV1_0:
		return &VersionInfo{
			Version:     "1.0",
			Description: "Initial release with block-based compression and inverted indexing",
			Introduced:  "2025-11",
			Features: []string{
				"Block-based storage with gzip compression",
				"Inverted indexes for kinds, namespaces, groups",
				"Bloom filters for space-efficient filtering",
				"MD5 checksums for block integrity",
				"Fixed-size file header and footer",
				"JSON-encoded index section",
			},
			Deprecated: false,
		}
	case FormatVersionV1_1:
		return &VersionInfo{
			Version:     "1.1",
			Description: "Enhanced metadata and optional protobuf encoding",
			Introduced:  "(planned)",
			Features: []string{
				"All 1.0 features",
				"Enhanced metadata tracking",
				"Support for both JSON and protobuf encoding",
				"Improved compression algorithms",
			},
			Deprecated: false,
		}
	case FormatVersionV2_0:
		return &VersionInfo{
			Version:     "2.0",
			Description: "Major format evolution with protobuf and extended features",
			Introduced:  "(planned)",
			Features: []string{
				"Full protobuf support",
				"Variable-length block sizes",
				"Dictionary learning for compression",
				"Distributed query support",
			},
			Deprecated: false,
		}
	default:
		return nil
	}
}

// ValidateVersion checks if a file version is supported
func ValidateVersion(version string) error {
	// Parse version to check basic format (e.g., "1.0")
	if version == "" {
		return fmt.Errorf("empty version string")
	}

	// Extract major version (everything before the first dot)
	dotIndex := 0
	for i := 0; i < len(version); i++ {
		if version[i] == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == 0 && len(version) > 0 && version[0] != '.' {
		dotIndex = len(version) // No dot found, use whole string as major
	}

	if dotIndex <= 0 {
		return fmt.Errorf("invalid version format: %s", version)
	}

	majorVersion := version[0:dotIndex]

	// Check if version is supported
	// For future compatibility, allow newer minor versions of supported major versions
	// e.g., 1.0 reader can read 1.1, 1.2, etc. files
	if majorVersion == "1" {
		// Allow all 1.x versions (backward compatible)
		return nil
	}

	return fmt.Errorf("unsupported version: %s (supported: 1.x)", version)
}
