---
title: Architecture Overview
description: High-level architecture of Spectre's event monitoring system
keywords: [architecture, design, system, components, overview]
---

# Architecture Overview

Spectre is a Kubernetes event monitoring system that captures all resource changes across a cluster and provides fast, queryable access to historical data through efficient storage and indexing.

## System Purpose

Spectre solves the problem of understanding *what happened* in a Kubernetes cluster:
- **Incident investigation** - Reconstruct timelines from events
- **Post-mortem analysis** - Analyze root causes with complete history
- **Deployment tracking** - Monitor rollouts and detect issues early
- **Compliance auditing** - Record all resource modifications

Unlike traditional logging or metrics systems, Spectre focuses on **resource lifecycle events** - the who, what, when, and why of every Kubernetes object change.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│           Spectre Event Monitoring System                    │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  K8s Watcher │  │ K8s Watcher  │  │ K8s Watcher  │      │
│  │  (Pods)      │  │ (Deployments)│  │  (Services)  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         └──────────────────┼──────────────────┘              │
│                            │ Events                          │
│                   ┌────────▼────────┐                        │
│                   │  Event Queue    │                        │
│                   │ (Concurrent)    │                        │
│                   └────────┬────────┘                        │
│                            │                                 │
│              ┌─────────────┴─────────────┐                  │
│              │ Pruning & Validation      │                  │
│              │ (Remove managedFields)    │                  │
│              └─────────────┬─────────────┘                  │
│                            │ Events                          │
│              ┌─────────────▼─────────────┐                  │
│              │  Storage Layer            │                  │
│              │  ┌──────────────────────┐ │                  │
│              │  │ Hourly Files         │ │                  │
│              │  │  ├─ File Header      │ │                  │
│              │  │  ├─ Blocks           │ │                  │
│              │  │  │  ├─ Compressed    │ │                  │
│              │  │  │  │   Data         │ │                  │
│              │  │  │  └─ Metadata      │ │                  │
│              │  │  ├─ Index Section    │ │                  │
│              │  │  │  ├─ Timestamp     │ │                  │
│              │  │  │  │   Index        │ │                  │
│              │  │  │  └─ Inverted      │ │                  │
│              │  │  │      Index        │ │                  │
│              │  │  └─ File Footer      │ │                  │
│              │  └──────────────────────┘ │                  │
│              └────────────┬────────────────┘                  │
│                           │                                 │
│              ┌────────────▼────────────┐                   │
│              │  Query Engine           │                   │
│              │  ├─ File Selection      │                   │
│              │  ├─ Block Filtering     │                   │
│              │  ├─ Decompression       │                   │
│              │  └─ Result Aggregation  │                   │
│              └────────────┬────────────┘                   │
│                           │ Query Results                   │
│              ┌────────────▼────────────┐                   │
│              │  HTTP API Server        │                   │
│              │  /api/search            │                   │
│              └─────────────────────────┘                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Watcher Component

**Purpose**: Capture Kubernetes resource changes in real-time

**Location**: `internal/watcher/`

**Responsibilities**:
- Establish watches on configured resource types (Pods, Deployments, Services, etc.)
- Receive ADD, UPDATE, DELETE events from Kubernetes API
- Buffer events in concurrent queue for batch processing
- Prune large metadata fields (`managedFields`) to reduce size
- Validate events before passing to storage

**Key Features**:
- Parallel watches for multiple resource types
- Concurrent event handling without loss
- 80-90% size reduction through field pruning
- Configurable resource type selection via YAML

**Related**: [Watcher Configuration](../configuration/watcher-config.md)

### 2. Storage Component

**Purpose**: Persist events with compression and indexing for fast retrieval

**Location**: `internal/storage/`

**Responsibilities**:
- Organize events into hourly files (immutable after hour completion)
- Compress events into fixed-size blocks (default 256KB)
- Build inverted indexes for multi-dimensional filtering
- Create sparse timestamp index for binary search
- Manage file lifecycle and retention policies

**Key Features**:
- 90%+ compression ratio (gzip on JSON events)
- Block-based storage enables selective decompression
- Inverted indexes skip 50-90% of blocks during queries
- MD5 checksums for data integrity validation
- Format versioning for backward compatibility

**Related**:
- [Storage Design](./storage-design.md)
- [Block Format](./block-format.md)
- [Indexing Strategy](./indexing-strategy.md)
- [Compression](./compression.md)

### 3. Query Engine

**Purpose**: Execute fast queries with filtering and optimization

**Location**: `internal/storage/query.go`, `internal/storage/filters.go`

**Responsibilities**:
- Select relevant files based on time window
- Use sparse index for binary search of timestamps
- Apply inverted indexes to skip non-matching blocks
- Decompress only candidate blocks
- Filter events by resource attributes
- Aggregate results from multiple files/blocks

**Key Features**:
- O(log N) timestamp lookups in sparse index
- 50-90% block skipping through inverted indexes
- Parallel file reading for multi-day queries
- Early termination when result limits reached
- AND semantics for multi-filter queries

**Related**: [Query Execution](./query-execution.md)

### 4. HTTP API Server

**Purpose**: Provide queryable interface for event retrieval

**Location**: `internal/api/`

**Responsibilities**:
- Expose `/api/search` endpoint for time-windowed queries
- Validate query parameters (time range, filters)
- Format results as JSON with execution metrics
- Handle CORS and request authentication
- Serve static UI assets

**API Specification**:
```
GET /api/search

Query Parameters:
  start (required)    : Unix timestamp (seconds or milliseconds)
  end (required)      : Unix timestamp (seconds or milliseconds)
  kind (optional)     : Resource kind (e.g., "Pod", "Deployment")
  namespace (optional): Kubernetes namespace
  group (optional)    : API group (e.g., "apps")
  version (optional)  : API version (e.g., "v1")

Response:
{
  "events": [...],                 // Array of matching events
  "count": 100,                    // Total events returned
  "executionTimeMs": 45,           // Query execution time
  "filesSearched": 24,             // Files accessed
  "segmentsScanned": 12,           // Blocks decompressed
  "segmentsSkipped": 88            // Blocks skipped via index
}
```

**Related**: [User Guide](../user-guide/index.md)

### 5. MCP Server (Optional)

**Purpose**: Enable AI-assisted investigations through Model Context Protocol

**Location**: `internal/mcp/`

**Responsibilities**:
- Expose 3 investigation tools (cluster_health, resource_changes, investigate)
- Provide 2 structured prompts (post-mortem analysis, live incident handling)
- Translate natural language queries to API calls
- Format responses for LLM consumption
- Support HTTP and stdio transports

**Key Features**:
- Conversational incident investigation with Claude
- Automated event correlation and timeline reconstruction
- Structured post-mortem report generation
- Read-only access (no cluster modifications)

**Related**: [MCP Integration](../mcp-integration/index.md)

## Data Flow

### Write Path (Events → Storage)

```
Kubernetes Event
    ↓
Watcher receives (ADD/UPDATE/DELETE)
    ↓
Event Queue (concurrent buffer)
    ↓
Pruning (remove managedFields, ~80% size reduction)
    ↓
Validation (check required fields)
    ↓
Storage Write
    ├─ Accumulate in EventBuffer
    ├─ When buffer full (256KB default):
    │   ├─ Create Block
    │   ├─ Compress with gzip (~90% reduction)
    │   ├─ Create metadata (bloom filters, sets)
    │   ├─ Compute checksum (MD5)
    │   └─ Write block to hourly file
    └─
    When hourly boundary crossed:
    ├─ Build inverted indexes (kind → blocks, namespace → blocks, group → blocks)
    ├─ Create index section (JSON)
    ├─ Write file footer
    └─ Seal file (immutable, enables concurrent reads)
```

**Throughput**: 139K events/sec sustained write rate

### Read Path (Query → Results)

```
HTTP API Request
    ↓
Validate parameters (time range, filters)
    ↓
Select files by time window (hourly granularity)
    ↓
For each file:
    ├─ Load header/footer (metadata)
    ├─ Load index section (sparse + inverted)
    ├─ Binary search timestamp index
    ├─ Intersect inverted indexes (kind ∩ namespace ∩ group)
    ├─ Skip non-matching blocks (50-90% reduction)
    ├─ Decompress candidate blocks
    ├─ Validate checksums
    ├─ Filter events by exact match
    └─ Aggregate results
    ↓
Combine results from all files
    ↓
Sort by timestamp (ascending)
    ↓
Format response (JSON with metrics)
    ↓
Return to client
```

**Latency**:
- Single hour (no filters): \<50ms
- Single hour (with filters): 10-20ms
- 24-hour window: 100-500ms
- 7-day window: \<2s

## Performance Characteristics

### Storage Efficiency

| Metric | Value | Notes |
|--------|-------|-------|
| Compression ratio | 7-10% | 90-93% reduction via gzip |
| Raw event size | ~2KB avg | Depends on resource type |
| Compressed event | ~200 bytes | After gzip compression |
| Block size | 256KB | Configurable (32KB-1MB) |
| Events per block | ~200-300 | Varies by resource type |
| Index overhead | ~1% | Inverted indexes + bloom filters |
| Bloom filter size | ~18KB/block | 5% false positive rate |

### Query Performance

| Scenario | Latency | Files | Blocks | Optimization |
|----------|---------|-------|--------|--------------|
| 1-hour window (no filters) | \<50ms | 1 | All (~300) | Minimal skipping |
| 1-hour window (kind filter) | 10-20ms | 1 | ~30 (10%) | Inverted index |
| 1-hour window (kind + ns) | 5-10ms | 1 | ~5 (2%) | Multi-index intersection |
| 24-hour window (filtered) | 100-200ms | 24 | ~120 (5%) | Parallel reads |
| 7-day window (filtered) | \<2s | 168 | ~500 (3%) | Parallel + early termination |

### Memory Usage

| Component | Memory | Notes |
|-----------|--------|-------|
| Base application | ~50MB | Runtime overhead |
| Per file loaded | ~10MB | Headers + indexes |
| Per decompressed block | ~256KB | Configurable block size |
| Event queue buffer | ~100MB | Configurable, high-throughput |
| Total (typical) | ~200MB | For active query workload |

### Throughput

| Operation | Rate | Notes |
|-----------|------|-------|
| Event ingestion | 139K events/sec | Sustained write throughput |
| Compression | >100MB/sec | Gzip via klauspost/compress |
| Decompression | >100MB/sec | Parallel block reads |
| Index lookup | \<1ms | O(log N) binary search |
| Block skip rate | 50-90% | With inverted indexes |

## Scalability Considerations

### Current Design: Single-Writer, Multi-Reader

**Write Path**:
- One Spectre instance per cluster captures events
- Events written to local storage (PersistentVolume)
- Hourly files sealed after completion (immutable)

**Read Path**:
- Multiple replicas can read same files concurrently
- File immutability enables safe parallel access
- No coordination required between readers

**Limitations**:
- Single writer per cluster (no horizontal write scaling)
- Storage limited to single PV capacity
- All data on one node (no distribution)

### Future Enhancements

**Multi-Writer Sharding** (planned):
- Shard by namespace or resource type
- Each writer handles subset of cluster
- Coordinated via consistent hashing

**Distributed Storage** (planned):
- S3-compatible object storage backend
- Decoupled storage from compute
- Multi-region replication

**Query Federation** (planned):
- Query across multiple clusters
- Aggregate results from federated sources
- Unified timeline view

## Design Principles

### 1. Write-Optimized

**Events are written once, read many times**:
- Batch writes into blocks for efficiency
- Immutable files after sealing
- No in-place updates or deletions

### 2. Index-Heavy

**Build rich indexes at write time for fast queries**:
- Inverted indexes enable block skipping
- Sparse timestamp index enables binary search
- Bloom filters reduce false positives
- Trade index build time (~500ms) for query speed (10-50ms)

### 3. Compression-First

**Storage is cheap, decompression is fast**:
- 90%+ compression via gzip
- Block-based compression enables selective decompression
- Only decompress candidate blocks (50-90% skipped)

### 4. Immutable Files

**Once sealed, files never change**:
- Enables concurrent reads without locking
- Simplifies retention and backup
- Atomic file replacement for reliability

### 5. Time-Partitioned

**Hourly files map to natural query patterns**:
- Most queries target recent time windows (hours/days)
- Time-based retention is straightforward
- Immutable hourly files enable simple cleanup

## Technology Stack

### Core Libraries

| Component | Library | Purpose |
|-----------|---------|---------|
| Kubernetes client | `k8s.io/client-go` | Watch resource changes |
| Compression | `klauspost/compress/gzip` | Fast gzip implementation |
| Bloom filters | `bits-and-blooms/bloom/v3` | Probabilistic set membership |
| HTTP server | `net/http` (stdlib) | API and UI serving |
| JSON parsing | `encoding/json` (stdlib) | Event serialization |
| Checksum | `crypto/md5` (stdlib) | Block integrity validation |
| MCP protocol | Custom JSON-RPC 2.0 | AI assistant integration |

### Language

**Go 1.21+**:
- High-performance I/O
- Excellent concurrency primitives (goroutines, channels)
- Static binaries for easy deployment
- Low memory overhead
- Rich Kubernetes ecosystem

## Related Documentation

- [Storage Design](./storage-design.md) - File organization and block structure
- [Block Format](./block-format.md) - Binary format specification
- [Indexing Strategy](./indexing-strategy.md) - Inverted indexes and bloom filters
- [Compression](./compression.md) - Compression algorithms and ratios
- [Query Execution](./query-execution.md) - Query processing pipeline
- [Data Flow](./data-flow.md) - Detailed write and read paths

<!-- Source: docs-backup/ARCHITECTURE.md, docs-backup/BLOCK_FORMAT_REFERENCE.md -->
