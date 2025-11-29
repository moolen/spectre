# Internal Storage Engine

This package implements a custom, append-only storage engine designed for high-throughput writing and efficient querying of Kubernetes audit events. It draws inspiration from log-structured storage systems like Loki and VictoriaMetrics.

## Overview

The storage engine organizes data into **Blocks**. Each block contains a batch of compressed events. To enable fast retrieval without scanning the entire file, the engine maintains an **Inverted Index** and **Bloom Filters**, which are persisted to the end of the file upon closing.

## File Structure

The file format is designed to be append-only friendly while supporting efficient random access for reads.

```text
+-----------------------------------------------------------------------+
|                              File Header                              |
| (Magic, Version, CreatedAt, Compression, BlockSize, Encoding, ...)    |
|                               77 Bytes                                |
+-----------------------------------------------------------------------+
|                               Block 0                                 |
|                     (Compressed Event Payload)                        |
+-----------------------------------------------------------------------+
|                               Block 1                                 |
|                     (Compressed Event Payload)                        |
+-----------------------------------------------------------------------+
|                                 ...                                   |
+-----------------------------------------------------------------------+
|                               Block N                                 |
|                     (Compressed Event Payload)                        |
+-----------------------------------------------------------------------+
|                             Index Section                             |
| (JSON: BlockMetadata[], InvertedIndexes, Statistics)                  |
+-----------------------------------------------------------------------+
|                              File Footer                              |
| (IndexOffset, IndexLength, Checksum, Magic)                           |
|                              324 Bytes                                |
+-----------------------------------------------------------------------+
```

### 1. File Header (77 Bytes)
The header contains metadata required to read the file, such as the format version and compression algorithm.
*   **Magic Bytes**: `RPKBLOCK` (8 bytes)
*   **Version**: e.g., `1.0`
*   **Compression**: `gzip` or `zstd`
*   **Block Size**: Target size for uncompressed blocks (default 256KB)

### 2. Blocks
Events are buffered in memory until they reach the configured `BlockSize`. They are then compressed (default: gzip) and written to disk as a **Block**.
*   **Compression**: Reduces disk usage and I/O.
*   **Checksums**: Each block has a checksum (in metadata) to ensure data integrity.

### 3. Index Section
Located at the end of the file (before the footer), this section contains the **Inverted Index** and **Block Metadata**. It is JSON-encoded for flexibility.
*   **Block Metadata**: Contains statistics for each block (min/max timestamp, event count) and **Bloom Filters**.
*   **Inverted Index**: Maps specific values (Namespace, Kind, API Group) to the list of Block IDs that contain them.

### 4. File Footer (324 Bytes)
The footer allows the reader to locate the Index Section.
*   **Index Offset**: Byte offset where the Index Section starts.
*   **Magic Bytes**: `RPKEND` (8 bytes) - used to verify that the file was closed properly.

## Indexing & Retrieval

The storage engine uses a two-level indexing strategy to minimize the number of blocks that need to be decompressed during a query.

### Inverted Index

The primary mechanism for filtering is the **Inverted Index**. It maps high-cardinality fields (Namespace, Kind, API Group) directly to the list of Block IDs that contain them. This allows the query engine to instantly identify which blocks are relevant for a given query.

**Structure:**

```go
// InvertedIndex maps resource metadata values to candidate blocks for rapid filtering
type InvertedIndex struct {
    // KindToBlocks maps resource kind -> list of block IDs
    KindToBlocks map[string][]int32 `json:"kind_to_blocks"`

    // NamespaceToBlocks maps namespace -> list of block IDs
    NamespaceToBlocks map[string][]int32 `json:"namespace_to_blocks"`

    // GroupToBlocks maps resource group -> list of block IDs
    GroupToBlocks map[string][]int32 `json:"group_to_blocks"`
}
```

**Example:**
If you query for `namespace="kube-system"`, the engine looks up `NamespaceToBlocks["kube-system"]` and gets a list of block IDs (e.g., `[0, 5, 12]`). Only these blocks are candidates for reading.

### Bloom Filters

For space-efficient filtering, each block's metadata includes **Bloom Filters**. A Bloom Filter is a probabilistic data structure that tests whether an element is a member of a set. It is extremely space-efficient but has a small probability of false positives (it might say "yes" when the answer is "no") but zero probability of false negatives (if it says "no", the element is definitely not there).

In this storage engine, Bloom Filters are used to quickly skip blocks that definitely do *not* contain a specific value, without needing to load the full Inverted Index into memory if it becomes too large, or for verifying block contents during recovery.

**Structure:**

```go
// StandardBloomFilter implements BloomFilter using bits-and-blooms/bloom
type StandardBloomFilter struct {
    filter             *bloom.BloomFilter
    falsePositiveRate  float32
    expectedElements   uint
    serializedBitset   []byte
    hashFunctions      uint
}
```

Each `BlockMetadata` contains three separate Bloom Filters:

```go
type BlockMetadata struct {
    // ...
    BloomFilterKinds      *StandardBloomFilter `json:"bloom_filter_kinds,omitempty"`
    BloomFilterNamespaces *StandardBloomFilter `json:"bloom_filter_namespaces,omitempty"`
    BloomFilterGroups     *StandardBloomFilter `json:"bloom_filter_groups,omitempty"`
    // ...
}
```

**How it works:**
1.  **Write Path**: As events are added to a block, their Kind, Namespace, and Group are added to the respective Bloom Filters.
2.  **Read Path**: When checking if a block is a candidate for `kind="Deployment"`, the engine checks `BloomFilterKinds.Contains("Deployment")`.
    *   If it returns `false`, the block is **skipped** (optimization).
    *   If it returns `true`, the block is a candidate (subject to Inverted Index confirmation or direct inspection).

The Bloom Filters are serialized to JSON with the bitset encoded as Base64.

## Crash Recovery & Restore

The engine is designed to handle restarts and crashes gracefully.

### Normal Restart
1.  The engine checks for the existence of the storage file.
2.  It reads the **File Footer** to verify the file was closed properly.
3.  It reads the **Index Section** using the offset from the footer.
4.  It restores the in-memory **Inverted Index** and **Block Metadata**.
5.  It **truncates** the file to remove the old Index Section and Footer.
6.  The file is now ready for appending new blocks.

### Crash Detection (Incomplete Files)
If the application crashes, the file will likely be missing the Footer and Index Section.
1.  The engine attempts to read the Footer.
2.  If the Footer is missing or invalid (Magic Bytes don't match), the file is marked as **Incomplete**.
3.  **Action**: The incomplete file is renamed to `filename.incomplete.<timestamp>` to preserve data.
4.  A new, empty file is created for writing.

### Corruption
If the file exists but the header or structure is invalid, it is renamed to `filename.corrupted.<timestamp>`.
