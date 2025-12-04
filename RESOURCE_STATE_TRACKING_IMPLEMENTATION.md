# Resource State Tracking Implementation - Option 3

## Overview

This document describes the implementation of consistent resource state tracking across hour boundaries in the Spectre storage engine. This solves the problem where long-lived resources that haven't changed recently become invisible when querying current time ranges.

## Problem Statement

Previously, when querying recent data (e.g., "last 1 hour"), resources that hadn't had any events in that time period would not appear, even if they existed and were active in the system. For example:
- A resource was created 7 days ago
- No updates for 7 days
- Query for "last 1 hour" → resource is not visible
- User sees an incomplete picture of the current state

## Solution: State Snapshot in File Metadata (Option 3)

The implementation adds resource state snapshots to file metadata without requiring full file scans or synthetic events. When a storage file closes, it extracts and persists the final state of each resource in that file.

## Key Components

### 1. Data Structures

**ResourceLastState** - Represents the final state of a resource
```go
type ResourceLastState struct {
    UID          string                  // Unique identifier
    EventType    string                  // CREATE, UPDATE, or DELETE
    Timestamp    int64                   // When state was last observed
    ResourceData json.RawMessage         // Full Kubernetes object
}
```

**Extended IndexSection** - Now includes final resource states
```go
type IndexSection struct {
    // ... existing fields ...
    FinalResourceStates map[string]*ResourceLastState  // Key: group/version/kind/namespace/name
}
```

### 2. File Lifecycle with State Carryover

1. **Hour N**: Events are written and finalized
   - When file closes, `extractFinalResourceStates()` reads all blocks
   - Builds map of latest state per resource: `group/version/kind/namespace/name` → `ResourceLastState`
   - Persists in IndexSection.FinalResourceStates

2. **Hour N+1**: New hour file is created
   - State snapshots from hour N are carried forward to hour N+1
   - This enables consistent view across hour boundaries
   - Carried states are updated as new events arrive in hour N+1

3. **Query Time**: Resources appear consistently
   - Regular events from blocks are retrieved
   - State snapshots fill in resources without recent events
   - Results merged transparently

### 3. Core Implementation Points

#### State Extraction (block_storage.go)
```
extractFinalResourceStates()
  ├─ Open BlockReader for current file
  ├─ For each block:
  │   └─ Read all events in order
  ├─ For each event:
  │   └─ Update final state map (latest wins)
  └─ Return map keyed by resource identifier
```

#### State Carryover (storage.go)
```
getOrCreateCurrentFile()
  ├─ Check if hour boundary crossed
  ├─ If yes:
  │   ├─ Extract final states from current file
  │   ├─ Close current file (with states persisted)
  │   ├─ Create new file
  │   └─ Carry over states to new file
  └─ Return current file
```

#### Query Enhancement (query.go)
```
queryFile()
  ├─ Query blocks normally (actual events)
  ├─ Track which resources have events
  ├─ For each state snapshot:
  │   ├─ Skip if resource has events in results
  │   ├─ Skip if resource is DELETE type
  │   ├─ Skip if outside query time range
  │   ├─ Check resource filter match
  │   └─ Create synthetic event from state
  └─ Return merged results
```

### 4. Cleanup Strategy

**CleanupOldStateSnapshots(maxAgeDays)**
- Removes state snapshots older than retention period
- Keeps non-deleted resources even if old (they're still relevant)
- Removes deleted resources older than cutoff (they're gone anyway)
- Rewrites index section in-place

### 5. Key Design Decisions

| Aspect | Decision | Reason |
|--------|----------|--------|
| **State Storage** | File metadata (IndexSection) | Single source of truth, no dual structures |
| **State Extraction** | On file close | Already have all blocks in memory |
| **State Carryover** | At hour boundary | Transparent to queries, works with existing architecture |
| **Deleted Resources** | Excluded from view | Consistency with Kubernetes semantics |
| **Filtering** | Applied to carried states | Only relevant resources appear |

## Implementation Highlights

### Single Source of Truth
- States stored only in storage files, not in-memory persistence
- No need to restore state map on startup
- Metadata automatically deserialized when file is opened

### Transparency to Users
- No changes to query API
- State snapshots appear as regular events
- Marked with synthetic IDs for auditing: `state-{resourceKey}-{timestamp}`

### Consistent Semantics
- Deleted resources don't appear (they're gone)
- Non-deleted resources kept even if old (for consistency)
- 14-day retention for cleanup (configurable)

### Performance
- No full file scans during queries
- State snapshots only created at file close (hourly)
- Filtering applied before synthetic event creation

## Testing

Comprehensive test suite in `state_snapshot_test.go`:

1. **TestStateSnapshot_BasicPersistence** - States persist to disk
2. **TestStateSnapshot_ConsistentView** - States carry across hours
3. **TestStateSnapshot_DeletedResourceHidden** - Deleted resources don't appear
4. **TestStateSnapshot_FilteredQuery** - Filters work with carried states
5. **TestStateSnapshot_MultipleUpdates** - Latest state wins
6. **TestStateSnapshot_Cleanup** - Old deleted states removed
7. **TestStateSnapshot_NonDeletedResourcesPreserved** - Non-deleted kept

All tests pass ✓

## Migration Notes

### Backward Compatibility
- Old files without state snapshots work fine (FinalResourceStates is empty map)
- New files can read old files without issues
- State snapshots gradually build as files are updated

### File Format
- No format version bump needed (optional field in IndexSection)
- Existing file readers ignore FinalResourceStates
- Existing readers still work with new files

## Operational Usage

### Enable State Carryover
No configuration needed - works automatically when files close.

### Cleanup Old States
```go
storage.CleanupOldStateSnapshots(14)  // 14-day retention
```

### Query with Consistent View
```go
executor := NewQueryExecutor(storage)
result, err := executor.Execute(query)
// result.Events includes both actual events and state snapshots
```

## Future Enhancements

1. **Periodic State Snapshots** - Daily or weekly snapshots for faster lookups
2. **State Index** - Separate index file for O(1) resource lookup
3. **State Compression** - Share common state data across snapshots
4. **Distributed State** - State gossip across nodes

## Comparison with Alternatives

### vs Option 1 (In-Memory Map)
- ✓ No persistence complexity
- ✓ Single source of truth
- ✓ Automatic on startup
- ✓ No synchronization issues

### vs Option 2 (File Sweep & Carryover Events)
- ✓ No expensive file scans
- ✓ No synthetic events polluting audit trail
- ✓ Cleaner query semantics
- ✓ Easier to reason about

### vs State Snapshot in Metadata
- This IS Option 3!
- Best balance of simplicity and functionality
- Leverages existing metadata infrastructure

## Conclusion

Option 3 provides a clean, maintainable solution for consistent resource state tracking across time boundaries. By storing final resource states in file metadata and carrying them forward between hours, users get a consistent view of all resources regardless of event recency, without architectural complexity or performance overhead.
