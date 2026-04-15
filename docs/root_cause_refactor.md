# Refactor `root_cause` MCP Response to Causality-First Schema

You are a senior Go engineer and distributed-systems architect working on the open-source project **Spectre**.

Your task is to **redesign and implement a causality-first response schema** for the MCP tool `root_cause`, so that LLMs and agents can reliably reason about Kubernetes incidents using Spectre’s graph data.

You are expected to:

* Derive an implementation plan
* Execute it incrementally
* Update tests accordingly

---

## 1. Problem Statement

The current `root_cause` MCP response:

* Is symptom-centric
* Does not explicitly encode causal reasoning
* Forces the LLM to infer graph traversal and time ordering
* Produces low or zero confidence scores despite strong evidence

The goal is to restructure the response so that **causality is explicitly encoded**, not implied.

---

## 2. Target Outcome

The new response MUST:

1. Separate **observed symptoms** from **inferred causes**
2. Explicitly encode the **causal chain** derived from the graph
3. Identify a **root cause hypothesis** with supporting change events
4. List **supporting evidence** and **excluded alternatives**
5. Provide a **transparent confidence score with rationale**
6. Remain fully machine-verifiable (JSON)

The LLM must only **summarize and explain**, not discover causality.

---

## 3. Required Response Structure (Authoritative)

Implement the following high-level structure:

```json
{
  "incident": {
    "observedSymptom": { ... },
    "causalChain": [ ... ],
    "rootCause": { ... },
    "confidence": { ... }
  },
  "supportingEvidence": [ ... ],
  "excludedAlternatives": [ ... ],
  "queryMetadata": { ... }
}
```

You must define concrete schemas for each section and enforce them in code.

---

## 4. Semantic Rules (Non-Negotiable)

* **ObservedSymptom**

  * Must contain only directly observed facts (status, errors)
  * No interpretation or inference

* **CausalChain**

  * Must be ordered
  * Each step must include:

    * resource
    * relationship type
    * reason
  * Must be derived from the graph, not the LLM

* **RootCause**

  * Must reference a specific resource and change event
  * Must explain *why* the change plausibly caused the symptom

* **Confidence**

  * Must include a numeric score AND human-readable rationale
  * Score must be computable deterministically

* **ExcludedAlternatives**

  * Must list at least one plausible but rejected hypothesis when applicable

---

## 5. Implementation Tasks (Agent Must Plan & Execute)

You must:

### A. Design the New Response Types

* Define Go structs for the new schema
* Add JSON schema validation if applicable
* Mark deprecated fields from the old response

### B. Refactor Root Cause Computation

* Extract:

  * Symptom detection
  * Graph traversal
  * Root cause attribution
* Ensure each maps cleanly to a response section

### C. Implement Explicit Causal Chain Construction

* Walk:
  `Pod → ReplicaSet → Deployment → HelmRelease`
* Capture relationship type and reasoning at each hop
* Preserve temporal ordering

### D. Implement Evidence & Exclusion Logic

* Encode evidence as reason codes
* Explicitly detect and record excluded alternatives
* Avoid speculative explanations

### E. Implement Deterministic Confidence Scoring

* Base score on:

  * Direct spec change
  * Temporal precedence
  * Error message match
  * Relationship strength
* Document the formula in code comments

### F. Update Tests

* Refactor existing e2e tests to assert:

  * causalChain correctness
  * rootCause resource identity
  * confidence > threshold
* Avoid asserting natural language text

---

## 7. Quality Bar

Your implementation will be considered correct only if:

* Causal chains are explicit and ordered
* No confidence score is zero when strong evidence exists
* LLMs can explain incidents without graph traversal
* Tests pass deterministically

If tradeoffs are required, **document them explicitly**.

---

## 8. Deliverables

At the end of your work, provide:

1. A brief implementation plan
2. Summary of code changes
3. Any open questions or assumptions

---

## 9. Information You Need From the Codebase

Before starting, locate or ask for:

1. **Current `root_cause` MCP handler**

   * File path
   * Request/response structs

2. **Graph traversal API**

   * How relationships are queried
   * How paths are returned

3. **ChangeEvent model**

   * How spec vs status changes are represented

4. **Evidence / confidence logic (if any exists)**

   * Current scoring or heuristics

5. **E2E test harness**

   * How kind clusters are created
   * How MCP responses are asserted

6. **API versioning strategy (if any)**

If any of the above are missing or unclear, state assumptions explicitly before implementing.

---

## 10. Guiding Principle (Do Not Ignore)

> **If Spectre can compute causality, it must encode it explicitly.
> The LLM must never be asked to infer it.**
