// Package analysis provides graph-based root cause analysis for Kubernetes incidents.
//
// # Overview
//
// This package implements a causality-first approach to root cause analysis (RCA) that
// eliminates the need for LLM-based inference. Instead of analyzing symptoms and guessing
// at causes, it uses the Kubernetes resource graph to trace actual causal chains from
// observed symptoms back to their triggering changes.
//
// # Architecture
//
// The analysis process follows a deterministic pipeline:
//
//  1. Extract observed symptom (facts only, no inference)
//  2. Build causal graph by traversing ownership and relationship edges
//  3. Identify root cause from the causal chain
//  4. Calculate confidence score based on evidence strength
//  5. Collect supporting evidence and excluded alternatives
//
// # Key Concepts
//
// Ownership Chain: The sequence of resources connected via OWNS relationships
// (e.g., Pod <- ReplicaSet <- Deployment <- HelmRelease). This forms the "spine"
// of the causal graph.
//
// Manager Relationships: Resources connected via MANAGES edges (e.g., HelmRelease
// manages Deployment). These represent external management systems.
//
// Related Resources: Supporting resources like Nodes, ServiceAccounts, ConfigMaps,
// etc. that are connected via various relationship types (SCHEDULED_ON, USES_SERVICE_ACCOUNT,
// REFERENCES_SPEC, SELECTS, GRANTS_TO, INGRESS_REF).
//
// SPINE Nodes: Resources in the main ownership chain that form the primary causal path.
//
// RELATED Nodes: Supporting resources that provide context but aren't in the direct
// ownership chain.
//
// # Graph Building Process
//
// The causal graph is built in three phases (see causal_chain.go):
//
//  1. Build SPINE nodes: Create nodes for all resources in the ownership chain
//     and their managers
//  2. Build SPINE edges: Create OWNS and MANAGES edges connecting the chain
//  3. Build RELATED graph: Create nodes and edges for supporting resources
//
// # Query Organization
//
// Graph queries are organized by responsibility:
//
//   - query_ownership.go: Ownership chain traversal
//   - query_relationships.go: Manager and related resource queries
//   - query_events.go: Change event and K8s event queries
//   - query_helpers.go: Shared utility functions
//
// # Confidence Scoring
//
// Confidence scores are deterministically computed from multiple factors:
//
//   - Direct spec change: Did the configuration actually change?
//   - Temporal proximity: How close in time was the change to the symptom?
//   - Relationship strength: How strong is the causal relationship (MANAGES > OWNS)?
//   - Error message match: Does the error explain the symptom?
//   - Chain completeness: How complete is the causal chain?
//
// See confidence.go for implementation details.
//
// # Types and Data Structures
//
// All type definitions are in types.go, organized by purpose:
//
//   - Causality-first schema (RootCauseAnalysisV2, IncidentAnalysis)
//   - Graph structures (CausalGraph, GraphNode, GraphEdge)
//   - Event types (ChangeEventInfo, K8sEventInfo)
//   - Query types (ResourceWithDistance, ManagerData, RelatedResourceData)
//
// # Usage Example
//
//	analyzer := analysis.NewRootCauseAnalyzer(graphClient)
//	result, err := analyzer.Analyze(ctx, analysis.AnalyzeInput{
//		ResourceUID:      "pod-uid-123",
//		FailureTimestamp: time.Now().UnixNano(),
//		LookbackNs:       10 * time.Minute.Nanoseconds(),
//	})
//
// # Design Principles
//
//  1. Explicit over implicit: All causality is derived from the graph, not inferred
//  2. Deterministic computation: Same input always produces same output
//  3. Graph-first thinking: The graph represents ground truth
//  4. Incident-time debuggability: Everything needed to debug an incident is included
//  5. Boring, idiomatic Go: Prefer clarity and maintainability over cleverness
//
// # Performance Considerations
//
// The analyzer batches graph queries to minimize round trips:
//
//   - Ownership chain query (1 query)
//   - Manager relationships (1 batched query)
//   - Related resources (1 batched query)
//   - Change events (1 batched query)
//   - K8s events (1 batched query)
//
// Total: 5 graph queries per analysis, regardless of chain size.
//
// # Testing
//
// The package includes comprehensive tests:
//
//   - analyzer_test.go: End-to-end analyzer tests
//   - causal_chain_test.go: Graph building logic tests
//   - confidence_test.go: Confidence calculation tests
//   - symptom_test.go: Symptom classification tests
//   - root_cause_test.go: Root cause identification tests
//   - causal_chain_integration_test.go: Integration tests with graph queries
package analysis
