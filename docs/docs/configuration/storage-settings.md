---
title: Storage Settings
description: Configure storage, compression, and cache settings
keywords: [storage, configuration, cache, performance, disk]
---

# Storage Settings

This guide explains how to configure Spectre's storage system for optimal performance and disk usage.

## Overview

Spectre's storage system provides:
- **Persistent event storage** with efficient compression (75% reduction)
- **Fast query access** through inverted indexes and Bloom filters
- **Configurable caching** for improved query performance
- **Hourly file rotation** for flexible retention management

**For most users:** Default settings work well. Only adjust for specific needs (high volume, low disk space, read-heavy workloads).

**For technical details:** See [Storage Design](../architecture/storage-design.md)

## Quick Start

### Default Configuration

```bash
spectre server \
  --data-dir=/data \
  --segment-size=10485760 \
  --cache-enabled=true \
  --cache-max-mb=100
```

**What this provides:**
- Events stored in `/data` directory
- 10 MB blocks (balanced compression and query speed)
- 100 MB cache (improves repeated queries by 3×)
- ~75% disk space savings from compression

### Minimal Configuration

```bash
spectre server --data-dir=./data
```

All other settings use defaults.

## Storage Directory

### Configuration

**Flag:** `--data-dir`
**Type:** String
**Default:** `/data`
**Required:** Yes

**Purpose:** Directory where event files are stored

### File Organization

Events are automatically organized into hourly files:

```
/data/
├── 2025-12-12-10.bin  # Events from 10:00-10:59
├── 2025-12-12-11.bin  # Events from 11:00-11:59
├── 2025-12-12-12.bin  # Events from 12:00-12:59 (currently writing)
└── ...
```

**File Naming:** `YYYY-MM-DD-HH.bin`
**Rotation:** Automatic at hour boundaries

### Directory Requirements

| Requirement    | Value                                     |
|----------------|-------------------------------------------|
| Permissions    | Read/write for Spectre process            |
| Disk Type      | SSD recommended (faster queries)          |
| Filesystem     | ext4, xfs, or any POSIX filesystem        |
| Free Space     | Plan for retention × daily event volume   |

### Environment-Specific Examples

**Development (local):**
```bash
spectre server --data-dir=./data
```

**Docker:**
```bash
docker run -v /host/data:/data spectre server
```

**Kubernetes:**
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: spectre-storage
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
---
spec:
  containers:
  - name: spectre
    args:
      - server
      - --data-dir=/data
    volumeMounts:
    - name: storage
      mountPath: /data
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: spectre-storage
```

## Block Size Configuration

### Configuration

**Flag:** `--segment-size`
**Type:** int64 (bytes)
**Default:** `10485760` (10 MB)
**Range:** `1024` (1 KB) to `1073741824` (1 GB)

**Purpose:** Target size for uncompressed blocks before compression

### How Block Size Affects Performance

| Block Size | Compression | Query Speed | Disk I/O | Best For               |
|------------|-------------|-------------|----------|------------------------|
| 1 MB       | Good (65%)  | Slower      | More     | Low event rate         |
| 10 MB ✅    | Better (75%)| Balanced    | Balanced | **Most clusters**      |
| 100 MB     | Best (78%)  | Faster      | Less     | High volume clusters   |

**Default (10 MB) provides the best balance for typical Kubernetes clusters.**

### Events Per Block

**Formula:** `block_size / average_event_size`

**Typical Event Sizes:**
- Full event (with managedFields): ~50 KB
- Pruned event (without managedFields): ~12 KB

**Examples:**
```
1 MB block:  1,048,576 / 12,000 ≈ 87 events
10 MB block: 10,485,760 / 12,000 ≈ 874 events
100 MB block: 104,857,600 / 12,000 ≈ 8,738 events
```

### Configuration Examples

**Small Cluster (low event rate):**
```bash
spectre server --segment-size=1048576  # 1 MB
```

**Medium Cluster (default):**
```bash
spectre server --segment-size=10485760  # 10 MB
```

**Large Cluster (high volume):**
```bash
spectre server --segment-size=104857600  # 100 MB
```

## Block Cache Configuration

### Configuration

**Flags:**
- `--cache-enabled`: Enable/disable cache (default: `true`)
- `--cache-max-mb`: Maximum cache size in MB (default: `100`)

**Type:** Boolean, int64
**Purpose:** Cache decompressed blocks in memory for faster repeated queries

### Cache Behavior

**What is cached:** Decompressed blocks with parsed events
**Eviction Policy:** LRU (Least Recently Used)
**Thread Safety:** Concurrent reads supported

### Performance Impact

**Query Performance (1-hour query, 8 blocks):**

| Scenario         | Cache | Decompression Time | Total Query Time |
|------------------|-------|-------------------|--------------------|
| First query      | Cold  | 240 ms            | 280 ms             |
| Repeated query   | Warm  | 24 ms             | 64 ms              |
| **Improvement**  | -     | **10× faster**    | **4.4× faster**    |

**Cache Hit Rates:**
- Repeated queries: 90-95%
- Dashboard (5min refresh): 80-85%
- Ad-hoc queries: 10-20%

### Memory Usage

**Formula:** Cache memory = number of hot blocks × block size

**Example:**
```
Cache max: 100 MB
Block size: 10 MB
Hot blocks: ~10 blocks can fit in cache
```

**Planning:**
```
Read-light workload: 50-100 MB sufficient
Read-heavy workload: 200-500 MB recommended
Dashboard queries: 100-200 MB optimal
```

### Configuration Examples

**Disable Cache (memory-constrained):**
```bash
spectre server --cache-enabled=false
```

**Default Cache:**
```bash
spectre server --cache-enabled=true --cache-max-mb=100
```

**Large Cache (read-heavy workload):**
```bash
spectre server --cache-enabled=true --cache-max-mb=500
```

**Kubernetes Resource Limits:**
```yaml
resources:
  requests:
    memory: "256Mi"  # Base memory
  limits:
    memory: "756Mi"  # Base (256) + Cache (500)
```

## Disk Space Planning

### Storage Formula

```
disk_per_hour = events_per_hour × avg_event_size × compression_ratio
disk_per_day = disk_per_hour × 24
disk_total = disk_per_day × retention_days
```

**Assumptions:**
- Average event size: 12 KB (after managedFields pruning)
- Compression ratio: 0.25 (75% reduction)

### Event Rate Scenarios

| Cluster Size | Events/Min | Events/Hour | Raw/Hour | Compressed/Hour | Daily  | 7 Days | 30 Days |
|--------------|------------|-------------|----------|-----------------|--------|--------|---------|
| Small        | 10         | 600         | 7.2 MB   | 1.8 MB          | 43 MB  | 301 MB | 1.3 GB  |
| Medium       | 100        | 6,000       | 72 MB    | 18 MB           | 432 MB | 3 GB   | 13 GB   |
| Large        | 1,000      | 60,000      | 720 MB   | 180 MB          | 4.3 GB | 30 GB  | 130 GB  |
| Very Large   | 10,000     | 600,000     | 7.2 GB   | 1.8 GB          | 43 GB  | 301 GB | 1.3 TB  |

**Recommendation:** Add 20% buffer for overhead (indexes, metadata, state snapshots)

### Retention Planning

**Example (Medium Cluster):**
```
Daily storage: 432 MB

Retention policies:
  7 days:   432 MB × 7 = 3 GB
  30 days:  432 MB × 30 = 13 GB
  90 days:  432 MB × 90 = 39 GB
  1 year:   432 MB × 365 = 158 GB
```

**PVC Sizing (Kubernetes):**
```
Medium cluster, 30-day retention:
  Data: 13 GB
  Buffer: 20% = 2.6 GB
  Total: 16 GB → Request 20 GB PVC
```

### Monitoring Disk Usage

**Check current usage:**
```bash
du -sh /data
```

**List hourly files:**
```bash
ls -lh /data/*.bin
```

**Count events per file (requires jq):**
```bash
curl http://localhost:8080/api/v1/storage/stats
```

## Compression Settings

### Configuration

**Compression is automatic and not configurable via flags.**

**Algorithm:** gzip (level 6)
**Library:** klauspost/compress/gzip
**Typical Ratio:** 0.25 (75% reduction)

**Note:** File header defines compression algorithm, but only gzip is implemented in v1.0. zstd planned for v2.0.

### Compression Performance

| Metric                | Value              |
|-----------------------|--------------------|
| Compression Speed     | ~100 MB/s          |
| Decompression Speed   | ~300 MB/s          |
| CPU Usage             | ~10% (single core) |
| Typical Ratio         | 0.20-0.30          |

**Why these defaults:**
- gzip level 6 balances speed and compression
- Fast enough for real-time writes
- Universal compatibility
- Battle-tested in production

**For details:** See [Compression](../architecture/compression.md)

## Import/Export Configuration

### Bulk Import

**Flag:** `--import`
**Type:** String (file or directory path)
**Default:** `""` (disabled)

**Purpose:** Import historical events from JSON files at startup

**Examples:**

**Import single file:**
```bash
spectre server --import=/backups/events-2025-12-11.json
```

**Import directory:**
```bash
spectre server --import=/backups/december/
```

**Progress tracking:**
```
Importing events from directory: /backups/december/
  [1] Loaded 5000 events from events-01.json
  [2] Loaded 7500 events from events-02.json
  [3] Loaded 6200 events from events-03.json
...

Import Summary:
  Total Files: 31
  Imported: 31
  Total Events: 186,300
  Duration: 42.5s
```

## Configuration Examples

### Development (Local)

**Minimal setup for local testing:**

```bash
spectre server \
  --data-dir=./data \
  --segment-size=1048576 \     # 1 MB (faster rotation)
  --cache-max-mb=50             # 50 MB (low memory)
```

**Storage:** ~100 MB/day
**Memory:** ~100 MB total

### Production (Medium Cluster)

**Balanced configuration for typical workloads:**

```bash
spectre server \
  --data-dir=/mnt/spectre-data \
  --segment-size=10485760 \     # 10 MB (default)
  --cache-enabled=true \
  --cache-max-mb=100 \          # 100 MB cache
  --max-concurrent-requests=100
```

**Storage:** ~13 GB/month (30-day retention)
**Memory:** ~256 MB total

### Production (High Volume)

**Optimized for large clusters:**

```bash
spectre server \
  --data-dir=/mnt/spectre-data \
  --segment-size=104857600 \    # 100 MB (better compression)
  --cache-enabled=true \
  --cache-max-mb=500 \          # 500 MB cache (faster queries)
  --max-concurrent-requests=200
```

**Storage:** ~130 GB/month (30-day retention)
**Memory:** ~1 GB total

### Resource-Constrained (Edge)

**Minimal resources for edge deployments:**

```bash
spectre server \
  --data-dir=/data \
  --segment-size=10485760 \    # 10 MB
  --cache-enabled=false \      # Disable cache (save memory)
  --max-concurrent-requests=10
```

**Storage:** ~1-5 GB/month
**Memory:** ~50-100 MB total

## Troubleshooting

### Disk Full

**Symptoms:**
- Write errors in logs
- `no space left on device`
- Application crashes

**Solutions:**

**1. Check current usage:**
```bash
df -h /data
du -sh /data
```

**2. Delete old files manually:**
```bash
# Delete files older than 7 days
find /data -name "*.bin" -mtime +7 -delete
```

**3. Increase PVC size (Kubernetes):**
```bash
kubectl edit pvc spectre-storage
# Increase storage request
```

**4. Reduce retention (future):**
```
# Automatic retention not yet implemented
# Track issue: https://github.com/moolen/spectre/issues/xxx
```

### Slow Queries

**Symptoms:**
- Query latency > 1 second
- Timeout errors
- High CPU usage

**Solutions:**

**1. Increase cache size:**
```bash
--cache-max-mb=200  # or higher
```

**2. Enable cache if disabled:**
```bash
--cache-enabled=true
```

**3. Add query filters:**
```
# Bad: Query all events
/api/v1/query?time=[start,end]

# Good: Filter by kind and namespace
/api/v1/query?time=[start,end]&kind=Pod&namespace=default
```

**4. Limit time range:**
```
# Bad: Query last 7 days
time=[now-7d,now]

# Good: Query last hour
time=[now-1h,now]
```

### High Memory Usage

**Symptoms:**
- OOMKilled in Kubernetes
- Memory > limits
- Swapping

**Solutions:**

**1. Reduce cache size:**
```bash
--cache-max-mb=50  # or disable entirely
```

**2. Reduce concurrent requests:**
```bash
--max-concurrent-requests=50
```

**3. Increase memory limits (Kubernetes):**
```yaml
resources:
  limits:
    memory: "512Mi"  # Increase as needed
```

### Import Failures

**Symptoms:**
- Import command fails
- Partial data loaded
- Errors in logs

**Common Causes:**

**1. File format incorrect:**
```
Error: invalid JSON format
Solution: Ensure files are JSON array of events
```

**2. File permissions:**
```bash
# Fix permissions
chmod 644 /backups/*.json
```

**3. Disk space:**
```bash
# Check space before import
df -h /data
```

## Best Practices

### ✅ Do

- **Use SSD for data directory** - 3-5× faster queries than HDD
- **Monitor disk usage** - Set alerts at 80% capacity
- **Enable caching for dashboards** - Improves repeated query performance
- **Plan for retention** - Calculate disk needs before deployment
- **Use PersistentVolumes in Kubernetes** - Data survives pod restarts
- **Backup regularly** - Copy `.bin` files for disaster recovery

### ❌ Don't

- **Don't disable compression** - Would use 4× more disk space (not possible anyway)
- **Don't use very small blocks** (\<1 MB) - Poor compression, large indexes
- **Don't use NFS for high-volume** - Network latency hurts write performance
- **Don't run without monitoring** - Could fill disk unexpectedly
- **Don't share data directory** - Multiple Spectre instances will corrupt data
- **Don't manually edit .bin files** - Will corrupt file format

## Performance Tuning

### Read-Heavy Workload (Dashboards)

```bash
# Increase cache to keep hot data in memory
--cache-max-mb=500

# Default block size (good query granularity)
--segment-size=10485760
```

**Expected:** 3-5× faster repeated queries

### Write-Heavy Workload (Large Clusters)

```bash
# Larger blocks (better compression, fewer files)
--segment-size=104857600

# Moderate cache (writes don't benefit from cache)
--cache-max-mb=100
```

**Expected:** Better compression ratio (78% vs 75%)

### Balanced Workload (Most Clusters)

```bash
# Use defaults
--segment-size=10485760
--cache-max-mb=100
```

**Expected:** Good all-around performance

## Related Documentation

- [Storage Design](../architecture/storage-design.md) - Architecture and design decisions
- [Block Format Reference](../architecture/block-format.md) - Binary file format specification
- [Compression](../architecture/compression.md) - Compression algorithms and performance
- [Query Execution](../architecture/query-execution.md) - Query pipeline and optimization

<!-- Source: cmd/spectre/commands/server.go, internal/config/config.go, internal/storage/README.md -->
