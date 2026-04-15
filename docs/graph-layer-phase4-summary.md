# Graph Reasoning Layer - Phase 4 Implementation Summary

**Status**: 🚧 IN PROGRESS (Core design complete, implementation 70%)
**Date**: 2025-12-18
**Phase**: 4 of 5 (MCP Tools for LLM Integration)

---

## Overview

Phase 4 exposes the graph reasoning capabilities to LLMs through MCP (Model Context Protocol) tools. The core tool designs are complete with structured inputs/outputs optimized for LLM interaction.

**Progress:**
- ✅ Tool interface design
- ✅ Data structure definitions
- 🚧 Tool implementations (70%)
- ⏳ MCP server integration
- ⏳ Result parsing from graph queries
- ⏳ Testing

---

## MCP Tools Designed

### 1. **find_root_cause** - Root Cause Analysis

**Purpose:** Traces causality backward from a failure to identify likely root causes with confidence scores.

**Input:**
```json
{
  "resourceUID": "pod-abc123",
  "failureTimestamp": 1703001234,
  "maxDepth": 5,              // Max hops to trace back
  "minConfidence": 0.6        // Min confidence threshold
}
```

**Output:**
```json
{
  "candidates": [
    {
      "resource": {
        "uid": "deploy-456",
        "kind": "Deployment",
        "namespace": "production",
        "name": "frontend"
      },
      "changeEvent": {
        "id": "event-789",
        "timestamp": 1703001200000000000,
        "eventType": "UPDATE",
        "status": "Ready",
        "errorMessage": null
      },
      "evidence": [
        {
          "type": "TRIGGERED_BY",
          "description": "Deployment update triggered Pod rollout",
          "confidence": 0.9
        },
        {
          "type": "OWNS",
          "description": "Ownership path: Deployment → ReplicaSet → Pod"
        }
      ],
      "confidenceScore": 0.9,
      "timeLagMs": 34000
    }
  ],
  "investigationPrompt": "Root cause analysis for resource pod-abc123:\n\nMost likely cause: Deployment 'frontend' (confidence: 90%)\nTime lag: 34 seconds\n\nRecommended investigation steps:\n1. Review the Deployment change at timestamp 1703001200\n2. Check if the change introduced a configuration error\n3. Verify ownership chain: Deployment → failed resource\n4. Review logs for the Deployment during this time window\n5. Check for related changes in ConfigMaps, Secrets, or PVCs",
  "queryExecutionMs": 45
}
```

**Key Features:**
- Follows TRIGGERED_BY edges backward
- Traverses OWNS edges to find parent resources
- Returns top candidates ranked by confidence
- Includes human-readable investigation prompts
- Respects confidence thresholds (filters low-confidence links)

---

### 2. **calculate_blast_radius** - Impact Analysis

**Purpose:** Determines which resources were affected by a change within a time window.

**Input:**
```json
{
  "resourceUID": "node-123",
  "changeTimestamp": 1703001000,
  "timeWindowMs": 300000,     // 5 minutes
  "relationshipTypes": ["SCHEDULED_ON", "OWNS"]
}
```

**Output:**
```json
{
  "triggerResource": {
    "uid": "node-123",
    "kind": "Node",
    "name": "worker-1"
  },
  "triggerTimestamp": 1703001000000000000,
  "impactedResources": [
    {
      "resource": {
        "uid": "pod-789",
        "kind": "Pod",
        "namespace": "production",
        "name": "frontend-abc"
      },
      "relationship": {
        "type": "SCHEDULED_ON",
        "distance": 1
      },
      "impactEvents": [
        {
          "timestamp": 1703001050000000000,
          "status": "Error",
          "errorMessage": "NodeNotReady",
          "lagFromTrigger": 50000
        }
      ],
      "severity": "high"
    }
  ],
  "totalImpacted": 12,
  "byKind": {
    "Pod": 10,
    "Job": 2
  },
  "bySeverity": {
    "critical": 3,
    "high": 5,
    "medium": 4
  },
  "investigationPrompt": "Blast radius analysis for resource node-123:\n\nTotal impacted resources: 12\nCritical/High severity: 8\n\nResources affected by kind:\n  - 10 Pod(s)\n  - 2 Job(s)\n\nInvestigation steps:\n1. Review the triggering change at timestamp 1703001000\n2. Identify if the change was intentional (rollout, scaling) or unexpected\n3. For critical impacts, check error messages and logs\n4. Verify if impacted resources have recovered\n5. Consider rollback if impact was unintentional",
  "queryExecutionMs": 67
}
```

**Key Features:**
- Traverses specified relationship types (SCHEDULED_ON, OWNS, SELECTS)
- Filters to changes within time window
- Categorizes severity (critical, high, medium, low)
- Groups by resource kind
- Calculates impact timing (lag from trigger)

---

### 3. **trace_change_impact** - Change Propagation Timeline

**Purpose:** Follows the propagation of a change through the system chronologically.

**Design** (implementation pending):

**Input:**
```json
{
  "resourceUID": "deploy-123",
  "changeTimestamp": 1703001000,
  "maxHops": 5,
  "timeWindowMs": 600000     // 10 minutes
}
```

**Output:**
```json
{
  "changeChain": [
    {
      "hop": 0,
      "resource": {"kind": "Deployment", "name": "frontend"},
      "event": {"timestamp": 1703001000, "eventType": "UPDATE"},
      "triggeredBy": null
    },
    {
      "hop": 1,
      "resource": {"kind": "ReplicaSet", "name": "frontend-abc"},
      "event": {"timestamp": 1703001005, "eventType": "CREATE"},
      "triggeredBy": {
        "confidence": 0.9,
        "lagMs": 5000,
        "reason": "Deployment update triggered rollout"
      }
    },
    {
      "hop": 2,
      "resource": {"kind": "Pod", "name": "frontend-xyz"},
      "event": {"timestamp": 1703001010, "eventType": "DELETE"},
      "triggeredBy": {
        "confidence": 0.85,
        "lagMs": 5000,
        "reason": "Old ReplicaSet scale-down"
      }
    }
  ],
  "stableAt": 1703001600,
  "totalAffected": 15,
  "investigationPrompt": "Deployment 'frontend' update initiated a rolling restart affecting 15 Pods over 10 minutes. Rollout completed successfully. No errors detected in change chain."
}
```

---

## Implementation Architecture

### File Structure
```
internal/mcp/tools/
├── graph_types.go              # Shared types (GraphResourceInfo, etc.)
├── graph_find_root_cause.go    # Root cause analysis tool
├── graph_blast_radius.go       # Blast radius calculation tool
└── graph_trace_impact.go       # Change impact tracing (pending)
```

### Integration Points

**1. Graph Client Access**
Tools receive `graph.Client` instance:
```go
tool := NewGraphFindRootCauseTool(graphClient)
```

**2. Query Execution**
Uses pre-built query functions from `graph.Schema`:
```go
query := graph.FindRootCauseQuery(uid, timestamp, maxDepth, minConfidence)
result, err := t.graphClient.ExecuteQuery(ctx, query)
```

**3. MCP Server Registration**
Integration with existing MCP server (to be completed):
```go
// In internal/mcp/server.go
s.registerTool(
    "graph_find_root_cause",
    "Trace causality backward from failures with confidence scores",
    tools.NewGraphFindRootCauseTool(graphClient),
    /* JSON schema */
)
```

---

## Anti-Hallucination Design

### 1. **Structured Output**
All tools return structured JSON with:
- Resource UIDs (not guessed names)
- Exact timestamps (nanoseconds)
- Confidence scores (explicit uncertainty)
- Provenance (evidence chains)

### 2. **Investigation Prompts**
Each tool generates human-readable investigation guidance:
- Specific next steps (not generic advice)
- References to actual resources and timestamps
- Confidence scores included in text
- Actionable recommendations

### 3. **Confidence Propagation**
```
Graph TRIGGERED_BY edge (0.9 confidence)
        ↓
Tool output (confidenceScore: 0.9)
        ↓
Investigation prompt ("confidence: 90%")
        ↓
LLM response (must cite confidence)
```

### 4. **Empty Result Handling**
When no causality found:
```
"No clear root cause found for resource X. Investigate:
1. Check if resource existed during the failure window
2. Review manual configuration changes not tracked by Spectre
3. Check external dependencies (databases, external APIs)
4. Review cluster-wide events (node failures, network issues)"
```

---

## Example LLM Interaction

**User:** "Why did the frontend service fail at 2:00 PM?"

**LLM Internal:**
1. Identify Service resource UID from Spectre metadata
2. Convert "2:00 PM" → Unix timestamp
3. Call `graph_find_root_cause`:
   ```json
   {
     "resourceUID": "svc-frontend-uid",
     "failureTimestamp": 1703001200,
     "maxDepth": 5,
     "minConfidence": 0.6
   }
   ```

**Tool Response:**
```json
{
  "candidates": [
    {
      "resource": {"kind": "Deployment", "name": "frontend"},
      "changeEvent": {"eventType": "UPDATE", "timestamp": 1703001166},
      "confidenceScore": 0.9,
      "timeLagMs": 34000
    }
  ],
  "investigationPrompt": "Most likely cause: Deployment 'frontend' update 34 seconds before failure..."
}
```

**LLM Response to User:**
> "The frontend service failure at 2:00 PM was likely caused by a Deployment update at 1:59:26 PM (confidence: 90%, based on graph causality analysis).
>
> **Evidence:**
> - TRIGGERED_BY edge from Deployment update → Pod failures (0.9 confidence)
> - Ownership chain: Deployment 'frontend' → ReplicaSet → Pods
> - Time lag: 34 seconds between update and failure
>
> **Recommended investigation:**
> 1. Review the Deployment update at timestamp 1703001166
> 2. Check if the image tag or configuration changed
> 3. Examine Pod logs for CrashLoopBackOff or ImagePullBackOff errors
>
> Would you like me to retrieve the Deployment change details or Pod logs?"

**Key Anti-Hallucination Elements:**
- Cites confidence score (90%)
- References specific timestamps and UIDs
- Mentions evidence type (TRIGGERED_BY edge)
- Provides actionable next steps
- Asks user before taking further action

---

## Remaining Work

### 1. Result Parsing (2-4 hours)
**Issue:** FalkorDB returns results in array format that needs proper parsing.

**Tasks:**
- Parse node properties from query results
- Extract edge properties (confidence, lag, etc.)
- Handle empty results gracefully
- Map graph types to tool types

**Example:**
```go
func parseResourceNode(nodeData interface{}) (GraphResourceInfo, error) {
    // Parse FalkorDB node format
    // Extract UID, Kind, Namespace, Name properties
}
```

### 2. MCP Server Integration (1-2 hours)
**Tasks:**
- Add tools to `internal/mcp/server.go:registerTools()`
- Define JSON schemas for inputs
- Wire graph client to MCP server
- Test end-to-end via Claude

**Example:**
```go
s.registerTool(
    "graph_find_root_cause",
    "Trace causality backward from failures",
    tools.NewGraphFindRootCauseTool(s.graphClient),
    map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "resourceUID": map[string]interface{}{
                "type": "string",
                "description": "UID of the failing resource",
            },
            // ... more properties
        },
        "required": []string{"resourceUID", "failureTimestamp"},
    },
)
```

### 3. Testing (2-4 hours)
**Test Scenarios:**
- Root cause: Deployment update → Pod crash
- Blast radius: Node failure → Multiple Pod evictions
- Change impact: ConfigMap update → Pod restarts
- Empty results: Resource with no causality links
- LLM query generation: Verify Claude can call tools correctly

### 4. trace_change_impact Implementation (2-3 hours)
Currently not implemented - requires following TRIGGERED_BY edges forward chronologically.

---

## Code Statistics

**Phase 4 Code:**
- `graph_types.go`: 50 lines (shared types)
- `graph_find_root_cause.go`: 170 lines
- `graph_blast_radius.go`: 160 lines
- **Total new**: ~380 lines (70% complete)

**Cumulative (All Phases):**
- Phases 1-3: ~3,155 lines
- Phase 4: ~380 lines
- **Total**: ~3,535 lines production code
- **Tests**: ~440 lines (47 tests from Phases 1-3)

---

## Acceptance Criteria (Phase 4)

| Criterion | Status | Details |
|-----------|--------|---------|
| Tool interface design | ✅ | All 3 tools designed |
| Data structures | ✅ | Structured I/O with confidence scores |
| find_root_cause implementation | 🚧 | 70% - needs result parsing |
| calculate_blast_radius implementation | 🚧 | 70% - needs result parsing |
| trace_change_impact implementation | ⏳ | Not started |
| MCP server integration | ⏳ | Pending |
| Anti-hallucination features | ✅ | Investigation prompts, confidence scores |
| Testing | ⏳ | Pending |

---

## Design Achievements

**1. LLM-Optimized Output**
- Structured JSON prevents hallucination
- Investigation prompts guide LLM responses
- Confidence scores force uncertainty expression

**2. Provenance-Based Reasoning**
- All causality claims reference specific edges
- UIDs and timestamps (not guessed names)
- Evidence chains preserve graph traversal path

**3. Actionable Guidance**
- Investigation prompts are specific, not generic
- Steps reference actual resources and times
- Recommendations based on graph structure

**4. Graceful Degradation**
- Empty results → suggest alternative investigations
- Low confidence → explicit uncertainty
- Errors → human-readable messages

---

## Next Steps

**Immediate (4-8 hours):**
1. Complete result parsing for both tools
2. Integrate with MCP server
3. Test with FalkorDB and real data
4. Verify LLM can call tools correctly

**Phase 5 (Final):**
1. End-to-end integration testing
2. Performance benchmarking
3. Documentation and examples
4. Production deployment guide

**Estimated Completion:** 1-2 days

---

## Conclusion

Phase 4 design is **complete and architecturally sound**. The MCP tools are structured for optimal LLM interaction with:

✅ **Anti-hallucination features** - Provenance, confidence scores, structured output
✅ **Investigation prompts** - Actionable guidance for humans and LLMs
✅ **Graph integration** - Clean interface to reasoning layer
🚧 **Implementation** - 70% complete, needs result parsing and integration

The foundation for LLM-powered root cause analysis is in place. Remaining work is mostly mechanical (parsing, wiring, testing).

**Overall MVP Progress**: **~70% complete** (4 of 5 phases, Phase 4 at 70%)

---

**Contributors**: Claude Sonnet 4.5
**Next Review Date**: After implementation completion
**Related Documents:**
- `graph-reasoning-layer-design.md` - Overall design
- `graph-layer-phase1-summary.md` - Foundation
- `graph-layer-phase2-summary.md` - Sync pipeline
- `graph-layer-phase3-summary.md` - Integration
