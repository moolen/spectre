# API Documentation: Kubernetes Event Monitor

**Endpoint**: `/v1/search`
**Method**: `GET`
**Content-Type**: `application/json`

---

## Overview

The `/v1/search` endpoint allows querying stored Kubernetes events with flexible filtering by time window and resource attributes.

---

## Request

### URL Format

```
GET /v1/search?start=<timestamp>&end=<timestamp>[&filters]
```

### Query Parameters

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| `start` | int64 | Yes | Unix timestamp (seconds) - start of time window | `1700000000` |
| `end` | int64 | Yes | Unix timestamp (seconds) - end of time window | `1700086400` |
| `kind` | string | No | Resource kind to filter | `Pod`, `Deployment`, `Service` |
| `namespace` | string | No | Kubernetes namespace to filter | `default`, `kube-system` |
| `group` | string | No | API group to filter | `apps`, `batch`, `storage.k8s.io` |
| `version` | string | No | API version to filter | `v1`, `v1beta1` |

### Parameter Validation

- **Timestamps**: Must be Unix time in seconds (valid range: 0 to 9999999999)
- **Time Window**: `start < end` (required)
- **Strings**: Alphanumeric, `-`, `.`, `/` allowed
- **String Length**: Max 256 characters

### Filter Semantics

- **Multiple filters**: AND logic (all conditions must match)
- **Unspecified filters**: Wildcard (matches all values)
- **Case-sensitive**: All values are case-sensitive

---

## Examples

### 1. Query All Events in Time Window

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400"
```

**Use Case**: Retrieve all events for past 24 hours
**Result**: All events regardless of kind/namespace

### 2. Query Pods in Default Namespace

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod&namespace=default"
```

**Use Case**: Monitor all Pod changes in default namespace
**Result**: Only Pod creation/update/delete events in "default"

### 3. Query Deployments (Any Namespace)

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment"
```

**Use Case**: Find all Deployment changes across cluster
**Result**: All Deployment events, any namespace

### 4. Query by API Group

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&group=apps&kind=StatefulSet"
```

**Use Case**: Query resources in specific API group
**Result**: StatefulSet events from "apps" group

### 5. Complex Filter (AND Logic)

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400&group=apps&version=v1&kind=Deployment&namespace=production"
```

**Use Case**: Find v1 Deployments in production namespace
**Result**: Only events matching ALL criteria

### 6. Pretty Print with jq

```bash
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod" | jq .
```

**Output**: Formatted JSON for human readability

### 7. Get Only Event Count

```bash
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod" | jq '.count'
```

**Output**: Just the number: `42`

### 8. Check Query Performance

```bash
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Deployment" | jq '{time: .executionTimeMs, scanned: .segmentsScanned, skipped: .segmentsSkipped}'
```

**Output**: Performance metrics

---

## Response

### Success Response (200 OK)

```json
{
  "events": [
    {
      "id": "evt-12345",
      "timestamp": 1700000123,
      "type": "CREATE",
      "resource": {
        "kind": "Pod",
        "namespace": "default",
        "name": "test-pod-abc123",
        "group": "",
        "version": "v1",
        "uid": "12345678-1234-1234-1234-123456789012"
      },
      "data": {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {...},
        "spec": {...},
        "status": {...}
      }
    },
    ...
  ],
  "count": 42,
  "executionTimeMs": 45,
  "filesSearched": 24,
  "segmentsScanned": 12,
  "segmentsSkipped": 88
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `events` | array | Array of matching Event objects |
| `count` | int | Total number of events returned |
| `executionTimeMs` | int | Query execution time in milliseconds |
| `filesSearched` | int | Number of hourly files examined |
| `segmentsScanned` | int | Number of segments decompressed and filtered |
| `segmentsSkipped` | int | Number of segments skipped (optimization success) |

### Event Object Structure

```json
{
  "id": "string",                    // Unique event ID
  "timestamp": 1234567890,           // Unix timestamp (seconds)
  "type": "CREATE|UPDATE|DELETE",    // Event type
  "resource": {
    "kind": "Pod",                   // Resource kind
    "namespace": "default",          // Kubernetes namespace
    "name": "pod-name",              // Resource name
    "group": "apps",                 // API group
    "version": "v1",                 // API version
    "uid": "uuid-string"             // Resource UID
  },
  "data": { ... }                    // Full resource object (JSON)
}
```

### Error Responses

#### 400 Bad Request

```json
{
  "error": "invalid start timestamp",
  "details": "start must be less than end"
}
```

**Common causes**:
- Missing required parameters
- Invalid timestamp format
- start >= end
- Invalid filter values

#### 404 Not Found

```json
{
  "error": "no events found",
  "details": "no storage files available for requested time window"
}
```

**Causes**:
- Time window before any events captured
- All matching events filtered out

#### 500 Internal Server Error

```json
{
  "error": "query execution failed",
  "details": "error reading storage file: I/O error"
}
```

**Causes**:
- Disk I/O failures
- Storage file corruption
- Out of memory

---

## Performance Notes

### Query Optimization

The system automatically optimizes queries:

1. **Index-based block selection**: Uses inverted indexes to skip non-matching blocks
2. **Lazy decompression**: Only decompresses candidate blocks
3. **Early termination**: Returns results as soon as available
4. **Parallel reading**: Processes multiple hourly files concurrently

### Performance Metrics

- **Single file query**: 10-50ms
- **24-hour window**: 100-200ms
- **7-day window**: <2 seconds
- **Skip rate**: 50-80% of blocks (depends on selectivity)

### Best Practices

1. **Narrow time windows**: Smaller windows = faster queries
   ```bash
   # Good: 1 hour
   curl "...?start=1700000000&end=1700003600"

   # Slower: 30 days
   curl "...?start=1698408000&end=1700001600"
   ```

2. **Use specific filters**: More filters = fewer blocks to scan
   ```bash
   # Good: Specific resource
   curl "...?kind=Deployment&namespace=default"

   # Slower: No filters
   curl "...?start=X&end=Y"
   ```

3. **Check segmentsSkipped**: High value = good optimization
   ```bash
   # If segmentsSkipped < 50%, try adding more filters
   curl "...?kind=Pod&namespace=default" | jq '.segmentsSkipped'
   ```

---

## Common Query Patterns

### Monitor Specific Deployment Changes

```bash
# Get all changes to "web-app" Deployment in production
curl -X GET "http://localhost:8080/v1/search" \
  -G \
  -d "start=1700000000" \
  -d "end=1700086400" \
  -d "kind=Deployment" \
  -d "namespace=production" | jq '.events[] | select(.resource.name == "web-app")'
```

### Find All Delete Events

```bash
# Get all resource deletions in past hour
NOW=$(date +%s)
HOUR_AGO=$((NOW - 3600))

curl -X GET "http://localhost:8080/v1/search" \
  -G \
  -d "start=$HOUR_AGO" \
  -d "end=$NOW" | jq '.events[] | select(.type == "DELETE")'
```

### Track Pod Creation Rate

```bash
# How many Pods were created in past 24 hours?
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod" | \
  jq '.events | map(select(.type == "CREATE")) | length'
```

### Find Recent Changes in All Namespaces

```bash
# All changes in past 5 minutes
NOW=$(date +%s)
FIVE_MIN_AGO=$((NOW - 300))

curl -X GET "http://localhost:8080/v1/search" \
  -G \
  -d "start=$FIVE_MIN_AGO" \
  -d "end=$NOW"
```

### Export Events to CSV

```bash
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400" | \
  jq -r '.events[] | [.timestamp, .type, .resource.kind, .resource.namespace, .resource.name] | @csv' > events.csv
```

---

## Timestamps Reference

### Current Timestamp

```bash
# Get current Unix timestamp
date +%s

# Result: 1700001234
```

### Calculate Time Windows

```bash
# Past 24 hours
NOW=$(date +%s)
DAY_AGO=$((NOW - 86400))
echo "?start=$DAY_AGO&end=$NOW"

# Past 7 days
WEEK_AGO=$((NOW - 604800))
echo "?start=$WEEK_AGO&end=$NOW"

# Past hour
HOUR_AGO=$((NOW - 3600))
echo "?start=$HOUR_AGO&end=$NOW"

# Specific date (2025-11-25 00:00 UTC)
SPECIFIC=$(date -d "2025-11-25 00:00:00 UTC" +%s)
echo "?start=$SPECIFIC"
```

### Online Timestamp Converter

- https://www.unixtimestamp.com/
- Useful for converting human-readable dates to Unix timestamps

---

## Rate Limiting & Quotas

Currently **no rate limiting** is enforced. Future versions may implement:

- Per-client quotas
- Request rate limits
- Maximum result set sizes
- Timeout on long-running queries

---

## Client Libraries

### cURL (Command-line)

```bash
curl -X GET "http://localhost:8080/v1/search?start=1700000000&end=1700086400"
```

### Go

```go
package main

import (
  "fmt"
  "net/http"
)

func main() {
  resp, _ := http.Get("http://localhost:8080/v1/search?start=1700000000&end=1700086400")
  // ... handle response
}
```

### Python

```python
import requests

url = "http://localhost:8080/v1/search"
params = {
    "start": 1700000000,
    "end": 1700086400,
    "kind": "Pod"
}
response = requests.get(url, params=params)
print(response.json())
```

### JavaScript/Node.js

```javascript
const fetch = require('node-fetch');

fetch('http://localhost:8080/v1/search?start=1700000000&end=1700086400')
  .then(r => r.json())
  .then(data => console.log(data));
```

---

## Troubleshooting

### Empty Results

```bash
# Check if events exist in time range
curl "http://localhost:8080/v1/search?start=1&end=9999999999"

# If still empty, no events have been captured yet
# Trigger a resource change in Kubernetes to generate events
```

### Slow Queries

```bash
# Check segmentsSkipped ratio
curl "...query..." | jq '.segmentsSkipped / .segmentsScanned'

# If < 0.5 (50%), add more specific filters
# Or reduce time window
```

### Connection Refused

```bash
# Verify server is running
lsof -i :8080

# Or check logs
kubectl logs -n monitoring deployment/k8s-event-monitor
```

### No Matching Events

```bash
# Verify filter values are correct (case-sensitive)
curl "http://localhost:8080/v1/search?start=X&end=Y&kind=pod"  # Wrong
curl "http://localhost:8080/v1/search?start=X&end=Y&kind=Pod"  # Correct
```

---

## See Also

- [Quickstart Guide](../specs/001-k8s-event-monitor/quickstart.md)
- [Architecture Overview](./ARCHITECTURE.md)
- [Operations Guide](./OPERATIONS.md)
