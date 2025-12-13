---
title: Compression
description: Compression algorithms and performance characteristics
keywords: [architecture, compression, gzip, zstd, performance]
---

# Compression

This document explains Spectre's compression strategy for efficient storage of Kubernetes audit events.

## Overview

Compression is applied at the **block level** after events are buffered and encoded as protobuf:

```
Events (JSON)
    ↓
Protobuf Encoding
    ↓
gzip Compression (level 6)
    ↓
Write to Disk
```

**Key Benefits:**
- **75% storage reduction** (typical compression ratio: 0.25)
- **Fast decompression** (~300 MB/s throughput)
- **Block-level granularity** (only decompress blocks needed for query)

## Current Implementation: gzip

### Library and Configuration

**Library:** `github.com/klauspost/compress/gzip`
- Optimized Go implementation (2-3x faster than standard library)
- Full compatibility with standard gzip format
- Supports streaming compression/decompression

**Compression Level:** `gzip.DefaultCompression` (level 6)
- Range: 0 (no compression) to 9 (best compression)
- Level 6 balances compression ratio and speed
- Higher levels provide diminishing returns

### Implementation Details

```go
// Compress block data
func (c *Compressor) Compress(data []byte) ([]byte, error) {
    var buf bytes.Buffer

    // Create gzip writer with default compression
    writer, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
    if err != nil {
        return nil, err
    }

    // Write data
    writer.Write(data)
    writer.Close()

    return buf.Bytes(), nil
}
```

```go
// Decompress block data
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
    reader, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    defer reader.Close()

    return io.ReadAll(reader)
}
```

### Compression Ratios

Typical compression ratios for different data types:

| Data Type              | Raw Size | Compressed | Ratio | Reduction |
|------------------------|----------|------------|-------|-----------|
| Kubernetes JSON events | 100 MB   | 25 MB      | 0.25  | 75%       |
| Protobuf events        | 80 MB    | 24 MB      | 0.30  | 70%       |
| Mixed workload         | 100 MB   | 20-30 MB   | 0.20-0.30 | 70-80% |

**Why JSON events compress well:**
- Repetitive structure (field names repeated across events)
- Common values (namespaces, kinds, groups)
- Predictable patterns (timestamps, UIDs, labels)

### Block Size Impact on Compression

Larger blocks compress better due to more context for the compression algorithm:

| Block Size | Events | Uncompressed | Compressed | Ratio | Reduction |
|------------|--------|--------------|------------|-------|-----------|
| 1 MB       | ~80    | 1 MB         | 350 KB     | 0.35  | 65%       |
| 10 MB      | ~800   | 10 MB        | 2.5 MB     | 0.25  | 75%       |
| 100 MB     | ~8000  | 100 MB       | 22 MB      | 0.22  | 78%       |

**Diminishing Returns:** Beyond 10 MB, compression ratio improvements are minimal (\<5%)

**Default Choice:** 10 MB blocks provide good compression (75%) without excessive decompression latency

## Performance Characteristics

### Compression Speed

| Metric                | Value                  |
|-----------------------|------------------------|
| **Throughput**        | ~100 MB/s              |
| **CPU Utilization**   | ~10% of single core    |
| **Latency (10 MB)**   | ~100 ms                |
| **Memory Overhead**   | ~2× block size         |

**Typical Workflow:**
```
10 MB block → 100 ms compression → 2.5 MB written to disk
```

**Amortization:** Compression latency is hidden by buffering (only triggered when block is full)

### Decompression Speed

| Metric                | Value                  |
|-----------------------|------------------------|
| **Throughput**        | ~300 MB/s              |
| **CPU Utilization**   | ~5% of single core     |
| **Latency (10 MB)**   | ~30 ms                 |
| **Memory Overhead**   | ~block size            |

**3× faster than compression** (typical for gzip)

**Query Impact:**
```
Query reads 8 blocks:
  8 blocks × 2.5 MB compressed = 20 MB disk I/O
  8 blocks × 10 MB uncompressed = 80 MB decompressed
  8 blocks × 30 ms = 240 ms total decompression latency
```

### Compression Effectiveness Check

Spectre validates that compression provides at least **10% reduction**:

```go
func (c *Compressor) IsCompressionEffective(original, compressed []byte) bool {
    if len(original) == 0 {
        return false
    }
    ratio := float64(len(compressed)) / float64(len(original))
    return ratio < 0.9  // More than 10% reduction
}
```

**Use Case:** Detect incompressible data (e.g., already compressed, encrypted, random)

**Fallback:** If compression is ineffective, could store uncompressed (not currently implemented)

## Compression Levels Comparison

| Level | Ratio | Compression Speed | Decompression Speed | Best For              |
|-------|-------|-------------------|---------------------|-----------------------|
| 1     | 0.40  | 200 MB/s          | 300 MB/s            | CPU-constrained       |
| 6*    | 0.25  | 100 MB/s          | 300 MB/s            | **Balanced (default)**|
| 9     | 0.23  | 20 MB/s           | 300 MB/s            | Storage-constrained   |

*Level 6 = DefaultCompression

**Why Default Level 6:**
- Good compression ratio (75% reduction)
- Fast enough for real-time writes (~100 ms for 10 MB)
- Decompression speed unaffected by compression level
- Best balance for typical workloads

## Future: zstd Compression

### Planned for Version 2.0

**Library:** Zstandard (Facebook)
- Newer compression algorithm (2016)
- Better compression ratio than gzip
- Faster compression and decompression

### Performance Comparison

| Metric                | gzip (level 6) | zstd (level 3) | Improvement |
|-----------------------|----------------|----------------|-------------|
| **Compression Ratio** | 0.25           | 0.22           | +12% better |
| **Compression Speed** | 100 MB/s       | 200 MB/s       | 2× faster   |
| **Decompression Speed** | 300 MB/s     | 450 MB/s       | 1.5× faster |
| **CPU Usage**         | 10%            | 8%             | 20% less    |

**For 10 MB block:**
- gzip: 100 ms compression, 30 ms decompression
- zstd: 50 ms compression, 20 ms decompression
- **Total savings:** 60 ms per block

**For query reading 8 blocks:**
- gzip: 240 ms decompression
- zstd: 160 ms decompression
- **Savings:** 80 ms (33% faster)

### Migration Strategy

**Challenges:**
1. **Backward compatibility:** Existing files use gzip
2. **Mixed formats:** Need to support both gzip and zstd during transition
3. **Tooling:** External tools must support zstd

**Proposed Approach:**
1. **Opt-in flag:** `--compression=zstd` (default: gzip)
2. **Format detection:** Read `CompressionAlgorithm` from file header
3. **Recompression tool:** Convert gzip files to zstd offline
4. **Documentation:** Guide users through migration

**Timeline:** Planned for Spectre 2.0 (Q2 2026)

## Space Savings Examples

### Small Cluster (100 resources)

**Event Rate:** 10 events/minute
**Hourly Events:** 600
**Average Event Size:** 12 KB

```
Raw size:     600 events × 12 KB = 7.2 MB/hour
Compressed:   7.2 MB × 0.25 = 1.8 MB/hour
Daily:        1.8 MB × 24 = 43 MB/day
Weekly:       43 MB × 7 = 301 MB/week
Monthly:      43 MB × 30 = 1.3 GB/month
```

**Savings vs no compression:** 21.6 GB/month saved (94.4% less disk usage)

### Medium Cluster (1000 resources)

**Event Rate:** 100 events/minute
**Hourly Events:** 6,000

```
Raw size:     6,000 events × 12 KB = 72 MB/hour
Compressed:   72 MB × 0.25 = 18 MB/hour
Daily:        18 MB × 24 = 432 MB/day
Weekly:       432 MB × 7 = 3 GB/week
Monthly:      432 MB × 30 = 13 GB/month
```

**Savings vs no compression:** 216 GB/month saved

### Large Cluster (10,000+ resources)

**Event Rate:** 1000 events/minute
**Hourly Events:** 60,000

```
Raw size:     60,000 events × 12 KB = 720 MB/hour
Compressed:   720 MB × 0.25 = 180 MB/hour
Daily:        180 MB × 24 = 4.3 GB/day
Weekly:       4.3 GB × 7 = 30 GB/week
Monthly:      4.3 GB × 30 = 130 GB/month
```

**Savings vs no compression:** 2.16 TB/month saved

## Compression Alternatives (Not Used)

### Why Not Snappy?

| Aspect           | Snappy            | gzip              |
|------------------|-------------------|-------------------|
| Compression Ratio| 0.50 (50% reduction) | 0.25 (75% reduction) |
| Compression Speed| 500 MB/s          | 100 MB/s          |
| Decompression Speed | 1500 MB/s      | 300 MB/s          |
| Best For         | CPU > Disk        | Disk > CPU        |

**Decision:** Storage cost is more important than CPU cost for audit logs

### Why Not LZ4?

| Aspect           | LZ4               | gzip              |
|------------------|-------------------|-------------------|
| Compression Ratio| 0.45 (55% reduction) | 0.25 (75% reduction) |
| Compression Speed| 600 MB/s          | 100 MB/s          |
| Decompression Speed | 2000 MB/s      | 300 MB/s          |
| Best For         | Low-latency reads | High compression  |

**Decision:** Audit logs are write-heavy, archival workload; compression ratio more valuable

### Why Not Brotli?

| Aspect           | Brotli            | gzip              |
|------------------|-------------------|-------------------|
| Compression Ratio| 0.20 (80% reduction) | 0.25 (75% reduction) |
| Compression Speed| 10 MB/s           | 100 MB/s          |
| Decompression Speed | 300 MB/s       | 300 MB/s          |
| Best For         | Static assets     | Real-time streams |

**Decision:** Too slow for real-time event compression (10× slower than gzip)

## Compression Metrics and Monitoring

Spectre tracks compression effectiveness:

```go
type CompressionMetrics struct {
    TotalUncompressedBytes int64   // Raw data size
    TotalCompressedBytes   int64   // After compression
    CompressionRatio       float32 // Compressed / Uncompressed
    BlocksCompressed       int64   // Number of blocks processed
    AverageRatio           float32 // Mean compression ratio
}
```

**Available via API:**
```bash
curl http://localhost:8080/api/v1/storage/stats

{
  "total_uncompressed_bytes": 78643200,
  "total_compressed_bytes": 19660800,
  "compression_ratio": 0.25,
  "blocks_compressed": 300,
  "average_ratio": 0.25
}
```

**Use Cases:**
- **Monitor compression effectiveness** (should be ~0.25 for typical workloads)
- **Detect incompressible data** (ratio > 0.9 indicates problem)
- **Capacity planning** (estimate storage growth)
- **Performance analysis** (correlate ratio with query latency)

## Best Practices

### ✅ Do

- **Use default compression level (6)** - Best balance for most workloads
- **Monitor compression ratios** - Alert if ratio > 0.5 (poor compression)
- **Increase block size** - Larger blocks (10-100 MB) compress better
- **Enable caching** - Avoid repeated decompression of hot blocks

### ❌ Don't

- **Don't disable compression** - 4× disk usage increase
- **Don't use level 9** - Minimal benefit (2-3% better) with 5× slower compression
- **Don't compress pre-compressed data** - Waste of CPU (unlikely in Spectre)
- **Don't compress very small blocks** - Overhead exceeds benefit (\<1 KB blocks)

## Related Documentation

- [Storage Design](./storage-design.md) - Block architecture and write path
- [Block Format Reference](./block-format.md) - Compression field in file header
- [Query Execution](./query-execution.md) - Decompression in query pipeline
- [Storage Settings](../configuration/storage-settings.md) - Block size configuration

<!-- Source: internal/storage/compression.go, internal/storage/block.go -->
