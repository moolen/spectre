# Root Cause Tool V2: Quick Reference

## What Changed?

The `find_root_cause` MCP tool now returns a **causality-first** response that explicitly encodes the causal chain from root cause to symptom.

### Before (V1)
```json
{
  "candidates": [
    {
      "resource": {"kind": "Pod", "name": "app-pod"},
      "changeEvent": {...},
      "evidence": ["...", "...", "..."],
      "confidenceScore": 0.0  // Often zero!
    }
  ]
}
```

### After (V2)
```json
{
  "incident": {
    "observedSymptom": {
      "resource": {"kind": "Pod", "name": "app-pod"},
      "status": "Error",
      "errorMessage": "ImagePullBackOff",
      "symptomType": "ImagePullError"
    },
    "causalChain": [
      {
        "stepNumber": 1,
        "resource": {"kind": "HelmRelease", "name": "app"},
        "relationshipType": "MANAGES",
        "reasoning": "HelmRelease manages Deployment lifecycle"
      },
      {
        "stepNumber": 2,
        "resource": {"kind": "Deployment", "name": "app"},
        "relationshipType": "OWNS"
      }
    ],
    "rootCause": {
      "resource": {"kind": "HelmRelease", "name": "app"},
      "causationType": "ConfigChange",
      "explanation": "HelmRelease 'app' configuration was changed, which cascaded through Deployment → Pod"
    },
    "confidence": {
      "score": 0.87,
      "rationale": "Based on: direct spec change, strong management relationship",
      "factors": {
        "directSpecChange": 1.0,
        "temporalProximity": 0.99,
        "relationshipStrength": 1.0
      }
    }
  }
}
```

## Key Improvements

| Aspect | V1 | V2 |
|--------|----|----|
| **Root Cause** | Buried in candidates | Explicit `rootCause` field |
| **Causality** | LLM must infer from evidence | Explicit ordered `causalChain` |
| **Confidence** | Often 0.0 despite evidence | Deterministic, never 0 when evidence exists |
| **Symptom** | Mixed with cause | Separate `observedSymptom` (facts only) |
| **Evidence** | Duplicated, unstructured | Consolidated (max 5 items) |
| **Alternatives** | Not shown | Explicit `excludedAlternatives` |

## Confidence Factors

The confidence score (0.0-1.0) is computed from 5 factors:

1. **directSpecChange** (30%): Did the spec actually change?
   - 1.0 = configChanged=true
   - 0.5 = UPDATE event
   - 0.0 = no change

2. **temporalProximity** (25%): How close in time?
   - 1.0 = immediate (1s)
   - 0.5 = medium (5min)
   - 0.0 = long lag (>10min)

3. **relationshipStrength** (25%): Type of relationship
   - 1.0 = MANAGES
   - 0.8 = OWNS
   - 0.7 = TRIGGERED_BY
   - 0.5 = other

4. **errorMessageMatch** (10%): Does error explain symptom?
   - 1.0 = mentions image/config
   - 0.5 = generic error
   - 0.0 = no error

5. **chainCompleteness** (10%): How complete is the chain?
   - 1.0 = full chain (3+ steps)
   - 0.67 = partial (2 steps)
   - 0.33 = minimal (1 step)

## Symptom Types

Automatically classified from error messages and container issues:

- `ImagePullError` - Image pull failures
- `CrashLoop` - CrashLoopBackOff
- `OOMKilled` - Out of memory
- `SchedulingFailure` - Cannot schedule Pod
- `Evicted` - Pod evicted
- `Error` - Generic error
- `Warning` - Warning state
- `Terminating` - Graceful shutdown
- `Unknown` - Unknown state

## Relationship Types

From the graph schema:

- `MANAGES` - Lifecycle management (e.g., HelmRelease → Deployment)
- `OWNS` - Ownership (e.g., Deployment → ReplicaSet)
- `TRIGGERED_BY` - Temporal causality
- `SYMPTOM` - The observed failure itself

## Evidence Types

Consolidated and categorized:

- `RELATIONSHIP` - Management/ownership evidence
- `TEMPORAL` - Time-based correlation
- `STRUCTURAL` - Ownership chain structure
- `ERROR_CORRELATION` - Error message matches

## Usage (Unchanged)

```json
{
  "resourceUID": "pod-uid-12345",
  "failureTimestamp": 1703001234000000000,
  "maxDepth": 5,
  "minConfidence": 0.6
}
```

## Backward Compatibility

✅ Fully compatible:
- Same tool name: `find_root_cause`
- Same inputs
- V2 is now the default
- V1 still available via `ExecuteV1()` if needed

## Files to Check

### Core Implementation
- `internal/mcp/tools/root_cause_schema.go` - Data structures
- `internal/mcp/tools/root_cause_analysis.go` - Analysis logic
- `internal/mcp/tools/graph_find_root_cause.go` - Tool integration

### Tests
- `internal/mcp/tools/root_cause_analysis_test.go` - Unit tests

### Documentation
- `docs/root_cause_tool/plan.md` - Implementation plan
- `docs/root_cause_tool/IMPLEMENTATION_SUMMARY.md` - Summary

## Common Questions

**Q: Why is my confidence score 0.6?**
A: Check the factors breakdown. Low temporal proximity or missing spec change can reduce confidence.

**Q: Why is the root cause a HelmRelease, not a Pod?**
A: V2 traces the causal chain backward to find the actual source of the change, not just the symptom.

**Q: Can I get the old response format?**
A: The old implementation is preserved as `ExecuteV1()` but V2 is now the default.

**Q: What if no causal chain is found?**
A: The response will still include the `observedSymptom` and potentially `excludedAlternatives` showing what was considered.

## Example: Reading V2 Response

```go
response := tool.ExecuteV2(ctx, input)

// Get the root cause
rootKind := response.Incident.RootCause.Resource.Kind
rootName := response.Incident.RootCause.Resource.Name
causationType := response.Incident.RootCause.CausationType

// Get confidence
confidence := response.Incident.Confidence.Score
rationale := response.Incident.Confidence.Rationale

// Get causal chain
for _, step := range response.Incident.CausalChain {
    fmt.Printf("Step %d: %s (%s)\n", 
        step.StepNumber, 
        step.Resource.Kind,
        step.RelationshipType)
}

// Check evidence
for _, evidence := range response.SupportingEvidence {
    fmt.Printf("%s: %s (confidence: %.2f)\n",
        evidence.Type,
        evidence.Description,
        evidence.Confidence)
}
```

---

**Last Updated**: 2024-12-19
