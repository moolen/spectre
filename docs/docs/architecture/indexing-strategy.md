---
title: Indexing Strategy
description: Query optimization through inverted indexes and Bloom filters
keywords: [architecture, indexing, bloom filters, query optimization]
---

# Indexing Strategy

This document explains Spectre's indexing strategy for fast query execution. The goal is to **skip 90%+ of blocks** for filtered queries without scanning the entire dataset.

## Three-Tier Indexing Architecture

Spectre uses a layered filtering approach:

```
┌─────────────────────────────────────────────────────────────┐
│               Query: kind=Pod, namespace=default            │
└────────────────────────┬────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────┐
│  Tier 1: Inverted Indexes (Exact Match)                     │
│  kind_to_blocks["Pod"] = [0, 1, 3, 7, 9]                   │
│  namespace_to_blocks["default"] = [0, 2, 4, 6]             │
│  Intersection: [0]  → Skip 99% of blocks                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────┐
│  Tier 2: Bloom Filters (Probabilistic)                      │
│  block[0].bloomKinds.Contains("Pod")? → true                │
│  block[0].bloomNamespaces.Contains("default")? → true       │
│  → Candidate for decompression                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────┐
│  Tier 3: Timestamp Filtering (Range Check)                  │
│  block[0].TimestampMin ≤ queryEnd? → true                   │
│  block[0].TimestampMax ≥ queryStart? → true                 │
│  → Decompress and scan events                               │
└────────────────────────────────────────────────────────────┘
```

## Inverted Indexes

### Structure

Inverted indexes map resource attribute values directly to the list of block IDs containing them:

```go
type InvertedIndex struct {
    // Maps kind → block IDs
    KindToBlocks map[string][]int32

    // Maps namespace → block IDs
    NamespaceToBlocks map[string][]int32

    // Maps API group → block IDs
    GroupToBlocks map[string][]int32
}
```

**Example Index:**
```json
{
  "kind_to_blocks": {
    "Pod": [0, 1, 3, 7, 9, 12, 15],
    "Deployment": [0, 2, 5, 8, 11],
    "Service": [0, 4, 6, 10, 14],
    "ConfigMap": [1, 3, 5, 7, 9, 11, 13]
  },
  "namespace_to_blocks": {
    "default": [0, 1, 2, 3, 4, 5],
    "kube-system": [6, 7, 8, 9, 10],
    "production": [11, 12, 13, 14, 15]
  },
  "group_to_blocks": {
    "": [0, 1, 3, 7],           // core API group
    "apps": [2, 5, 8, 11, 14],
    "batch": [4, 6, 9, 12]
  }
}
```

### Query Optimization

#### Single Filter

**Query:** `kind=Pod`

```go
candidateBlocks := index.KindToBlocks["Pod"]
// Result: [0, 1, 3, 7, 9, 12, 15]
// Skip: 53% of blocks (8 out of 15 skipped)
```

#### Multiple Filters (AND Logic)

**Query:** `kind=Pod AND namespace=default`

```go
// Step 1: Lookup each filter dimension
kindBlocks := index.KindToBlocks["Pod"]
// [0, 1, 3, 7, 9, 12, 15]

namespaceBlocks := index.NamespaceToBlocks["default"]
// [0, 1, 2, 3, 4, 5]

// Step 2: Compute intersection
candidateBlocks := Intersect(kindBlocks, namespaceBlocks)
// [0, 1, 3] (only blocks with BOTH Pod AND default)

// Skip: 80% of blocks (3 candidates out of 16 total)
```

#### Three-Way Intersection

**Query:** `kind=Deployment AND namespace=production AND group=apps`

```go
kindBlocks := [0, 2, 5, 8, 11]
namespaceBlocks := [11, 12, 13, 14, 15]
groupBlocks := [2, 5, 8, 11, 14]

// Intersection: [11]
// Skip: 93% of blocks (1 candidate out of 16 total)
```

### Intersection Algorithm

```go
func GetCandidateBlocks(index *InvertedIndex, filters map[string]string) []int32 {
    var candidates map[int32]bool

    // For each filter dimension
    for dimension, value := range filters {
        var dimensionBlocks []int32

        switch dimension {
        case "kind":
            dimensionBlocks = index.KindToBlocks[value]
        case "namespace":
            dimensionBlocks = index.NamespaceToBlocks[value]
        case "group":
            dimensionBlocks = index.GroupToBlocks[value]
        }

        // If no blocks contain this value, return empty (early exit)
        if len(dimensionBlocks) == 0 {
            return nil
        }

        if candidates == nil {
            // First filter: Initialize candidates
            candidates = make(map[int32]bool)
            for _, blockID := range dimensionBlocks {
                candidates[blockID] = true
            }
        } else {
            // Subsequent filters: Intersect with existing candidates
            newCandidates := make(map[int32]bool)
            for _, blockID := range dimensionBlocks {
                if candidates[blockID] {
                    newCandidates[blockID] = true
                }
            }
            candidates = newCandidates
        }

        // Early exit if no candidates remain
        if len(candidates) == 0 {
            return nil
        }
    }

    // Convert map to sorted slice
    result := make([]int32, 0, len(candidates))
    for blockID := range candidates {
        result = append(result, blockID)
    }
    return result
}
```

**Complexity:** O(F × N) where F = number of filters, N = average blocks per filter value

**Optimization:** Filters are evaluated in sequence with early exit if intersection becomes empty.

### Index Build Performance

Built at file close time from block metadata:

```go
func BuildInvertedIndexes(blocks []*BlockMetadata) *InvertedIndex {
    index := &InvertedIndex{
        KindToBlocks:      make(map[string][]int32),
        NamespaceToBlocks: make(map[string][]int32),
        GroupToBlocks:     make(map[string][]int32),
    }

    for _, block := range blocks {
        // Add all kinds from this block
        for _, kind := range block.KindSet {
            index.KindToBlocks[kind] = append(index.KindToBlocks[kind], block.ID)
        }

        // Add all namespaces from this block
        for _, ns := range block.NamespaceSet {
            index.NamespaceToBlocks[ns] = append(index.NamespaceToBlocks[ns], block.ID)
        }

        // Add all groups from this block
        for _, group := range block.GroupSet {
            index.GroupToBlocks[group] = append(index.GroupToBlocks[group], block.ID)
        }
    }

    return index
}
```

**Complexity:** O(B × V) where B = number of blocks, V = average unique values per block

**Typical Performance:** \<500ms for 300 blocks, 60K events (hourly file)

## Bloom Filters

### Purpose

Bloom filters provide **space-efficient probabilistic filtering** for each block without storing complete value lists.

**Key Property:**
- **False Positives:** Possible (might say "yes" when answer is "no")
- **False Negatives:** Impossible (never says "no" when answer is "yes")

**Use Case:** Quickly eliminate blocks that definitely don't contain a value.

### Configuration

Each block has three Bloom filters:

```go
type BlockMetadata struct {
    BloomFilterKinds      *StandardBloomFilter  // For resource kinds
    BloomFilterNamespaces *StandardBloomFilter  // For namespaces
    BloomFilterGroups     *StandardBloomFilter  // For API groups
    // ...
}
```

**Filter Parameters:**

| Filter Type  | Expected Elements | False Positive Rate | Hash Functions | Bit Array Size |
|--------------|-------------------|---------------------|----------------|----------------|
| Kinds        | 1000              | 0.05 (5%)           | ~4             | ~1.2 KB        |
| Namespaces   | 100               | 0.05 (5%)           | ~4             | ~120 bytes     |
| Groups       | 100               | 0.05 (5%)           | ~4             | ~120 bytes     |

**Total Overhead:** ~1.5 KB per block (minimal compared to 10 MB block data)

### How Bloom Filters Work

#### Adding Values (Write Time)

```go
// When building a block, add each value to its Bloom filter
for _, event := range events {
    block.BloomFilterKinds.Add(event.Resource.Kind)
    block.BloomFilterNamespaces.Add(event.Resource.Namespace)
    block.BloomFilterGroups.Add(event.Resource.Group)
}
```

**Process:**
1. Hash the value with K hash functions (typically 4)
2. Set K bits in the bit array to 1
3. Repeat for all values

#### Checking Values (Query Time)

```go
// Check if block might contain a value
if !block.BloomFilterKinds.Contains("Pod") {
    // Definitely does NOT contain "Pod" → skip this block
    return false
}

// Might contain "Pod" (or false positive) → need to check further
```

**Process:**
1. Hash the query value with same K hash functions
2. Check if all K bits are set to 1
3. If any bit is 0: **definitely not present** (skip block)
4. If all bits are 1: **maybe present** (check with inverted index or decompress)

### False Positive Rate

**Single Filter:** 5% (configured)

**Combined (3 filters with AND logic):**
```
P(false positive) = 1 - (1 - 0.05)³
                  = 1 - 0.857
                  ≈ 14.3%
```

**Impact:** Out of 100 blocks, ~14 might be false positives (scanned unnecessarily)

**Acceptable Trade-off:** 14% extra decompression vs 100% without filtering

### Space Efficiency Comparison

For a block with 800 events, 50 unique kinds, 10 namespaces, 5 groups:

| Approach              | Storage Size | Lookup Speed |
|-----------------------|--------------|--------------|
| **Exact Sets**        | ~2 KB        | O(N)         |
| **Bloom Filters**     | ~1.5 KB      | O(k) = O(1)  |
| **No Filter**         | 0 bytes      | Decompress   |

**Bloom filters save space while providing fast lookups (4-5 hash operations vs decompressing 10 MB).**

## Timestamp Indexes

### Block-Level Time Ranges

Each block metadata stores the min/max event timestamps:

```go
type BlockMetadata struct {
    TimestampMin int64  // Earliest event in block (nanoseconds)
    TimestampMax int64  // Latest event in block (nanoseconds)
    // ...
}
```

### Time Range Filtering

**Query:** `startTime=1733915200000000000, endTime=1733918800000000000`

```go
for _, blockMeta := range blocks {
    // Check if block overlaps query time range
    if blockMeta.TimestampMax < query.StartTime {
        continue  // Block ends before query starts → skip
    }
    if blockMeta.TimestampMin > query.EndTime {
        continue  // Block starts after query ends → skip
    }

    // Block overlaps query range → candidate for reading
    candidateBlocks = append(candidateBlocks, blockMeta.ID)
}
```

**Complexity:** O(B) where B = number of blocks

**Typical Skip Rate:** 30-60% for time-limited queries (e.g., last 1 hour out of 24-hour file)

## Multi-Stage Filtering Pipeline

### Complete Query Execution Flow

```
┌────────────────────────────────────────────────────────────┐
│ Query: kind=Pod, namespace=default, time=[10:00, 11:00]   │
└───────────────────────┬────────────────────────────────────┘
                        │
                        v
┌────────────────────────────────────────────────────────────┐
│ Stage 1: File Selection (by hour)                          │
│ All files: 24 hourly files (1 day)                         │
│ Filtered: 2 files (10:00-10:59, 11:00-11:59)              │
│ Skip Rate: 91% of files (22 skipped)                       │
└───────────────────────┬────────────────────────────────────┘
                        │
                        v
┌────────────────────────────────────────────────────────────┐
│ Stage 2: Inverted Index Filtering                          │
│ Total blocks: 600 (2 files × 300 blocks/file)              │
│ kind=Pod: [0-50] (50 blocks)                               │
│ namespace=default: [0-30] (30 blocks)                      │
│ Intersection: [0-15] (15 blocks)                           │
│ Skip Rate: 97.5% of blocks (585 skipped)                   │
└───────────────────────┬────────────────────────────────────┘
                        │
                        v
┌────────────────────────────────────────────────────────────┐
│ Stage 3: Bloom Filter Verification                         │
│ Candidates: 15 blocks                                       │
│ Bloom filter checks: 15 × 2 filters = 30 checks            │
│ False positives: ~2 blocks (14.3% FP rate)                 │
│ True candidates: 13 blocks                                  │
│ Skip Rate: 13% additional (2 blocks)                        │
└───────────────────────┬────────────────────────────────────┘
                        │
                        v
┌────────────────────────────────────────────────────────────┐
│ Stage 4: Timestamp Filtering                               │
│ Candidates: 13 blocks                                       │
│ Time range overlap: 8 blocks                                │
│ Skip Rate: 38% additional (5 blocks)                        │
└───────────────────────┬────────────────────────────────────┘
                        │
                        v
┌────────────────────────────────────────────────────────────┐
│ Stage 5: Decompression & Event Scanning                    │
│ Blocks to decompress: 8 (1.3% of original 600)             │
│ Overall Skip Rate: 98.7%                                    │
└────────────────────────────────────────────────────────────┘
```

### Performance Metrics

| Stage                 | Input    | Output   | Skip Rate | Latency  |
|-----------------------|----------|----------|-----------|----------|
| File Selection        | 24 files | 2 files  | 91%       | \<1 ms    |
| Inverted Index        | 600 blocks | 15 blocks | 97.5%   | ~2 ms    |
| Bloom Filters         | 15 blocks | 13 blocks | 13%      | \<1 ms    |
| Timestamp Filter      | 13 blocks | 8 blocks  | 38%      | \<1 ms    |
| Decompression         | 8 blocks  | 8 blocks  | 0%       | ~240 ms  |
| **Total**             | **600**   | **8**     | **98.7%** | **~245 ms** |

**Result:** Query processes only 1.3% of total blocks, dramatically reducing I/O and decompression overhead.

## Index Memory Footprint

### Per-File Index Size

For a typical hourly file with 300 blocks:

| Component             | Size       | Description                           |
|-----------------------|------------|---------------------------------------|
| Block Metadata Array  | ~600 KB    | 300 blocks × 2 KB metadata each       |
| Inverted Indexes      | ~200 KB    | Maps for kinds, namespaces, groups    |
| Bloom Filters         | ~450 KB    | 300 blocks × 1.5 KB filters           |
| Statistics            | ~1 KB      | File-level stats                      |
| **Total Index**       | **~1.25 MB** | Per hourly file                     |

### System-Wide Memory Usage

**Scenario:** 168 hourly files (1 week retention)

```
Total Index Memory = 168 files × 1.25 MB/file
                   = 210 MB
```

**Optimization:** Indexes loaded on-demand (only for files matching query time range)

**Typical Query:** Loads 1-24 files → 1.25-30 MB index memory

## Index Persistence

Indexes are **stored in the file** (not external):

```
File Structure:
├─ Header (77 bytes)
├─ Blocks (compressed events)
├─ Index Section (JSON)  ← Inverted indexes stored here
│   ├─ Block Metadata (with Bloom filters)
│   ├─ Inverted Indexes (kind/namespace/group maps)
│   └─ Statistics
└─ Footer (324 bytes)
```

**Benefits:**
- **No external dependencies:** No separate index database
- **Portability:** Copy file, get indexes for free
- **Crash safety:** Indexes built atomically at file close
- **Version alignment:** Index always matches block data

## Query Without Indexes

**What if indexes are missing or corrupted?**

Fallback: Sequential scan of all blocks

```
1. Read all block metadata (still available in footer)
2. Decompress each block
3. Scan events linearly
4. Filter in-memory

Performance: 100× slower but still works
```

**Use Case:** Recovery tool for corrupted index sections

## Related Documentation

- [Storage Design](./storage-design.md) - Overall architecture
- [Block Format Reference](./block-format.md) - Index section format
- [Query Execution](./query-execution.md) - Complete query pipeline
- [Compression](./compression.md) - Block compression details

<!-- Source: internal/storage/block_format.go, internal/storage/query.go, internal/storage/README.md -->
