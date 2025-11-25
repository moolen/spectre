# Data Model: Audit Timeline UI

**Feature**: Audit Timeline UI (`004-audit-timeline-ui`)
**Phase**: 1 (Design)
**Date**: 2025-11-25

## Core Entities

### Resource

Represents a Kubernetes resource being audited within the system.

**Type Definition** (TypeScript)
```typescript
interface Resource {
  id: string;                    // Unique identifier (e.g., "default-nginx-deployment-1")
  name: string;                  // Resource name (e.g., "nginx-deployment")
  kind: string;                  // Resource kind (e.g., "Deployment")
  apiVersion: string;            // API version (e.g., "apps/v1")
  namespace: string;             // Kubernetes namespace
  createdAt: Date;               // Resource creation timestamp
  deletedAt?: Date;              // Resource deletion timestamp (null if still exists)
  labels?: Record<string, string>; // Kubernetes labels for filtering/organization
}
```

**Attributes**:
- `id`: Unique identifier combining namespace, kind, and name
- `name`: User-visible resource name
- `kind`: Resource type classification
- `namespace`: Kubernetes namespace for filtering
- `createdAt`: Immutable creation timestamp
- `deletedAt`: When resource was deleted (null if active)
- `labels`: Optional metadata for advanced filtering

**Relationships**:
- Has many `Segment` instances across its lifespan
- Generates multiple `AuditEvent` entries
- Belongs to one `Namespace`
- Categorized by one `Kind`

**Validation**:
- `id` must be non-empty and unique
- `name` must be non-empty, max 253 characters (Kubernetes constraint)
- `namespace` must match existing namespaces from metadata API
- `kind` must match existing kinds from metadata API
- `createdAt` must be before `deletedAt` (if present)

---

### Segment

Represents a continuous time period during which a Resource maintained a consistent status/state.

**Type Definition**
```typescript
interface Segment {
  id: string;                    // Unique identifier (e.g., "seg-uuid-1")
  resourceId: string;            // FK to Resource
  status: 'Ready' | 'Warning' | 'Error' | 'Terminating' | 'Unknown';
  startTime: Date;               // Segment start timestamp
  endTime: Date;                 // Segment end timestamp
  message?: string;              // Status message (e.g., "Running normally")
  configuration: Record<string, any>; // JSON configuration snapshot at startTime
}
```

**Attributes**:
- `id`: Unique identifier for this segment
- `resourceId`: Foreign key to parent Resource
- `status`: Enum of 5 possible states with specific color mappings
- `startTime`: Precise moment status changed
- `endTime`: Precise moment status changed again
- `message`: Human-readable status explanation (optional)
- `configuration`: Serialized JSON object representing resource spec at segment start

**Status Color Mapping**:
- `Ready`: Green (#10b981 or similar success color)
- `Warning`: Yellow (#f59e0b or similar warning color)
- `Error`: Red (#ef4444 or similar error color)
- `Terminating`: Gray (#9ca3af or similar neutral color)
- `Unknown`: Gray (#9ca3af or similar neutral color)

**Relationships**:
- Belongs to exactly one `Resource`
- Contains multiple `AuditEvent` entries (events with timestamps within [startTime, endTime])
- References previous segment via resourceId + time ordering

**Validation**:
- `resourceId` must reference existing Resource
- `status` must be one of 5 allowed values
- `startTime` must be before `endTime`
- `startTime` must be ≥ parent Resource's createdAt
- `endTime` must be ≤ parent Resource's deletedAt (if present)
- `configuration` must be valid JSON serializable
- For same resource: segments must be non-overlapping and chronologically ordered

---

### AuditEvent

Represents a discrete action or change applied to a Resource.

**Type Definition**
```typescript
interface AuditEvent {
  id: string;                    // Unique identifier
  resourceId: string;            // FK to Resource
  timestamp: Date;               // When the event occurred
  eventType: 'create' | 'update' | 'patch' | 'delete' | 'status' | string;
  user?: string;                 // User who triggered the event
  changes?: Record<string, any>; // What changed in this event (optional detail)
  message?: string;              // Event description
}
```

**Attributes**:
- `id`: Unique event identifier
- `resourceId`: Foreign key to Resource
- `timestamp`: Exact time of the event
- `eventType`: Categorizes the type of change (enumeration)
- `user`: Optional username or service account that triggered event
- `changes`: Optional structured representation of what changed
- `message`: Human-readable event summary

**Event Type Mapping**:
- `create`: Resource was created
- `update`: Resource was replaced/updated entirely
- `patch`: Partial update to resource
- `delete`: Resource was deleted
- `status`: Status/condition changed without spec change
- Custom types allowed for domain-specific events

**Relationships**:
- Belongs to exactly one `Resource`
- Occurs within a `Segment` based on timestamp
- Rendered as small dot/marker on timeline UI

**Validation**:
- `resourceId` must reference existing Resource
- `timestamp` must be between Resource's createdAt and deletedAt (if present)
- `eventType` must be non-empty string
- `timestamp` must have millisecond precision or better

---

### Namespace

Represents a Kubernetes namespace for filtering and organization.

**Type Definition**
```typescript
interface Namespace {
  name: string;                  // Namespace identifier (e.g., "default", "kube-system")
  displayName?: string;          // Human-readable name (same as name if not provided)
  resourceCount: number;         // Number of resources in this namespace
}
```

**Attributes**:
- `name`: Kubernetes namespace identifier
- `displayName`: For UI display (defaults to name)
- `resourceCount`: Count of unique resources in this namespace (for filter UI)

**Relationships**:
- Contains multiple `Resource` instances
- Populated from metadata API on initial page load

**Validation**:
- `name` must be non-empty and match Kubernetes namespace naming rules
- `resourceCount` must be ≥ 0
- Name must be unique in dataset

---

### Kind

Represents a Kubernetes resource type/kind for filtering and categorization.

**Type Definition**
```typescript
interface Kind {
  name: string;                  // Kind name (e.g., "Pod", "Deployment")
  group?: string;                // API group (e.g., "apps", empty for core)
  version?: string;              // API version within group
  displayName?: string;          // Human-readable name for UI
  resourceCount: number;         // Number of resources of this kind
}
```

**Attributes**:
- `name`: Resource kind identifier
- `group`: Optional API group (empty string for core v1 resources)
- `version`: Optional API version
- `displayName`: For UI display (defaults to name if not provided)
- `resourceCount`: Count of resources matching this kind

**Display Format**:
- If group is empty: `{name}` (e.g., "Pod")
- If group present: `{name}.{group}` (e.g., "Deployment.apps")

**Relationships**:
- Categorizes multiple `Resource` instances
- Populated from metadata API on initial page load

**Validation**:
- `name` must be non-empty
- `resourceCount` must be ≥ 0
- Combination of (name, group, version) must be unique

---

## Data Flow

### 1. Initial Page Load

```
User opens UI
  ↓
fetch /api/metadata
  ↓
Populate Namespace list (for filter dropdown)
Populate Kind list (for filter dropdown)
  ↓
fetch /api/resources (all resources, all time)
  ↓
Build in-memory Segment tree indexed by resourceId
  ↓
Render Timeline with all resources visible
```

### 2. Apply Filters

```
User selects namespace/kind/search
  ↓
JavaScript filters in-memory Segment/Resource data
  ↓
Update filtered result set (no API call needed)
  ↓
Re-render Timeline with filtered resources
```

### 3. Select Segment

```
User clicks segment on timeline
  ↓
Identify resourceId + segmentId from click target
  ↓
fetch /api/events/{resourceId} (get all events for resource)
  ↓
Filter events by segment time window
  ↓
Render DetailPanel with:
  - Resource metadata
  - Segment status + time + message
  - Configuration diff (vs. previous segment)
  - Filtered event list
```

### 4. Navigate Segment History

```
User presses Left/Right arrow (or clicks navigation)
  ↓
Calculate previous/next segment for same resource
  ↓
Re-fetch detail panel content if needed
  ↓
Update DetailPanel display
```

---

## Data Transformation

### From Backend to Frontend

**Backend Provides**:
- List of Resources with all Segments embedded
- List of AuditEvents for requested resource

**Frontend Transforms**:

```typescript
// Raw backend response
{
  resources: [
    {
      id: "default-nginx-1",
      name: "nginx",
      kind: "Pod",
      namespace: "default",
      segments: [
        { id: "seg1", status: "Ready", startTime, endTime, message, configuration }
      ]
    }
  ]
}

// Transformed for frontend rendering
Map<resourceId, Resource>  // Fast lookup by resource
Map<resourceId, Segment[]> // Segments grouped by resource (sorted by startTime)
Map<segmentId, Segment>    // Fast segment lookup

// Filtered view (from UI filter selections)
FilteredResources: Segment[] // Flattened and filtered by active filters
```

### Timeline Rendering Calculation

For each visible Segment:
```
xStart = scale(segment.startTime)     // D3 time scale
xEnd = scale(segment.endTime)
width = xEnd - xStart
yPos = resourceIndex * resourceHeight  // D3 band scale
color = STATUS_COLORS[segment.status]
```

---

## Performance Considerations

### Data Structure Choices

1. **Map<resourceId, Segment[]>**: O(1) lookup by resource, O(n) to filter
2. **Sorted Segment arrays**: Allow binary search for "segments in time window"
3. **In-memory filtering**: No additional API calls when filters change

### Rendering Optimization

1. **Virtualization**: Only render segments with `xEnd > viewport.xStart && xStart < viewport.xEnd`
2. **Event dots**: Only render events with timestamps in visible time window
3. **Memoization**: Segment component is React.memo to prevent re-renders when siblings update

### Caching Strategy

1. **Initial data**: Fetched once on page load, stored in Context
2. **Metadata**: Cached indefinitely (changes infrequently)
3. **Detail panel data**: Fetched on demand, cached per resource
4. **Filter results**: Computed on-demand, cached in useMemo

---

## API Response Schema

### GET /api/metadata

```json
{
  "namespaces": [
    {
      "name": "default",
      "resourceCount": 15
    },
    {
      "name": "kube-system",
      "resourceCount": 42
    }
  ],
  "kinds": [
    {
      "name": "Pod",
      "group": "",
      "version": "v1",
      "resourceCount": 10
    },
    {
      "name": "Deployment",
      "group": "apps",
      "version": "v1",
      "resourceCount": 5
    }
  ]
}
```

### GET /api/resources

```json
{
  "resources": [
    {
      "id": "default-nginx-1",
      "name": "nginx",
      "kind": "Pod",
      "apiVersion": "v1",
      "namespace": "default",
      "createdAt": "2025-11-20T10:00:00Z",
      "deletedAt": null,
      "segments": [
        {
          "id": "seg-uuid-1",
          "status": "Ready",
          "startTime": "2025-11-20T10:00:00Z",
          "endTime": "2025-11-21T14:30:00Z",
          "message": "Pod is running",
          "configuration": { ... }
        }
      ]
    }
  ]
}
```

### GET /api/events/{resourceId}

```json
{
  "events": [
    {
      "id": "evt-uuid-1",
      "timestamp": "2025-11-20T10:00:15Z",
      "eventType": "create",
      "user": "kubectl",
      "message": "Pod created successfully"
    },
    {
      "id": "evt-uuid-2",
      "timestamp": "2025-11-21T14:30:00Z",
      "eventType": "delete",
      "user": "kubectl",
      "message": "Pod terminated"
    }
  ]
}
```

---

## State Management

### React Context Structure

```typescript
// Data fetched from backend
interface DataContextType {
  resources: Map<string, Resource>;
  segments: Map<string, Segment[]>;
  events: Map<string, AuditEvent[]>;
  loading: boolean;
  error?: string;
}

// User filter selections
interface FilterContextType {
  selectedNamespaces: Set<string>;
  selectedKinds: Set<string>;
  searchTerm: string;
  filteredSegments: Segment[];
}

// UI interaction state
interface SelectionContextType {
  selectedSegmentId?: string;
  detailPanelOpen: boolean;
  timelineZoom: number;
  timelinePanOffset: number;
}
```

---

## Validation Rules Summary

| Entity | Field | Rule |
|--------|-------|------|
| Resource | id | Must be unique, non-empty |
| Resource | name | Non-empty, max 253 chars |
| Resource | namespace | Must exist in Namespace list |
| Resource | kind | Must exist in Kind list |
| Segment | id | Must be unique, non-empty |
| Segment | resourceId | Must reference existing Resource |
| Segment | status | Must be one of 5 enum values |
| Segment | startTime < endTime | Always true |
| Segment | time within Resource lifespan | Segment times within [created, deleted] |
| AuditEvent | timestamp | Within Resource lifespan |
| AuditEvent | eventType | Non-empty string |
| Namespace | name | Unique, non-empty |
| Kind | (name, group, version) | Unique combination |

Data model complete. Ready for Phase 1 contract generation.
