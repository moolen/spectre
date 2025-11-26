# Data Model: UI-API Integration

**Phase**: 1 - Design & Contracts
**Date**: 2025-11-26

## Entity Definitions

### Backend Entities (Go)

These entities define the API response structure from the Go backend.

#### SearchResponse

**Purpose**: Root response object from `/v1/search` endpoint

**Fields**:
- `resources: []Resource` - Array of matched Kubernetes resources with audit history
- `count: int` - Total number of resources returned
- `executionTimeMs: int64` - Server query execution time in milliseconds

**Validation Rules**:
- resources array must not be nil (empty array if no matches)
- count must be non-negative and equal to len(resources)
- executionTimeMs must be >= 0

**Usage**: Returned as JSON from `/v1/search` GET endpoint

```go
type SearchResponse struct {
  Resources      []Resource `json:"resources"`
  Count          int        `json:"count"`
  ExecutionTimeMs int64     `json:"executionTimeMs"`
}
```

---

#### Resource

**Purpose**: Represents a single Kubernetes resource with its complete audit history

**Fields**:
- `id: string` - Unique identifier (Kubernetes UID)
- `group: string` - API group (e.g., "apps", "batch", "core" for v1)
- `version: string` - API version (e.g., "v1", "v2beta1")
- `kind: string` - Resource kind (e.g., "Pod", "Deployment", "Service")
- `namespace: string` - Kubernetes namespace
- `name: string` - Resource name within namespace
- `statusSegments: []StatusSegment` - Timeline of status changes
- `events: []AuditEvent` - Audit/activity events for this resource

**Validation Rules**:
- id must be non-empty
- kind, namespace, name must be non-empty
- statusSegments and events arrays must not be nil (empty if none)
- statusSegments must be sorted by startTime ascending
- events must be sorted by timestamp ascending

**Relationships**:
- Has many StatusSegments (1:N)
- Has many AuditEvents (1:N)
- StatusSegments and AuditEvents are separate but relate to same resource

**State Transitions**:
- A resource can be Created, Updated, Deleted over time
- Status can transition between Ready, Warning, Error, Terminating

```go
type Resource struct {
  ID              string           `json:"id"`
  Group           string           `json:"group"`
  Version         string           `json:"version"`
  Kind            string           `json:"kind"`
  Namespace       string           `json:"namespace"`
  Name            string           `json:"name"`
  StatusSegments  []StatusSegment  `json:"statusSegments"`
  Events          []AuditEvent     `json:"events"`
}
```

---

#### StatusSegment

**Purpose**: Represents a continuous period during which a resource maintained a specific status

**Fields**:
- `startTime: int64` - Segment start time (Unix seconds)
- `endTime: int64` - Segment end time (Unix seconds)
- `status: string` - Status during this period (Ready | Warning | Error | Terminating | Unknown)
- `message: string` - Optional human-readable description of status
- `config: map[string]interface{}` - Configuration snapshot at this point in time

**Validation Rules**:
- startTime must be < endTime
- startTime and endTime must be non-negative
- status must be one of the enumerated values
- status must not be empty
- config must not be nil (empty map if no config)

**Relationships**:
- Belongs to Resource (N:1)
- Events can occur during this segment

**State Values**:
- `Ready`: Resource is operational and healthy
- `Warning`: Resource is experiencing issues but still operational
- `Error`: Resource has encountered errors
- `Terminating`: Resource is being deleted
- `Unknown`: Status could not be determined

```go
type StatusSegment struct {
  StartTime  int64                  `json:"startTime"`
  EndTime    int64                  `json:"endTime"`
  Status     string                 `json:"status"`
  Message    string                 `json:"message,omitempty"`
  Config     map[string]interface{} `json:"config"`
}
```

---

#### AuditEvent

**Purpose**: Represents a Kubernetes audit log entry or API activity for a resource

**Fields**:
- `id: string` - Unique event identifier
- `timestamp: int64` - Event occurrence time (Unix seconds)
- `verb: string` - Action performed (create | update | patch | delete | get | list)
- `user: string` - User or service account that performed the action
- `message: string` - Description of what happened
- `details: string` - Optional additional context or error details

**Validation Rules**:
- id must be non-empty
- timestamp must be non-negative
- verb must be one of the enumerated values
- user must be non-empty string
- message must be non-empty

**Relationships**:
- Belongs to Resource (N:1)
- Occurs during a StatusSegment (but not strictly bound)

**Verb Types**:
- `create`: Resource was created
- `update`: Resource was fully updated
- `patch`: Resource was partially updated
- `delete`: Resource was deleted
- `get`: Resource was read/accessed
- `list`: Resource was listed as part of query

```go
type AuditEvent struct {
  ID        string `json:"id"`
  Timestamp int64  `json:"timestamp"`
  Verb      string `json:"verb"`
  User      string `json:"user"`
  Message   string `json:"message"`
  Details   string `json:"details,omitempty"`
}
```

---

### Frontend Entities (TypeScript)

These entities are the UI's internal representations, converted from backend entities.

#### K8sResource (from ui/src/types.ts)

**Purpose**: Frontend representation of a Kubernetes resource with audit history

**Fields**:
- `id: string` - Unique identifier
- `group: string` - API group
- `version: string` - API version
- `kind: string` - Resource kind
- `namespace: string` - Kubernetes namespace
- `name: string` - Resource name
- `statusSegments: ResourceStatusSegment[]` - Timeline of status changes
- `events: K8sEvent[]` - Audit events

**Conversion from Backend**:
- Backend Resource → K8sResource (1:1 mapping)
- Backend timestamps (Unix seconds) → JavaScript Date objects
- All string fields preserved as-is

**Usage**: Primary data structure throughout React components

```typescript
export interface K8sResource {
  id: string;
  group: string;
  version: string;
  kind: string;
  namespace: string;
  name: string;
  statusSegments: ResourceStatusSegment[];
  events: K8sEvent[];
}
```

---

#### ResourceStatusSegment (from ui/src/types.ts)

**Purpose**: Frontend representation of a status period

**Fields**:
- `start: Date` - Segment start time (JavaScript Date)
- `end: Date` - Segment end time (JavaScript Date)
- `status: ResourceStatus` - Status enum value
- `message?: string` - Optional description
- `config: Record<string, any>` - Configuration snapshot

**Conversion from Backend**:
- Backend `startTime` (Unix seconds) → JavaScript Date object
- Backend `endTime` (Unix seconds) → JavaScript Date object
- Backend `status` string → ResourceStatus enum value

```typescript
export interface ResourceStatusSegment {
  start: Date;
  end: Date;
  status: ResourceStatus;
  message?: string;
  config: Record<string, any>;
}
```

---

#### K8sEvent (from ui/src/types.ts)

**Purpose**: Frontend representation of an audit event

**Fields**:
- `id: string` - Event identifier
- `timestamp: Date` - Event time (JavaScript Date)
- `verb: string` - Action type
- `message: string` - Description
- `user: string` - User who performed action
- `details?: string` - Optional extra info

**Conversion from Backend**:
- Backend `timestamp` (Unix seconds) → JavaScript Date object
- All other fields preserved as-is

```typescript
export interface K8sEvent {
  id: string;
  timestamp: Date;
  verb: 'create' | 'update' | 'patch' | 'delete' | 'get' | 'list';
  message: string;
  user: string;
  details?: string;
}
```

---

#### ResourceStatus (from ui/src/types.ts)

**Purpose**: Enumeration of possible resource status values

**Values**:
- `Unknown = 'Unknown'`
- `Ready = 'Ready'`
- `Warning = 'Warning'`
- `Error = 'Error'`
- `Terminating = 'Terminating'`

```typescript
export enum ResourceStatus {
  Unknown = 'Unknown',
  Ready = 'Ready',
  Warning = 'Warning',
  Error = 'Error',
  Terminating = 'Terminating'
}
```

---

### Query/Filter Entities

#### QueryFilters (from internal/models)

**Purpose**: Filters applied to search queries

**Fields**:
- `group?: string` - Optional API group filter
- `version?: string` - Optional API version filter
- `kind?: string` - Optional resource kind filter
- `namespace?: string` - Optional namespace filter

**Validation Rules**:
- All fields are optional
- If provided, each must be non-empty string
- Empty filters (all fields missing) matches all resources

**Usage**: Query parameters to `/v1/search` endpoint

```go
type QueryFilters struct {
  Group     string
  Version   string
  Kind      string
  Namespace string
}
```

---

## Data Flow

### Request Flow

```
User Action (filter change)
  ↓
React Component calls useTimeline hook
  ↓
useTimeline.fetchData()
  ↓
apiClient.searchResources(startTime, endTime, filters)
  ↓
HTTP GET /v1/search?start=X&end=Y&namespace=N&kind=K
  ↓
Backend processes query
  ↓
SearchResponse JSON returned
```

### Response Flow

```
Backend SearchResponse (Go)
  ↓
HTTP response with SearchResponse JSON
  ↓
apiClient receives response
  ↓
transformSearchResponse() converts to K8sResource[]
  ↓
useTimeline stores in state
  ↓
React components read resources from state
  ↓
Timeline visualization renders
```

## Timestamp Handling

**Backend Storage**: Unix seconds (int64)
- Example: `1700000000` (November 15, 2023 04:26:40 UTC)

**API Transport**: JSON number (same value)
- Example: `"timestamp": 1700000000`

**Frontend Storage**: JavaScript Date object
- Conversion: `new Date(1700000000 * 1000)` (multiply by 1000 for milliseconds)

**Display**: Depends on component (various formatting options available)

## Validation Strategy

### Backend Validation
- Query parameters must have valid start/end timestamps
- Filters must be valid if provided
- Response data must be complete (no null/missing required fields)

### Frontend Validation
- Incoming response is validated before transformation
- Transformed objects must have expected types
- Invalid data is handled gracefully (skip or show error)

## Performance Considerations

### Data Size
- Response size depends on number of resources and events
- Typical query: 100-1000 resources, 1000-10000 events
- Estimated: 1-5 MB response size for typical queries

### Sorting
- Resources in response should be sorted (implementation detail)
- StatusSegments must be sorted by startTime ascending
- Events must be sorted by timestamp ascending

### Indexing
- Backend should have index on (namespace, kind) for filtering
- Time range indexes for efficient timestamp-based queries
