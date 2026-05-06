# Timeline Module Design

## Goal

Deepen the timeline module so timeline semantics live behind one seam instead of being split across HTTP, Connect, and compatibility wrappers.

## Current Friction

- `internal/api/handlers/timeline_handler.go` owns request parsing and response writing, but timeline execution policy still leaks through.
- `internal/api/timeline_connect_service.go` re-implements pagination policy, executor capability checks, Kubernetes Event side-query behavior, and batching decisions.
- `internal/api/timeline_grpc_service.go` duplicates part of the same transport orchestration, even though the active server path registers Connect, not the standalone gRPC adapter.
- `internal/api/timeline_service.go` and `internal/app/timeline/*` form a shallow seam: callers still need to know too much about the implementation.

## Design Decision

Create a deeper timeline module in `internal/app/timeline` that owns:

- active executor selection
- resource query execution
- Kubernetes Event side-query execution
- executor-aware pagination policy
- client-side fallback pagination
- timeline index construction
- stream-ready entry selection

Transport adapters keep only transport concerns:

- HTTP query parsing and JSON encoding
- Connect protobuf decoding and stream emission
- gRPC compatibility behavior

## Recommended Seam

The application seam starts after transport parsing:

- adapters normalize transport input into `models.QueryRequest` plus optional `models.PaginationRequest`
- `internal/app/timeline` returns a canonical execution result
- adapters render that result as page JSON or streamed batches

This preserves flexibility without forcing transport request types into the core module.

## First Slice

Implement one narrow architectural slice:

1. Add app-level timeline execution and pagination result types.
2. Move entry pagination logic from the Connect adapter into `internal/app/timeline`.
3. Move executor-aware pagination orchestration from the Connect adapter into `internal/app/timeline`.
4. Keep HTTP behavior stable.
5. Update the Connect adapter to consume the deeper seam.

## Invariants

- Timeline resource grouping and status segment semantics must remain unchanged.
- Kubernetes Event attachment must remain unchanged.
- Pagination behavior for Connect/gRPC-Web callers must remain unchanged.
- No transport adapter should need to branch on executor capability once this slice lands.

## Testing

- Add app-level tests for executor-aware pagination behavior.
- Move pagination behavior assertions from adapter-level tests to app-level tests where possible.
- Keep adapter tests focused on transport encoding and streaming behavior.
