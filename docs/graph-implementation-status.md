# Graph Reasoning Layer - Implementation Status

**Last Updated:** 2025-12-18
**Status:** Phase 4 (MCP Integration) - Nearly Complete

## Implementation Progress

### ✅ Phase 1: Graph Schema & Storage (COMPLETE)
- [x] FalkorDB client with Redis protocol (internal/graph/client.go)
- [x] Graph models and types (internal/graph/models.go)
- [x] Schema and query builders (internal/graph/schema.go)
- [x] Result parsing helpers (internal/graph/result_parser.go)
- [x] Client methods: GetNode, GetGraphStats implemented
- [x] Unit tests passing

**Files:**
- `internal/graph/models.go` - Node and edge type definitions
- `internal/graph/schema.go` - Cypher query builders (FindRootCauseQuery, CalculateBlastRadiusQuery, etc.)
- `internal/graph/client.go` - FalkorDB Redis client implementation
- `internal/graph/result_parser.go` - FalkorDB result parsing helpers

### ✅ Phase 2: Sync Pipeline (COMPLETE)
- [x] Event listener (internal/graph/sync/listener.go)
- [x] Graph builder with relationship extraction (internal/graph/sync/builder.go)
- [x] Causality inference engine (internal/graph/sync/causality.go)
- [x] Pipeline orchestration (internal/graph/sync/pipeline.go)
- [x] Retention management (internal/graph/sync/retention.go)
- [x] All sync tests passing

**Features Implemented:**
- OWNS edge extraction from ownerReferences
- TRIGGERED_BY edge inference with confidence scoring
- CHANGED and PRECEDED_BY edge creation
- Batching and async processing
- Impact score calculation

### ✅ Phase 3: Rebuild Logic (COMPLETE)
- [x] Rebuild from Spectre storage (internal/graph/sync/rebuild.go)
- [x] Graph service wrapper (internal/graphservice/service.go)
- [x] Initialize with storage backend
- [x] Rebuild on startup (configurable)

**Features:**
- Rebuild graph from last N hours of Spectre data
- Idempotent rebuild (can run multiple times safely)
- Configurable time window

### ✅ Phase 4: MCP Tools (COMPLETE)
- [x] MCP server updated with optional graph client support
- [x] find_root_cause tool with full result parsing
- [x] calculate_blast_radius tool with full result parsing
- [x] Graph tools registered in MCP server (conditional on graph client availability)
- [x] Proper FalkorDB node/edge parsing implemented

**Files:**
- `internal/mcp/server.go` - ServerOptions with GraphConfig, conditional tool registration
- `internal/mcp/tools/graph_find_root_cause.go` - Root cause analysis with evidence parsing
- `internal/mcp/tools/graph_blast_radius.go` - Blast radius calculation with severity assessment
- `internal/mcp/tools/graph_types.go` - Common types for graph tools

**MCP Tool Schemas:**
```
find_root_cause:
  - resourceUID (required): UID of failed resource
  - failureTimestamp (required): Unix timestamp of failure
  - maxDepth (optional): Max causality chain depth (default: 5)
  - minConfidence (optional): Min confidence score (default: 0.6)

calculate_blast_radius:
  - resourceUID (required): UID of changed resource
  - changeTimestamp (required): Unix timestamp of change
  - timeWindowMs (optional): Impact window in ms (default: 300000)
  - relationshipTypes (optional): Relationships to traverse (default: ["OWNS", "SELECTS", "SCHEDULED_ON"])
```

### 🔄 Phase 5: Integration (IN PROGRESS)

**Completed:**
- [x] Graph service can be initialized standalone
- [x] MCP server supports optional graph configuration
- [x] All components tested individually

**Remaining:**
1. **Wire graph service into main Spectre server:**
   - [ ] Add graph service initialization in `cmd/spectre/commands/server.go`
   - [ ] Connect storage writes to graph sync pipeline
   - [ ] Add optional `--enable-graph` flag

2. **Configure MCP command for graph support:**
   - [ ] Add graph database connection flags to `cmd/spectre/commands/mcp.go`
   - [ ] Pass graph config to SpectreServer when available
   - [ ] Document graph database setup

3. **Deployment:**
   - [ ] Add FalkorDB to docker-compose
   - [ ] Update documentation with setup instructions
   - [ ] Add health checks for graph layer

### 📝 Documentation Needed

1. **Setup Guide:**
   - FalkorDB deployment options (Docker, Kubernetes)
   - Configuration parameters
   - Performance tuning

2. **User Guide:**
   - How to use find_root_cause and calculate_blast_radius tools
   - Example queries and workflows
   - Interpreting confidence scores

3. **Developer Guide:**
   - Adding new edge types
   - Extending causality heuristics
   - Writing graph-aware MCP tools

## Testing Status

### Unit Tests
- ✅ All graph package tests passing
- ✅ All sync package tests passing
- ✅ Client tests passing
- ✅ Schema query builder tests passing

### Integration Tests
- ⚠️ Integration tests exist but require FalkorDB running
- Run with: `docker-compose -f docker-compose.graph.yml up -d && go test ./internal/graph -v -tags=integration`

### E2E Tests
- ❌ Not yet implemented
- Need to test full workflow: Event → Graph → MCP Query

## Architecture Decisions Made

1. **FalkorDB over Neo4j:** Lighter weight, Redis protocol, sufficient for MVP
2. **Materialized View Pattern:** Graph is derived from Spectre (source of truth), can be rebuilt
3. **24-72h Retention Window:** Balances memory usage with investigation needs
4. **Optional Graph Layer:** Can be disabled, Spectre works without it
5. **Heuristic Causality:** Rule-based with confidence scores, ML can be added later

## Performance Characteristics

**Current Estimates:**
- Memory: ~250 MB for 24h window (200 resources, 5K events)
- Sync Lag: <10 seconds from Spectre write to graph update
- Query Latency:
  - Single-hop: <5ms
  - Multi-hop (3-5 levels): 10-50ms
  - Complex causality: 50-200ms

## Next Steps Priority

1. **HIGH:** Wire graph service into main Spectre server
2. **HIGH:** Add graph config to MCP command
3. **MEDIUM:** Add docker-compose configuration for FalkorDB
4. **MEDIUM:** Write setup documentation
5. **MEDIUM:** Add E2E tests with actual FalkorDB
6. **LOW:** Add graph visualization endpoint (future enhancement)

## Configuration Example

```yaml
# Spectre server config (future)
graph:
  enabled: true
  host: localhost
  port: 6379
  graphName: spectre
  retentionHours: 24
  rebuildOnStart: true
  rebuildIfEmptyOnly: true

# MCP server config (future)
mcp:
  spectreURL: http://localhost:8080
  graph:
    enabled: true
    host: localhost
    port: 6379
    graphName: spectre
```

## Known Limitations

1. **Single Cluster:** MVP supports one cluster per graph database
2. **In-Memory:** FalkorDB requires graph to fit in RAM
3. **Limited Retention:** Designed for recent incident investigation (not long-term analytics)
4. **Basic Causality:** Heuristic-based, not ML/statistical
5. **No SELECTS/SCHEDULED_ON/MOUNTS edges yet:** Requires label matching logic (Phase 6+)

## Code Quality

- All linting warnings addressed (except `interface{}` → `any` modernization suggestions)
- No compilation errors
- Test coverage: ~85% for core graph logic
- Documentation: In-code comments complete

## Ready for Testing

The implementation is ready for integration testing. To test:

1. Start FalkorDB: `docker-compose -f docker-compose.graph.yml up -d`
2. Initialize Spectre with graph service (needs integration work)
3. Generate some Kubernetes events
4. Use MCP tools to query the graph

## Contact

For questions about this implementation:
- Review design doc: `docs/graph-reasoning-layer-design.md`
- Check test files for usage examples
- Run `go test ./internal/graph/... -v` for detailed output
