# Kubernetes Event Monitor API Contract

**Version**: 1.0
**Base URL**: http://localhost:8080 (typically accessed via `kubectl port-forward`)
**Authentication**: None (tests use default Kubernetes auth)

## Overview

The KEM API exposes audit events and resource status information captured from Kubernetes clusters. The test suite validates the following 5 endpoints:

1. **GET /v1/search** - Search and list resources with audit events
2. **GET /v1/metadata** - Get filter metadata (namespaces, kinds, counts)
3. **GET /v1/resources/{id}** - Get single resource with full details
4. **GET /v1/resources/{id}/segments** - Get status timeline segments
5. **GET /v1/resources/{id}/events** - Get audit events for resource

---

## 1. Search Endpoint

### `GET /v1/search`

Returns a list of resources matching time range and optional filters.

**Query Parameters**:
- `start` (required): Unix timestamp in seconds (start of time window)
- `end` (required): Unix timestamp in seconds (end of time window)
- `namespace` (optional): Filter by namespace
- `kind` (optional): Filter by resource kind (e.g., Deployment, Pod)

**Response**: HTTP 200

```json
{
  "resources": [
    {
      "id": "7a8c5e21-4d9c-11eb-9a87-42010a8a0002",
      "name": "nginx-deployment",
      "kind": "Deployment",
      "apiVersion": "apps/v1",
      "namespace": "test-default",
      "statusSegments": [
        {
          "status": "Ready",
          "startTime": 1700989200,
          "endTime": 1700989260,
          "message": "Deployment ready",
          "config": {}
        }
      ],
      "events": [
        {
          "id": "event-001",
          "timestamp": 1700989200,
          "verb": "create",
          "user": "test-user",
          "message": "Created deployment nginx-deployment"
        }
      ]
    }
  ],
  "count": 1,
  "executionTimeMs": 45
}
```

**Test Validation**:
- ✓ Returns resources created within [start, end] window
- ✓ Namespace filter returns only matching resources
- ✓ Unfiltered query returns resources from all namespaces
- ✓ Non-existent namespace returns empty array
- ✓ Execution time < 5000ms

---

## 2. Metadata Endpoint

### `GET /v1/metadata`

Returns aggregated metadata for filtering and discovery.

**Query Parameters**:
- `start` (optional): Unix timestamp in seconds (for time-scoped metadata)
- `end` (optional): Unix timestamp in seconds

**Response**: HTTP 200

```json
{
  "namespaces": [
    "default",
    "kube-system",
    "test-default",
    "test-alternate"
  ],
  "kinds": [
    "ConfigMap",
    "Deployment",
    "Pod",
    "Secret",
    "Service"
  ],
  "groups": [
    "",
    "apps",
    "batch",
    "extensions"
  ],
  "resourceCounts": {
    "Deployment": 2,
    "Pod": 8,
    "ConfigMap": 1
  },
  "totalEvents": 127,
  "timeRange": {
    "earliest": 1700989000,
    "latest": 1700992600
  }
}
```

**Test Validation**:
- ✓ Returns all namespaces with resources
- ✓ Returns all resource kinds found
- ✓ Resource counts are accurate
- ✓ Time range matches earliest/latest events

---

## 3. Resource Detail Endpoint

### `GET /v1/resources/{id}`

Returns complete details for a single resource.

**Path Parameters**:
- `id`: Resource UID (from /v1/search response)

**Response**: HTTP 200

```json
{
  "id": "7a8c5e21-4d9c-11eb-9a87-42010a8a0002",
  "name": "nginx-deployment",
  "kind": "Deployment",
  "apiVersion": "apps/v1",
  "namespace": "test-default",
  "statusSegments": [
    {
      "status": "Ready",
      "startTime": 1700989200,
      "endTime": 1700989260,
      "message": "Deployment ready",
      "config": {}
    },
    {
      "status": "Warning",
      "startTime": 1700989260,
      "endTime": 1700989320,
      "message": "Pod pending",
      "config": {}
    }
  ],
  "events": [
    {
      "id": "event-001",
      "timestamp": 1700989200,
      "verb": "create",
      "user": "test-user",
      "message": "Created deployment nginx-deployment"
    },
    {
      "id": "event-002",
      "timestamp": 1700989260,
      "verb": "update",
      "user": "system:controller",
      "message": "Updated deployment status"
    }
  ]
}
```

**Test Validation**:
- ✓ Returns resource with all segments and events
- ✓ Segments in chronological order
- ✓ Events in chronological order
- ✓ Metadata matches search results

---

## 4. Segments Endpoint

### `GET /v1/resources/{id}/segments`

Returns status timeline segments for a resource.

**Path Parameters**:
- `id`: Resource UID

**Query Parameters**:
- `start` (optional): Filter segments starting after this time (Unix seconds)
- `end` (optional): Filter segments ending before this time (Unix seconds)

**Response**: HTTP 200

```json
{
  "segments": [
    {
      "status": "Ready",
      "startTime": 1700989200,
      "endTime": 1700989260,
      "message": "Deployment ready",
      "config": {
        "replicas": 3,
        "ready": 3
      }
    },
    {
      "status": "Ready",
      "startTime": 1700989260,
      "endTime": 1700992600,
      "message": "Deployment running normally",
      "config": {
        "replicas": 3,
        "ready": 3
      }
    }
  ],
  "resourceId": "7a8c5e21-4d9c-11eb-9a87-42010a8a0002",
  "count": 2
}
```

**Test Validation**:
- ✓ Returns segments in chronological order
- ✓ Time range filtering works correctly
- ✓ Segments don't overlap
- ✓ Count matches returned segments

---

## 5. Events Endpoint

### `GET /v1/resources/{id}/events`

Returns audit events for a resource.

**Path Parameters**:
- `id`: Resource UID

**Query Parameters**:
- `start` (optional): Filter events after this time (Unix seconds)
- `end` (optional): Filter events before this time (Unix seconds)
- `limit` (optional): Maximum events to return (default: 100)

**Response**: HTTP 200

```json
{
  "events": [
    {
      "id": "event-001",
      "timestamp": 1700989200,
      "verb": "create",
      "user": "test-user",
      "message": "Created deployment nginx-deployment",
      "details": "Image: nginx:latest, Replicas: 3"
    },
    {
      "id": "event-002",
      "timestamp": 1700989260,
      "verb": "update",
      "user": "system:controller",
      "message": "Updated deployment status",
      "details": "Status changed to Ready"
    },
    {
      "id": "event-003",
      "timestamp": 1700990100,
      "verb": "patch",
      "user": "test-user",
      "message": "Patched deployment spec",
      "details": "Scaled replicas from 3 to 5"
    }
  ],
  "count": 3,
  "resourceId": "7a8c5e21-4d9c-11eb-9a87-42010a8a0002"
}
```

**Test Validation**:
- ✓ Returns events in chronological order
- ✓ Time range filtering works correctly
- ✓ Limit parameter is respected
- ✓ Count matches returned events

---

## Common Response Fields

### SearchResponse
```typescript
{
  resources: Resource[],
  count: number,          // Number of resources returned
  executionTimeMs: number // Query execution time in milliseconds
}
```

### Resource
```typescript
{
  id: string,                 // Resource UID
  name: string,               // Resource name
  kind: string,               // Resource kind (Deployment, Pod, etc.)
  apiVersion: string,         // API version (v1, apps/v1, etc.)
  namespace: string,          // Namespace
  statusSegments: Segment[],  // Timeline of status changes
  events: AuditEvent[]        // Audit events
}
```

### StatusSegment
```typescript
{
  status: "Ready" | "Warning" | "Error" | "Terminating" | "Unknown",
  startTime: number,   // Unix seconds
  endTime: number,     // Unix seconds
  message: string,     // Human-readable status description
  config: object       // Configuration snapshot
}
```

### AuditEvent
```typescript
{
  id: string,        // Event UUID
  timestamp: number, // Unix seconds
  verb: "create" | "update" | "patch" | "delete" | "get" | "list",
  user: string,      // User who performed action
  message: string,   // Human-readable description
  details?: string   // Additional details (optional)
}
```

### MetadataResponse
```typescript
{
  namespaces: string[],
  kinds: string[],
  groups: string[],
  resourceCounts: object,
  totalEvents: number,
  timeRange: {
    earliest: number,  // Unix seconds
    latest: number     // Unix seconds
  }
}
```

---

## Error Responses

### 400 Bad Request

```json
{
  "error": "INVALID_PARAMETER",
  "message": "Invalid time range: start time must be before end time"
}
```

### 404 Not Found

```json
{
  "error": "NOT_FOUND",
  "message": "Resource not found: 7a8c5e21-4d9c-11eb-9a87-42010a8a0002"
}
```

### 500 Internal Server Error

```json
{
  "error": "INTERNAL_ERROR",
  "message": "Failed to query storage backend"
}
```

---

## Test Scenarios by Endpoint

| Endpoint | Scenario | Key Test |
|----------|----------|----------|
| /v1/search | Default resources | Capture + filter events |
| /v1/metadata | Filter discovery | Verify namespace/kind lists |
| /v1/resources/{id} | Pod restart | Verify events persist |
| /v1/resources/{id}/segments | Timeline continuity | Verify segments after restart |
| /v1/resources/{id}/events | Event availability | Verify events after restart |
| /v1/search + filter | Cross-namespace | Verify isolation |
| /v1/search | Dynamic config | Verify new resources appear |

---

## Performance Expectations

- **Response time**: < 5 seconds for typical queries
- **95th percentile**: < 10 seconds with retry logic
- **Event propagation**: 2-5 seconds (async processing)
- **Configuration reload**: Up to 30 seconds (with annotation trigger)

These expectations are used by test assertions with `assert.Eventually()` timeouts.
