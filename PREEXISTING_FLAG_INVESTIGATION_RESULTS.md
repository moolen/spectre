# PreExisting Flag Investigation Results

## Summary

The `PreExisting` flag **IS** working correctly and **IS** being set to `true` when appropriate. However, the flag doesn't appear in query results because **state snapshot events are never reaching the ResourceBuilder**.

## Root Cause

The issue is not with the PreExisting flag logic itself, but with how state snapshots are queried:

### The Problem Flow

```
Query: last 30 minutes (start = now-30min, end = now)
Resource created: 80 minutes ago

1. Query Executor receives the query
2. Finds storage file from 80 minutes ago
3. Block in that file contains the resource event
4. **Block's timestamp range: 80min ago to now**
5. **Block's time range is BEFORE query start time**
6. **DECISION: Skip this block (optimization)**
7. State snapshots in block metadata are never read
8. ResourceBuilder never receives state snapshot events
9. PreExisting flag never gets set ❌
```

### Why Blocks Are Skipped

In `query.go:queryFile()` (lines 200-207):

```go
// Check if block overlaps with query time range
if blockMeta.TimestampMax < startTimeNs || blockMeta.TimestampMin > endTimeNs {
    qe.logger.Debug("File %s: skipping block %d (time range [%d, %d] outside query [%d, %d])",
        filePath, blockMeta.ID, blockMeta.TimestampMin, blockMeta.TimestampMax, startTimeNs, endTimeNs)
    segmentsSkipped++
    continue  // ← BLOCK IS SKIPPED
}
```

This is a **correct optimization** - if a block's events are all from before the query start, we don't need to read them. But it means **we also don't read the block's metadata**, which contains the state snapshots.

## The Issue

**State snapshots are stored in block metadata (`IndexSection.FinalResourceStates`), but that metadata is only read when the block is processed.**

If the block is skipped due to time range, the metadata (and thus state snapshots) are never read.

### Test Evidence

From `TestIntegration_StateSnapshotPreExistingFlag`:

```
segments_scanned=0 segments_skipped=1
```

The block is skipped, so:
- No events are returned
- No state snapshots are returned
- PreExisting flag never gets set (because events never reach ResourceBuilder)
```

## Why You're Not Seeing PreExisting=true

When you query the timeline endpoint and a resource appears with an event from before the query window, here's what happens:

### Scenario: Resource created 80 min ago, queried last 30 min

**Expected behavior (what should happen):**
1. Query recognizes resource didn't have events in last 30 min
2. Includes state snapshot showing "resource existed before query start"
3. Sets `PreExisting = true`
4. Returns resource in response with `preExisting: true`

**Actual behavior (what's happening):**
1. Block is outside query time range
2. Block is skipped entirely
3. State snapshots in block metadata are never read
4. State snapshot events never created
5. ResourceBuilder never called
6. PreExisting flag never set
7. Resource doesn't appear in query at all

## What the Code is Actually Doing

The code is **correctly** structured to set `PreExisting = true`:

```go
// In resource_builder.go - WORKING CORRECTLY
func (rb *ResourceBuilder) IsPreExisting(resourceUID string, allEvents []models.Event) bool {
    // Sort events by timestamp
    // Check if first event ID starts with "state-"
    return strings.HasPrefix(firstEvent.ID, "state-")
}

// Called from BuildResourcesFromEvents:
if len(resource.StatusSegments) > 0 {
    resource.PreExisting = rb.IsPreExisting(uid, baseEvents)  // ← THIS WORKS
}
```

**Test proof:**
```
Resource.PreExisting: true ✓  ← Flag IS set correctly when state snapshot events reach the builder
```

## Solutions

To make PreExisting=true appear in query results when resources exist from before the query window, you need to:

### Option 1: Read Block Metadata Even for Skipped Blocks (Recommended)

When a block's time range is before the query start, still read its `FinalResourceStates` from metadata.

```go
// In query.go:queryFile()
if blockMeta.TimestampMax < startTimeNs || blockMeta.TimestampMin > endTimeNs {
    // CURRENT: Skip block entirely
    segmentsSkipped++
    continue

    // PROPOSED: Skip reading block events, but still read state snapshots
    // stateSnapshots = getStateSnapshotsFromBlockMetadata(blockMeta)
    // results.append(stateSnapshots)
}
```

**Pros:**
- Minimal change
- Follows existing pattern
- No need to change metadata storage

**Cons:**
- Need to access block metadata for skipped blocks
- State snapshots would still be filtered by the boundary check

### Option 2: Change When State Snapshots Are Created

Instead of extracting state snapshots at file close, extract them every hour from the previous file's state snapshot.

**Pros:**
- Would automatically carry forward states

**Cons:**
- Changes the fundamental state tracking mechanism
- More complex

### Option 3: Filter on the Start Boundary Correctly

The reverted commit attempted this - read block metadata for old blocks, but only include state snapshots that fall within the query range.

## What You Actually Need

Based on your requirement: **"State snapshots from before the query start time SHOULD be included to show the resource pre-existed"**

This means:
1. State snapshots need to reach the ResourceBuilder
2. They need to be created with `PreExisting = true`
3. They need to appear in the API response

**Current code can do this**, but only if state snapshot events reach the ResourceBuilder, which currently doesn't happen for old blocks.

## Test Results Summary

### What IS Working ✓
- PreExisting flag logic is correct
- ID prefix detection works
- JSON serialization works
- Flag is properly set when state snapshots reach ResourceBuilder
- All 40+ tests pass

### What ISN'T Working ❌
- State snapshots from old blocks don't reach ResourceBuilder
- Block skipping optimization prevents metadata from being read
- Query returns 0 results instead of returning resource with preExisting=true

## Recommended Fix

Modify `query.go:queryFile()` to read state snapshots from block metadata even when the block itself is skipped:

```go
if blockMeta.TimestampMax < startTimeNs || blockMeta.TimestampMin > endTimeNs {
    // Block events are outside query range - skip reading them
    segmentsSkipped++

    // BUT: still include state snapshots from this block
    // These show resources that existed before the query window
    if len(blockMeta.FinalResourceStates) > 0 {
        stateEvents := qe.getStateSnapshotEvents(blockMeta.FinalResourceStates, query, resourcesWithEvents)
        results = append(results, stateEvents...)
    }
    continue
}
```

This would allow PreExisting resources from old blocks to appear in query results with `PreExisting = true`.

## Conclusion

**The PreExisting flag implementation is correct and working.** The issue is that state snapshots from old blocks never reach the code that sets the flag because the blocks are skipped due to time range optimization.

To show resources with `PreExisting = true` in query results, you need to modify the block skipping logic to still read metadata state snapshots even when the block events are outside the query range.
