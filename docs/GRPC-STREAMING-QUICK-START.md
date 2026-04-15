# gRPC Streaming Implementation - Quick Start Guide

**Date:** 2025-12-17
**Scope:** Timeline endpoint with server-side streaming
**Timeline:** 3-4 weeks (14 working days)

## TL;DR

**What:** Server-side gRPC streaming with intelligent batching
**Why:** 12x faster perceived load time (2,540ms → 150ms)
**How:** Count-first protocol + kind-grouped batches + client batching
**When:** 3-4 weeks implementation

## Key Design Decisions

### 1. Server-Side Streaming (Not Unary)

**Why streaming:**
- ✅ Count arrives in 50ms (skeleton rendering)
- ✅ First data in 150ms (visible viewport)
- ✅ 12x faster perceived performance
- ✅ Only 2-3 re-renders (acceptable)

**Protocol:**
```
Step 1: Send metadata (count, kinds) → 50ms
Step 2: Send first batch (10-20 resources) → 150ms
Step 3: Send remaining batches (80-100 per batch) → Background
```

### 2. Server-Side Grouping and Ordering

**Sorting strategy:**
```go
// Sort by: kind → namespace → name
// Groups naturally align with batches
// Overhead: ~2ms (negligible)
```

**Batching strategy:**
```
Batch 1: Pod resources (15 pods)
Batch 2: Deployment resources (8 deployments)
Batch 3: Service resources (12 services)
... (one kind per batch when possible)
```

### 3. Client-Side Intelligent Batching

**Batching logic:**
```typescript
// First batch: Send as soon as 10-20 resources arrive
// Subsequent: Accumulate 50-100 resources or 200ms timeout
// Result: 2-3 total re-renders
```

**Re-render strategy:**
1. **Render 1 (50ms):** Show skeleton with count
2. **Render 2 (150ms):** Populate visible viewport (10-20 resources)
3. **Render 3-4 (background):** Append remaining resources in chunks

## Performance Comparison

| Metric | REST | Unary gRPC | Streaming gRPC |
|--------|------|------------|----------------|
| **Time to Count** | 2,540ms | 1,900ms | **50ms** ⭐⭐⭐ |
| **Time to First Data** | 2,540ms | 1,900ms | **150ms** ⭐⭐⭐ |
| **Time to Complete** | 2,540ms | 1,900ms | 2,000ms |
| **Re-renders** | 1 | 1 | 2-3 |
| **Perceived Speed** | Slow | Better | **Fast!** ⭐⭐⭐ |

**Perceived Improvement: 12x faster** (2,540ms → 150ms)

## Implementation Checklist

### Week 1: Backend (Day 1-7)

**Day 1-2: Protobuf Schema**
- [ ] Define `TimelineService` with streaming RPC
- [ ] Define `TimelineChunk` (metadata | resources)
- [ ] Define `TimelineMetadata` (count, kinds)
- [ ] Define `ResourceBatch` (resources, kind, batch_number)
- [ ] Generate Go code
- [ ] Generate TypeScript code

**Day 3-4: Sorting and Grouping**
- [ ] Implement `SortAndGroupResources()`
- [ ] Sort by: kind → namespace → name
- [ ] Group by kind
- [ ] Benchmark (<5ms overhead)
- [ ] Unit tests

**Day 5-7: Streaming Service**
- [ ] Implement `GetTimeline()` streaming RPC
- [ ] Send metadata first
- [ ] Stream resources in kind-grouped batches
- [ ] Handle client cancellation
- [ ] Error handling
- [ ] Integration tests

### Week 2: Frontend (Day 8-12)

**Day 8-9: Stream Client**
- [ ] Create `GrpcStreamClient` class
- [ ] Implement intelligent batching logic
- [ ] First batch: 10-20 resources
- [ ] Subsequent: 50-100 resources or 200ms
- [ ] Unit tests

**Day 10-11: API Service**
- [ ] Update `api.ts` with `getTimelineStreaming()`
- [ ] Add progress callbacks (metadata, batch, complete)
- [ ] Transform gRPC types to internal types
- [ ] Error handling

**Day 12: Timeline Component**
- [ ] Update component to use streaming API
- [ ] Implement skeleton rendering (on metadata)
- [ ] Implement progressive loading (on batch)
- [ ] Loading indicators
- [ ] Error states

### Week 3: E2E & Deployment (Day 13-14)

**Day 13-14: E2E Tests**
- [ ] Test streaming performance
- [ ] Test grouping/ordering
- [ ] Test client batching
- [ ] Test error handling
- [ ] Test cancellation

**Week 4: Deployment**
- [ ] Deploy to staging
- [ ] Load testing
- [ ] Gradual rollout (10% → 50% → 100%)
- [ ] Monitor metrics
- [ ] Production stable

## Code Examples

### Backend: Streaming RPC

```go
func (s *TimelineService) GetTimeline(req *pb.TimelineRequest, stream pb.TimelineService_GetTimelineServer) error {
    // Execute queries
    // ... build resources

    // Sort and group
    groups := SortAndGroupResources(resourceMap)

    // Step 1: Send metadata
    metadata := &pb.TimelineMetadata{
        TotalCount: int32(len(resourceMap)),
        Kinds: extractKinds(groups),
    }
    stream.Send(&pb.TimelineChunk{Chunk: &pb.TimelineChunk_Metadata{Metadata: metadata}})

    // Step 2: Stream batches (one kind per batch)
    for i, group := range groups {
        batch := &pb.ResourceBatch{
            Resources: convertResources(group.Resources),
            Kind: group.Kind,
            BatchNumber: int32(i),
            IsLastBatch: i == len(groups)-1,
        }
        stream.Send(&pb.TimelineChunk{Chunk: &pb.TimelineChunk_Resources{Resources: batch}})
    }

    return nil
}
```

### Frontend: Stream Client

```typescript
grpcStreamClient.streamTimeline(startTime, endTime, filters, {
    onMetadata: (metadata) => {
        // Render skeleton
        setTotalCount(metadata.totalCount);
    },

    onBatch: (batch, accumulatedCount) => {
        // Append resources (triggers re-render)
        setResources(prev => [...prev, ...batch.resourcesList]);
    },

    onComplete: () => {
        setLoading(false);
    },
});
```

### Frontend: Timeline Component

```typescript
const Timeline = () => {
    const [resources, setResources] = useState([]);
    const [totalCount, setTotalCount] = useState(0);

    useEffect(() => {
        api.getTimelineStreaming(start, end, filters, {
            onMetadata: (metadata) => setTotalCount(metadata.totalCount),
            onBatch: (batch) => setResources(prev => [...prev, ...batch]),
        });
    }, [start, end, filters]);

    return (
        <>
            {totalCount > 0 && <Skeleton count={totalCount} />}
            <TimelineView resources={resources} />
        </>
    );
};
```

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| **Stream interruption** | Retry logic + fallback to REST |
| **Client batching bugs** | Comprehensive tests + feature flag |
| **Performance regression** | Load testing + gradual rollout |
| **Re-render jank** | React.memo + virtualization |

## Success Criteria

### After Week 4

- [ ] Time to metadata: <100ms
- [ ] Time to first data: <200ms
- [ ] Time to complete: <2,500ms
- [ ] Re-render count: ≤4
- [ ] Error rate: <0.1%
- [ ] Zero production incidents

## Quick Reference

### Files to Create/Modify

**Backend:**
- `proto/spectre/v1/timeline.proto` (new)
- `internal/grpc/timeline_sorter.go` (new)
- `internal/grpc/timeline_service.go` (new)
- `cmd/server/grpc.go` (new)

**Frontend:**
- `ui/src/generated/...` (generated)
- `ui/src/services/grpcStreamClient.ts` (new)
- `ui/src/services/api.ts` (modify)
- `ui/src/components/Timeline.tsx` (modify)

**Tests:**
- `tests/e2e/timeline_performance_test.go` (modify)

### Dependencies

**Backend:**
```bash
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go get github.com/improbable-eng/grpc-web@latest
```

**Frontend:**
```bash
npm install grpc-web google-protobuf
npm install --save-dev @types/google-protobuf grpc-tools
```

## Next Steps

1. ✅ Review implementation plan
2. ✅ Get team approval
3. 🚀 Start Day 1: Define Protobuf schema
4. 📅 Daily standups to track progress
5. 🎯 Deploy to production Week 4

## Questions?

**Q: Why streaming instead of pagination?**
A: Streaming provides progressive rendering (150ms vs 290ms) with minimal re-renders (3 vs 9).

**Q: Why not unary gRPC?**
A: Unary requires buffering complete response (1,900ms). Streaming shows data in 150ms.

**Q: What about complexity?**
A: Medium-High, but justified by 12x perceived improvement. 14 days of work.

**Q: What if streaming fails?**
A: Fallback to REST API (maintained during migration).

---

**Full Details:** See `GRPC-STREAMING-IMPLEMENTATION-PLAN.md` (40KB)

**Ready to implement? Let's go! 🚀**
