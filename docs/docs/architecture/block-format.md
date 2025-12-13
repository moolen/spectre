---
title: Block Format Reference
description: Complete binary file format specification for Spectre storage files
keywords: [architecture, binary format, file structure, specification]
---

# Block Format Reference

This document provides a complete specification of Spectre's binary storage file format. The format is designed for append-only writes, efficient compression, and fast query access through indexing.

## Format Overview

**Current Version:** 1.0
**File Extension:** `.bin`
**Encoding:** Little-endian binary
**Compression:** gzip (level 6 - DefaultCompression)
**Event Encoding:** Protobuf with length-prefixed messages

### Magic Bytes

- **Header Magic:** `RPKBLOCK` (8 bytes ASCII)
- **Footer Magic:** `RPKEND` (8 bytes ASCII)

These magic bytes enable file type identification and integrity validation.

### File Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                        File Header (77 bytes)                   │
│  Magic: RPKBLOCK | Version: 1.0 | Compression: gzip | ...       │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                         Block 0 (variable)                       │
│              Compressed Protobuf Event Stream                    │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                         Block 1 (variable)                       │
│              Compressed Protobuf Event Stream                    │
└─────────────────────────────────────────────────────────────────┘
                              ...
┌─────────────────────────────────────────────────────────────────┐
│                         Block N (variable)                       │
│              Compressed Protobuf Event Stream                    │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                    Index Section (JSON, variable)                │
│  BlockMetadata | InvertedIndexes | Statistics | States          │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                      File Footer (324 bytes)                     │
│  IndexOffset | IndexLength | Checksum | Magic: RPKEND           │
└─────────────────────────────────────────────────────────────────┘
```

## File Header (77 bytes)

The header contains metadata required to read and validate the file format.

### Header Layout

| Offset | Length | Field                 | Type    | Description                                    |
|--------|--------|-----------------------|---------|------------------------------------------------|
| 0      | 8      | MagicBytes            | ASCII   | Must be "RPKBLOCK"                             |
| 8      | 8      | FormatVersion         | ASCII   | Version string (e.g., "1.0"), null-padded      |
| 16     | 8      | CreatedAt             | int64   | Unix timestamp in nanoseconds                  |
| 24     | 16     | CompressionAlgorithm  | ASCII   | "gzip" or "zstd", null-padded                  |
| 40     | 4      | BlockSize             | int32   | Target uncompressed block size in bytes        |
| 44     | 16     | EncodingFormat        | ASCII   | "protobuf" or "json", null-padded              |
| 60     | 1      | ChecksumEnabled       | byte    | 0 = disabled, 1 = enabled                      |
| 61     | 16     | Reserved              | bytes   | Reserved for future use (zeros)                |
| **77** | -      | **Total**             | -       | Fixed header size                              |

### Reading the Header

```go
// Read file header
file.Seek(0, io.SeekStart)
headerBytes := make([]byte, 77)
file.Read(headerBytes)

// Parse magic bytes
magic := string(headerBytes[0:8])
if magic != "RPKBLOCK" {
    return fmt.Errorf("invalid file format")
}

// Parse version
version := string(bytes.TrimRight(headerBytes[8:16], "\x00"))

// Parse created timestamp
createdAt := int64(binary.LittleEndian.Uint64(headerBytes[16:24]))

// Parse compression algorithm
compression := string(bytes.TrimRight(headerBytes[24:40], "\x00"))

// Parse block size
blockSize := int32(binary.LittleEndian.Uint32(headerBytes[40:44]))

// Parse encoding format
encoding := string(bytes.TrimRight(headerBytes[44:60], "\x00"))

// Parse checksum flag
checksumEnabled := headerBytes[60] != 0
```

### Default Header Values

```go
MagicBytes:           "RPKBLOCK"
FormatVersion:        "1.0"
CreatedAt:            time.Now().UnixNano()
CompressionAlgorithm: "gzip"      // Note: "zstd" defined but not implemented
BlockSize:            262144      // 256KB (default constant in code)
EncodingFormat:       "protobuf"
ChecksumEnabled:      false
```

## Block Data Section

Blocks are written sequentially after the file header. Each block contains compressed event data.

### Block Structure

```go
type Block struct {
    ID                 int32   // Sequential block number (0-based)
    Offset             int64   // Byte offset in file
    Length             int64   // Compressed data length
    UncompressedLength int64   // Uncompressed data length
    EventCount         int32   // Number of events
    TimestampMin       int64   // Minimum event timestamp (nanoseconds)
    TimestampMax       int64   // Maximum event timestamp (nanoseconds)
    CompressedData     []byte  // gzip-compressed protobuf stream
}
```

### Event Encoding (Protobuf)

Events within a block are encoded as length-prefixed protobuf messages:

```
┌─────────────┬──────────────┬─────────────┬──────────────┐
│ varint len  │ protobuf msg │ varint len  │ protobuf msg │ ...
└─────────────┴──────────────┴─────────────┴──────────────┘
```

**Encoding Process:**
1. Unmarshal Event from JSON to Go struct
2. Marshal Event to protobuf bytes
3. Write varint-encoded length (using `binary.PutUvarint`)
4. Write protobuf bytes
5. Repeat for all events in block

**Decoding Process:**
1. Read varint-encoded length
2. Read protobuf bytes of that length
3. Unmarshal protobuf to Event struct
4. Repeat until end of decompressed data

### Compression

Each block's protobuf stream is compressed using **gzip**:

- **Library:** `github.com/klauspost/compress/gzip`
- **Compression Level:** `gzip.DefaultCompression` (level 6)
- **Typical Ratio:** 0.20-0.30 (70-80% reduction)
- **Effectiveness Check:** Compression must achieve at least 10% reduction (ratio < 0.9)

## Index Section (JSON)

The index section is a JSON-encoded structure written after all blocks, before the footer. It contains metadata for fast query execution.

### Index Structure

```json
{
  "format_version": "1.0",
  "block_metadata": [
    {
      "id": 0,
      "offset": 77,
      "compressed_length": 65432,
      "uncompressed_length": 262144,
      "event_count": 200,
      "timestamp_min": 1733915200000000000,
      "timestamp_max": 1733915259999999999,
      "kind_set": ["Pod", "Deployment", "Service"],
      "namespace_set": ["default", "kube-system"],
      "group_set": ["apps", ""],
      "bloom_filter_kinds": {
        "serialized_bitset": "base64-encoded-bloom-filter",
        "false_positive_rate": 0.05,
        "expected_elements": 1000,
        "hash_functions": 4
      },
      "bloom_filter_namespaces": { ... },
      "bloom_filter_groups": { ... },
      "checksum": ""
    }
  ],
  "inverted_indexes": {
    "kind_to_blocks": {
      "Pod": [0, 1, 3, 7],
      "Deployment": [0, 2, 5],
      "Service": [0, 4, 6]
    },
    "namespace_to_blocks": {
      "default": [0, 1, 2],
      "kube-system": [3, 4, 5]
    },
    "group_to_blocks": {
      "": [0, 1],
      "apps": [2, 3],
      "batch": [4]
    }
  },
  "statistics": {
    "total_blocks": 300,
    "total_events": 60000,
    "total_uncompressed_bytes": 78643200,
    "total_compressed_bytes": 19660800,
    "compression_ratio": 0.25,
    "unique_kinds": 15,
    "unique_namespaces": 8,
    "unique_groups": 6,
    "timestamp_min": 1733915200000000000,
    "timestamp_max": 1733918799999999999
  },
  "final_resource_states": {
    "apps/v1/Deployment/default/nginx": {
      "uid": "abc123",
      "event_type": "UPDATE",
      "timestamp": 1733918799999999999,
      "resource_data": { ... }
    }
  }
}
```

### Block Metadata Fields

| Field                     | Type                  | Description                                       |
|---------------------------|-----------------------|---------------------------------------------------|
| `id`                      | int32                 | Block ID (0-based sequential)                     |
| `offset`                  | int64                 | Byte offset in file where block starts            |
| `compressed_length`       | int64                 | Size of compressed data in bytes                  |
| `uncompressed_length`     | int64                 | Size before compression                           |
| `event_count`             | int32                 | Number of events in block                         |
| `timestamp_min`           | int64                 | Minimum event timestamp (nanoseconds)             |
| `timestamp_max`           | int64                 | Maximum event timestamp (nanoseconds)             |
| `kind_set`                | []string              | Unique resource kinds in block                    |
| `namespace_set`           | []string              | Unique namespaces in block                        |
| `group_set`               | []string              | Unique API groups in block                        |
| `bloom_filter_kinds`      | BloomFilter           | Probabilistic kind filter (5% FP rate)            |
| `bloom_filter_namespaces` | BloomFilter           | Probabilistic namespace filter (5% FP rate)       |
| `bloom_filter_groups`     | BloomFilter           | Probabilistic group filter (5% FP rate)           |
| `checksum`                | string                | CRC32 hex if enabled, empty otherwise             |

### Bloom Filter Configuration

Each block has three Bloom filters for efficient filtering:

| Filter Type  | Expected Elements | False Positive Rate | Purpose                   |
|--------------|-------------------|---------------------|---------------------------|
| Kinds        | 1000              | 0.05 (5%)           | Filter by resource kind   |
| Namespaces   | 100               | 0.05 (5%)           | Filter by namespace       |
| Groups       | 100               | 0.05 (5%)           | Filter by API group       |

**Combined False Positive Rate:** ~14.3% when using all three filters together (1 - (1 - 0.05)³)

### Inverted Indexes

The inverted indexes map resource attribute values to block IDs for fast filtering:

```go
type InvertedIndex struct {
    KindToBlocks      map[string][]int32  // kind → block IDs
    NamespaceToBlocks map[string][]int32  // namespace → block IDs
    GroupToBlocks     map[string][]int32  // group → block IDs
}
```

**Query Optimization:**
- Query: `kind=Pod AND namespace=default`
- Lookup: `kind_to_blocks["Pod"] = [0, 1, 3, 7]`
- Lookup: `namespace_to_blocks["default"] = [0, 1, 2]`
- Intersection: `[0, 1, 3, 7] ∩ [0, 1, 2] = [0, 1]`
- Result: Only blocks 0 and 1 need to be decompressed

### Final Resource States

The `final_resource_states` map preserves the last known state of each resource at the time the file was closed. This enables consistent resource views across hourly file boundaries.

**Key Format:** `group/version/kind/namespace/name`
**Example:** `apps/v1/Deployment/default/nginx`

```go
type ResourceLastState struct {
    UID          string          // Resource UID
    EventType    string          // CREATE, UPDATE, or DELETE
    Timestamp    int64           // Last observed timestamp
    ResourceData json.RawMessage // Full resource object (null for DELETE)
}
```

## File Footer (324 bytes)

The footer enables backward seeking to locate the index section and validates file integrity.

### Footer Layout

| Offset | Length | Field               | Type    | Description                                |
|--------|--------|---------------------|---------|---------------------------------------------|
| 0      | 8      | IndexSectionOffset  | int64   | Byte offset where index section starts      |
| 8      | 4      | IndexSectionLength  | int32   | Byte length of index section                |
| 12     | 256    | Checksum            | ASCII   | CRC32 hash (hex), null-padded if unused     |
| 268    | 48     | Reserved            | bytes   | Reserved for future use (zeros)             |
| 316    | 8      | MagicBytes          | ASCII   | Must be "RPKEND"                            |
| **324**| -      | **Total**           | -       | Fixed footer size                           |

### Reading the Footer

```go
// Seek to footer (324 bytes from end)
file.Seek(-324, io.SeekEnd)
footerBytes := make([]byte, 324)
file.Read(footerBytes)

// Verify magic bytes
magic := string(bytes.TrimRight(footerBytes[316:324], "\x00"))
if magic != "RPKEND" {
    return fmt.Errorf("invalid or incomplete file")
}

// Parse index offset and length
indexOffset := int64(binary.LittleEndian.Uint64(footerBytes[0:8]))
indexLength := int32(binary.LittleEndian.Uint32(footerBytes[8:12]))

// Parse checksum (optional)
checksum := string(bytes.TrimRight(footerBytes[12:268], "\x00"))

// Read index section
file.Seek(indexOffset, io.SeekStart)
indexBytes := make([]byte, indexLength)
file.Read(indexBytes)

// Parse JSON index
var index IndexSection
json.Unmarshal(indexBytes, &index)
```

## File Validation

### Integrity Checks

When opening a file, perform these validation steps:

1. **Header Magic Bytes:** Verify `MagicBytes == "RPKBLOCK"`
2. **Format Version:** Verify version is supported (currently only `1.0`)
3. **Footer Magic Bytes:** Verify `MagicBytes == "RPKEND"`
4. **Index Offset:** Verify offset is within file bounds
5. **Index Length:** Verify length is reasonable (not negative, not larger than file)
6. **Block Checksums:** Verify each block's CRC32 if checksums enabled

### Crash Detection

If the footer is missing or invalid:
- File is **incomplete** (crashed during write)
- Rename to `.incomplete.<timestamp>`
- Create new empty file

If the header is invalid:
- File is **corrupted**
- Rename to `.corrupted.<timestamp>`
- Create new empty file

## Version Compatibility

### Version Support Matrix

| Reader Version | File Version | Compatible? | Notes                              |
|----------------|--------------|-------------|------------------------------------|
| 1.0            | 1.0          | ✅ Yes      | Full support                       |
| 1.0            | 1.1          | ⚠️ Partial  | Forward compatible (minor version) |
| 1.0            | 2.0          | ❌ No       | Major version mismatch             |

### Version Validation

```go
func ValidateVersion(version string) error {
    // Version format: "major.minor"
    parts := strings.Split(version, ".")
    if len(parts) != 2 {
        return fmt.Errorf("invalid version format: %s", version)
    }

    major := parts[0]

    // Support all 1.x versions (forward compatible within major version)
    if major == "1" {
        return nil
    }

    return fmt.Errorf("unsupported major version: %s", major)
}
```

### Future Versions

**Version 1.1** (Planned):
- Enhanced metadata tracking
- Optional JSON encoding support
- Improved Bloom filter configurations

**Version 2.0** (Planned):
- Full zstd compression support
- Variable-length block sizes
- Dictionary learning for better compression
- Distributed query optimizations

## Complete Example: Reading a File

```go
package main

import (
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"
    "os"
)

func ReadStorageFile(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    // 1. Read and validate header
    file.Seek(0, io.SeekStart)
    headerBytes := make([]byte, 77)
    if _, err := file.Read(headerBytes); err != nil {
        return fmt.Errorf("failed to read header: %w", err)
    }

    magic := string(headerBytes[0:8])
    if magic != "RPKBLOCK" {
        return fmt.Errorf("invalid file format: %s", magic)
    }

    version := string(bytes.TrimRight(headerBytes[8:16], "\x00"))
    compression := string(bytes.TrimRight(headerBytes[24:40], "\x00"))
    fmt.Printf("File version: %s, compression: %s\n", version, compression)

    // 2. Read and validate footer
    file.Seek(-324, io.SeekEnd)
    footerBytes := make([]byte, 324)
    if _, err := file.Read(footerBytes); err != nil {
        return fmt.Errorf("failed to read footer: %w", err)
    }

    footerMagic := string(bytes.TrimRight(footerBytes[316:324], "\x00"))
    if footerMagic != "RPKEND" {
        return fmt.Errorf("incomplete or corrupted file")
    }

    // 3. Read index section
    indexOffset := int64(binary.LittleEndian.Uint64(footerBytes[0:8]))
    indexLength := int32(binary.LittleEndian.Uint32(footerBytes[8:12]))

    file.Seek(indexOffset, io.SeekStart)
    indexBytes := make([]byte, indexLength)
    if _, err := file.Read(indexBytes); err != nil {
        return fmt.Errorf("failed to read index: %w", err)
    }

    var index IndexSection
    if err := json.Unmarshal(indexBytes, &index); err != nil {
        return fmt.Errorf("failed to parse index: %w", err)
    }

    fmt.Printf("Total blocks: %d, total events: %d\n",
        index.Statistics.TotalBlocks, index.Statistics.TotalEvents)

    // 4. Query specific block
    blockMeta := index.BlockMetadata[5]
    file.Seek(blockMeta.Offset, io.SeekStart)
    compressedData := make([]byte, blockMeta.CompressedLength)
    if _, err := file.Read(compressedData); err != nil {
        return fmt.Errorf("failed to read block: %w", err)
    }

    // 5. Decompress and parse events
    events, err := DecompressAndParseBlock(compressedData)
    if err != nil {
        return fmt.Errorf("failed to decompress block: %w", err)
    }

    fmt.Printf("Block 5 contains %d events\n", len(events))

    return nil
}
```

## Performance Considerations

### File Size Estimates

| Component            | Size                      | Percentage |
|----------------------|---------------------------|------------|
| File Header          | 77 bytes                  | \<0.001%    |
| Compressed Events    | 15-25 MB/hour (typical)   | ~95%       |
| Index Metadata       | 500 KB - 2 MB/hour        | ~3-5%      |
| Bloom Filters        | 100-200 KB/hour           | ~1%        |
| File Footer          | 324 bytes                 | \<0.001%    |

### Read Performance

- **Header read:** O(1) - 77 bytes
- **Footer read:** O(1) - 324 bytes from end
- **Index read:** O(N) where N = index size (typically \<2 MB)
- **Block read:** O(1) seek + O(M) decompress where M = block size

### Write Performance

- **Event buffering:** O(1) per event
- **Block finalization:** O(N) where N = events in block (protobuf encode + gzip compress)
- **Index write:** O(M) where M = total blocks (build inverted indexes)

## Related Documentation

- [Storage Design](./storage-design.md) - Architecture and design decisions
- [Indexing Strategy](./indexing-strategy.md) - Query optimization techniques
- [Compression](./compression.md) - Compression algorithms and performance
- [Storage Settings](../configuration/storage-settings.md) - Configuration guide

<!-- Source: internal/storage/block_format.go, internal/storage/block.go, internal/storage/compression.go -->
